package eval

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"fortio.org/log"
	"fortio.org/terminal"
	"grol.io/grol/ast"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
	"grol.io/grol/token"
	"grol.io/grol/trie"
)

// Exported part of the eval package.

const (
	// Approximate maximum depth of recursion to avoid:
	// runtime: goroutine stack exceeds 1000000000-byte limit
	// fatal error: stack overflow. Was 250k but adding a log
	// in Error() makes it go over that (somehow).
	DefaultMaxDepth    = 150_000
	DefaultMaxDuration = 10 * time.Second
)

type State struct {
	Term       *terminal.Terminal
	Out        io.Writer
	LogOut     io.Writer
	macroState *object.Environment
	env        *object.Environment
	rootEnv    *object.Environment // same as ancestor of env but used for reset in panic recovery.
	cache      Cache
	Extensions object.ExtensionMap
	NoLog      bool // turn log() into println() (for EvalString)
	// Max depth / recursion level - default DefaultMaxDepth,
	// note that a simple function consumes at least 2 levels and typically at least 3 or 4.
	MaxDepth    int
	depth       int // current depth / recursion level
	lastNumSet  int64
	MaxValueLen int // max length of value to save in files, <= 0 for unlimited.
	// To enforce a max duration or cancel evals.
	Context context.Context //nolint:containedctx // we need a context for callbacks from extensions and to set it without API change.
	Cancel  context.CancelFunc
	PipeVal []byte // value to return from pipe() function
	NoReg   bool   // don't use registers.
	// Current file being processed (TODO: use it to have parsing errors showing as filename:line...)
	CurrentFile string
}

func NewState() *State {
	st := &State{
		env:        object.NewRootEnvironment(),
		Out:        os.Stdout,
		LogOut:     os.Stdout,
		cache:      NewCache(),
		Extensions: object.ExtraFunctions(),
		macroState: object.NewMacroEnvironment(),
		MaxDepth:   DefaultMaxDepth,
		depth:      0,
	}
	st.rootEnv = st.env
	return st
}

func NewBlankState() *State {
	st := &State{
		env:        object.NewMacroEnvironment(), // to get empty store
		Out:        io.Discard,
		LogOut:     io.Discard,
		cache:      NewCache(),
		Extensions: make(map[string]object.Extension),
		macroState: object.NewMacroEnvironment(),
		MaxDepth:   DefaultMaxDepth,
	}
	st.rootEnv = st.env
	return st
}

// Reset post panic recovery.
func (s *State) Reset() {
	s.env = s.rootEnv
	s.depth = 0
}

// RegisterTrie sets up the Trie to record all top level ids and functions.
// Forwards to the underlying object store environment.
func (s *State) RegisterTrie(t *trie.Trie) {
	s.env.RegisterTrie(t)
}

func (s *State) ResetCache() {
	s.cache = NewCache()
}

// Forward to env to count the number of bindings. Used mostly to know if there are any macros.
func (s *State) Len() int {
	return s.env.Len()
}

// Save() saves the current toplevel state (ids and functions) to the writer, forwards to the object store.
// Saves the top level (global) environment.
func (s *State) SaveGlobals(w io.Writer) (int, error) {
	return s.env.SaveGlobals(w, s.MaxValueLen)
}

// NumSet returns the previous and current cumulative number of set in the toplevel environment, if that
// number hasn't changed, no need to autosave.
func (s *State) UpdateNumSet() (oldvalue, newvalue int64) {
	oldvalue = s.lastNumSet
	newvalue = s.env.NumSet()
	s.lastNumSet = newvalue
	return
}

// SetContext sets the context for the evaluator, with a maximum duration.
// The returned cancel function can be used to cancel the context sooner and must be
// called (in a defer typically) to release resources (to avoid issue #204).
func (s *State) SetContext(ctx context.Context, d time.Duration) context.CancelFunc {
	if d <= 0 {
		log.LogVf("SetContext with unlimited duration")
		s.Context, s.Cancel = context.WithCancel(ctx)
	} else {
		s.Context, s.Cancel = context.WithTimeout(ctx, d)
	}
	return s.Cancel
}

