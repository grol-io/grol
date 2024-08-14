package eval

import (
	"fmt"
	"io"
	"os"

	"fortio.org/log"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
	"grol.io/grol/trie"
)

// Exported part of the eval package.

const DefaultMaxDepth = 250_000

type State struct {
	Out        io.Writer
	LogOut     io.Writer
	macroState *object.Environment
	env        *object.Environment
	cache      Cache
	extensions map[string]object.Extension
	NoLog      bool // turn log() into println() (for EvalString)
	// Max depth / recursion level - default DefaultMaxDepth,
	// note that a simple function consumes at least 2 levels and typically at least 3 or 4.
	MaxDepth   int
	depth      int // current depth / recursion level
	lastNumSet int64
}

func NewState() *State {
	return &State{
		env:        object.NewRootEnvironment(),
		Out:        os.Stdout,
		LogOut:     os.Stdout,
		cache:      NewCache(),
		extensions: object.ExtraFunctions(),
		macroState: object.NewMacroEnvironment(),
		MaxDepth:   DefaultMaxDepth,
		depth:      0,
	}
}

func NewBlankState() *State {
	return &State{
		env:        object.NewMacroEnvironment(), // to get empty store
		Out:        io.Discard,
		LogOut:     io.Discard,
		cache:      NewCache(),
		extensions: make(map[string]object.Extension),
		macroState: object.NewMacroEnvironment(),
	}
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
	return s.env.SaveGlobals(w)
}

// NumSet returns the previous and current cumulative number of set in the toplevel environment, if that
// number hasn't changed, no need to autosave.
func (s *State) UpdateNumSet() (oldvalue, newvalue int64) {
	oldvalue = s.lastNumSet
	newvalue = s.env.NumSet()
	s.lastNumSet = newvalue
	return
}

// Does unwrap (so stop bubbling up) return values.
func (s *State) Eval(node any) object.Object {
	if s.depth > s.MaxDepth {
		log.LogVf("max depth %d reached", s.MaxDepth) // will be logged by the panic handler.
		s.depth = 0
		panic(fmt.Sprintf("max depth %d reached", s.MaxDepth))
	}
	s.depth++
	result := s.evalInternal(node)
	s.depth--
	// unwrap return values only at the top.
	if returnValue, ok := result.(object.ReturnValue); ok {
		return returnValue.Value
	}
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

// Evals a string either from entirely blank environment or from the current environment.
// `unjson` uses emptyEnv == true (for now, pending better/safer implementation).
//
//nolint:revive // eval.EvalString is fine.
func EvalString(this any, code string, emptyEnv bool) (object.Object, error) {
	l := lexer.New(code)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return object.NULL, fmt.Errorf("parsing error: %v", p.Errors())
	}
	var evalState *State
	if emptyEnv {
		evalState = NewBlankState()
	} else {
		var ok bool
		evalState, ok = this.(*State)
		if !ok {
			return object.NULL, fmt.Errorf("invalid this: %T", this)
		}
		evalState.DefineMacros(program)
		_ = evalState.ExpandMacros(program)
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
