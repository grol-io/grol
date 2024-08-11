package object

import (
	"fmt"
	"io"
	"slices"
	"sort"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/sets"
	"grol.io/grol/token"
	"grol.io/grol/trie"
)

type Environment struct {
	store    map[string]Object
	outer    *Environment
	depth    int
	cacheKey string
	ids      *trie.Trie
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

var baseInfo Map

func (e *Environment) BaseInfo() Map {
	if baseInfo != nil {
		return baseInfo
	}
	baseInfo := make(Map, 6) // 5 here + all_ids
	tokInfo := token.Info()
	keys := make([]Object, 0, len(tokInfo.Keywords))
	for _, v := range sets.Sort(tokInfo.Keywords) {
		keys = append(keys, String{Value: v})
	}
	baseInfo[String{"keywords"}] = Array{Elements: keys}
	keys = make([]Object, 0, len(tokInfo.Tokens))
	for _, v := range sets.Sort(tokInfo.Tokens) {
		keys = append(keys, String{Value: v})
	}
	baseInfo[String{"tokens"}] = Array{Elements: keys}
	// Ditto cache this as it's set for a given environment.
	ext := ExtraFunctions()
	keys = make([]Object, 0, len(ext))
	for k := range ext {
		keys = append(keys, String{Value: k})
	}
	arr := Array{Elements: keys}
	sort.Sort(arr)
	baseInfo[String{"gofuncs"}] = arr
	baseInfo[String{"version"}] = String{Value: cli.ShortVersion}
	baseInfo[String{"platform"}] = String{Value: cli.LongVersion}
	return baseInfo
}

func record(ids *trie.Trie, key string, t Type) {
	if ids == nil {
		return
	}
	if t == FUNC {
		ids.Insert(key + "(")
	} else {
		ids.Insert(key + " ")
	}
	ids.Insert(key)
}

// Records the current toplevel ids and functions as well
// as sets up the callback to for future ids additions.
func (e *Environment) RegisterTrie(t *trie.Trie) {
	for e.outer != nil {
		e = e.outer
	}
	e.ids = t
	for k, v := range e.store {
		record(t, k, v.Type())
	}
}

// Returns the number of ids written.
func (e *Environment) SaveGlobals(to io.Writer) (int, error) {
	for e.outer != nil {
		e = e.outer
	}
	keys := make([]string, 0, len(e.store))
	for k := range e.store {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	n := 0
	for _, k := range keys {
		v := e.store[k]
		if v.Type() == FUNC {
			f := v.(Function)
			if f.Name != nil {
				// Named function inspect is ready for definition, eg func y(a,b){a+b}.
				_, err := fmt.Fprintf(to, "%s\n", f.Inspect())
				if err != nil {
					return n, err
				}
				n++
				continue
			}
			// Anonymous function are like other variables.
			//   x=func(a,b){a+b}
			// fallthrough.
		}
		_, err := fmt.Fprintf(to, "%s=%s\n", k, v.Inspect())
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (e *Environment) Info() Object {
	allKeys := make([]Object, e.depth+1)
	for {
		val := make(Map, e.Len())
		for k, v := range e.store {
			val[String{Value: k}] = v
		}
		allKeys[e.depth] = val
		if e.outer == nil {
			break
		}
		e = e.outer
	}
	info := e.BaseInfo()
	info[String{"all_ids"}] = Array{Elements: allKeys}
	return info
}

func (e *Environment) Get(name string) (Object, bool) {
	if name == "info" {
		return e.Info(), true
	}
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
		if i != 0 && (v == '_' || (v >= '0' && v <= '9')) {
			continue
		}
		if v < 'A' || v > 'Z' {
			return false
		}
	}
	return true
}

func (e *Environment) SetNoChecks(name string, val Object) Object {
	if e.depth == 0 {
		if _, ok := e.store[name]; !ok {
			// new top level entry, record it.
			record(e.ids, name, val.Type())
		}
	}
	e.store[name] = val
	return val
}

func (e *Environment) Set(name string, val Object) Object {
	if Constant(name) {
		old, ok := e.Get(name) // not ok
		if ok {
			log.Infof("Attempt to change constant %s from %v to %v", name, old, val)
			if !Equals(old, val) {
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
	env := &Environment{store: make(map[string]Object), outer: parent, cacheKey: fn.CacheKey, depth: parent.depth + 1}
	return env, sameFunction
}