func (s *State) SetDefaultContext() context.CancelFunc {
	return s.SetContext(context.Background(), DefaultMaxDuration)
}

// Final unwrapped result of an evaluation (for instance unwraps the registers which Eval() does not).
func (s *State) EvalToplevel(node any) object.Object {
	return object.Value(s.Eval(node))
}

// Does unwrap (so stop bubbling up) return values.
func (s *State) Eval(node any) object.Object {
	if s.depth > s.MaxDepth {
		log.LogVf("max depth %d reached", s.MaxDepth) // will be logged by the panic handler.
		// State must be reset using s.Reset() to reuse the evaluator post panic (not just for depth but for s.env)
		panic(fmt.Sprintf("max depth %d reached", s.MaxDepth))
	}
	s.depth++
	result := s.evalInternal(node)
	s.depth--
	// unwrap return values only at the top.
	if returnValue, ok := result.(object.ReturnValue); ok {
		if returnValue.ControlType != token.RETURN {
			return s.Errorf("unexpected control type %v outside of for loops", returnValue.ControlType)
		}
		result = returnValue.Value
	}
	if refValue, ok := result.(object.Reference); ok {
		return refValue.ObjValue()
	}
	/* Doing this at each eval breaks some optimization so we do it only in the caller/repl/last level.
	if registerValue, ok := result.(*object.Register); ok {
		return registerValue.ObjValue()
	}
	*/
	return result
}

// AddEvalResult adds the result of an evaluation (for instance a function object)
// to the base identifiers. Used to add grol defined functions to the base environment
// (e.g abs(), log2(), etc). Eventually we may instead `include("lib.gr")` or some such.
func AddEvalResult(name, code string) error {
	res, err := EvalString(NewState(), code, false)
	if err != nil {
		return err
	}
	object.AddIdentifier(name, res)
	return nil
}

// Name of the args array in the interpreter state.
const argsName = "args"

// SetArgs sets the args array for this interpreter state (used for #! shebang mode).
func (s *State) SetArgs(args []string) object.Object {
	arr := object.MakeObjectSlice(len(args))
	for _, arg := range args {
		arr = append(arr, object.String{Value: arg})
	}
	return s.env.SetNoChecks(argsName, object.NewArray(arr), true)
}

// Evals a string either from entirely blank environment or from the current environment.
// `unjson` uses emptyEnv == true (for now, pending better/safer implementation).
//
//nolint:revive // eval.EvalString is fine.
func EvalString(this any, code string, emptyEnv bool) (object.Object, error) {
	l := lexer.New(code)
	p := parser.New(l)
	var program ast.Node
	program = p.ParseProgram()
	if len(p.Errors()) != 0 {
		return object.NULL, fmt.Errorf("parsing error: %v", p.Errors())
	}
	evalState, ok := this.(*State)
	if emptyEnv {
		maxDepth := DefaultMaxDepth
		if ok {
			maxDepth = evalState.MaxDepth // in case it's lower, carry that lower value.
		}
		evalState = NewBlankState()
		evalState.MaxDepth = maxDepth
	} else {
		if !ok {
			return object.NULL, fmt.Errorf("invalid this: %T", this)
		}
		evalState.DefineMacros(program)
		program = evalState.ExpandMacros(program)
	}
	res := evalState.Eval(program)
	if res.Type() == object.ERROR {
		return res, fmt.Errorf("eval error: %v", res.Inspect())
	}
	return res, nil
}

func (s *State) NumMacros() int {
	return s.macroState.Len()
}

// TriggerNoCache() is replaced by DontCache boolean in object.Extension.

func (s *State) GetPipeValue() []byte {
	return s.PipeVal
}

// FlushOutput writes any buffered output to the actual output writer.
// This is needed before read operations to ensure prompts and other output
// are visible to the user immediately.
func (s *State) FlushOutput() {
	// Write buffered output to real output
	_, err := s.env.PrevOut.Write(s.env.OutputBuffer.Bytes())
	if err != nil {
		log.Warnf("flush output: %v", err)
	}
	// Clear the buffer but keep it for future writes
	s.env.OutputBuffer.Reset()
}
