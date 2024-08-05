package object

import (
	"fmt"

	"fortio.org/log"
)

type Environment struct {
	store    map[string]Object
	outer    *Environment
	cacheKey string
}

// Truly empty store suitable for macros storage.
func NewMacroEnvironment() *Environment {
	return &Environment{store: make(map[string]Object)}
}

// NewRootEnvironment contains the identifiers pre-seeded by extensions.
func NewRootEnvironment() *Environment {
	s := initialIdentifiersCopy()
	return &Environment{store: s}
}

func (e *Environment) Len() int {
	log.Debugf("Environment.Len() called for %#v with %d entries", e, len(e.store))
	if e.outer != nil {
		return len(e.store) + e.outer.Len()
	}
	return len(e.store)
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if ok || e.outer == nil {
		return obj, ok
	}
	return e.outer.Get(name) // recurse.
}

// Defines constant as all CAPS (with _ ok in the middle) identifiers.
// Note that we use []byte as all identifiers are ASCII.
func Constant(name string) bool {
	for i, v := range name {
		if i != 0 && v == '_' {
			continue
		}
		if v < 'A' || v > 'Z' {
			return false
		}
	}
	return true
}

func (e *Environment) SetNoChecks(name string, val Object) Object {
	e.store[name] = val
	return val
}

func (e *Environment) Set(name string, val Object) Object {
	if Constant(name) {
		old, ok := e.Get(name) // not ok
		if ok {
			log.Infof("Attempt to change constant %s from %v to %v", name, old, val)
			if !Hashable(old) || !Hashable(val) || old != val {
				return Error{Value: fmt.Sprintf("attempt to change constant %s from %s to %s", name, old.Inspect(), val.Inspect())}
			}
		}
	}
	return e.SetNoChecks(name, val)
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := &Environment{store: make(map[string]Object), outer: outer}
	return env
}

// Create a new environment either based on original function definitions' environment
// or the current one if the function is the same, that allows a function to set some values
// visible through recursion to itself.
//
//	func test(n) {if (n==2) {x=1}; if (n==1) {return x}; test(n-1)}; test(3)
//
// will return 1 (and not "identifier not found: x").
// Returns true if the function is the same as the current one and we should probably
// set the function's name in that environment to avoid deep search for it.
func NewFunctionEnvironment(fn Function, current *Environment) (*Environment, bool) {
	parent := current
	sameFunction := (current.cacheKey == fn.CacheKey)
	if !sameFunction {
		parent = fn.Env
	}
	env := &Environment{store: make(map[string]Object), outer: parent, cacheKey: fn.CacheKey}
	return env, sameFunction
}
