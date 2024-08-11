package eval

import (
	"fmt"
	"io"
	"os"

	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
	"grol.io/grol/trie"
)

// Exported part of the eval package.

type State struct {
	env        *object.Environment
	Out        io.Writer
	LogOut     io.Writer
	NoLog      bool // turn log() into println() (for EvalString)
	cache      Cache
	extensions map[string]object.Extension
}

func NewState() *State {
	return &State{
		env:        object.NewRootEnvironment(),
		Out:        os.Stdout,
		LogOut:     os.Stdout,
		cache:      NewCache(),
		extensions: object.ExtraFunctions(),
	}
}

func NewBlankState() *State {
	return &State{
		env:        object.NewMacroEnvironment(), // to get empty store
		Out:        io.Discard,
		LogOut:     io.Discard,
		cache:      NewCache(),
		extensions: make(map[string]object.Extension),
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
func (s *State) Save(w io.Writer) error {
	return s.env.Save(w)
}

// Does unwrap (so stop bubbling up) return values.
func (s *State) Eval(node any) object.Object {
	result := s.evalInternal(node)
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
	}
	res := evalState.Eval(program)
	if res.Type() == object.ERROR {
		return res, fmt.Errorf("eval error: %v", res.Inspect())
	}
	return res, nil
}
