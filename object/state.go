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
	stack    *Environment // Different from outer when we attach to top level lambdas. see logic in NewFunctionEnvironment.
	depth    int
	cacheKey string
	ids      *trie.Trie
	numSet   int64
	getMiss  int64
	function *Function
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

var baseInfo BigMap

func (e *Environment) BaseInfo() *BigMap {
	if baseInfo.kv != nil {
		return &baseInfo
	}
	baseInfo.kv = make([]keyValuePair, 0, 7) // 6 here + all_ids
	tokInfo := token.Info()
	keys := make([]Object, 0, len(tokInfo.Keywords))
	for _, v := range sets.Sort(tokInfo.Keywords) {
		keys = append(keys, String{Value: v})
	}
	baseInfo.Set(String{"keywords"}, BigArray{elements: keys}) // 1
	keys = make([]Object, 0, len(tokInfo.Tokens))
	for _, v := range sets.Sort(tokInfo.Tokens) {
		keys = append(keys, String{Value: v})
	}
	baseInfo.Set(String{"tokens"}, BigArray{elements: keys}) // 2
	keys = make([]Object, 0, len(tokInfo.Builtins))
	for _, v := range sets.Sort(tokInfo.Builtins) {
		keys = append(keys, String{Value: v})
	}
	baseInfo.Set(String{"builtins"}, BigArray{elements: keys}) // 3
	// Ditto cache this as it's set for a given environment.
	ext := ExtraFunctions()
	keys = make([]Object, 0, len(ext))
	for k := range ext {
		keys = append(keys, String{Value: k})
	}
	arr := BigArray{elements: keys}
	sort.Sort(arr)
	baseInfo.Set(String{"gofuncs"}, arr)                             // 4
	baseInfo.Set(String{"version"}, String{Value: cli.ShortVersion}) // 5
	baseInfo.Set(String{"platform"}, String{Value: cli.LongVersion}) // 6
	return &baseInfo
}

func (e *Environment) Info() Object {
	allKeys := make([]Object, e.depth+1)
	for {
		keys := make([]string, 0, len(e.store))
		for k := range e.store {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		arr := MakeObjectSlice(len(keys))
		for _, k := range keys {
			arr = append(arr, String{Value: k})
		}
		allKeys[e.depth] = NewArray(arr)
		if e.outer == nil {
			break
		}
		e = e.outer
	}
	info := e.BaseInfo()
	info.Set(String{"all_ids"}, BigArray{elements: allKeys}) // 7
	return info
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
	t.Insert("info ") // magic extra identifier (need the space).
}

// Returns the number of ids written. maxValueLen <= 0 means no limit.
func (e *Environment) SaveGlobals(to io.Writer, maxValueLen int) (int, error) {
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
		if isConstantAndExtraIdentifier(k) {
			// Don't save PI, E, etc.. that can't be changed.
			continue
		}
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
		val := v.Inspect()
		if maxValueLen > 0 && len(val) > maxValueLen {
			log.Warnf("Skipping %q as it's too long (%d > %d)", k, len(val), maxValueLen)
			continue
		}
		_, err := fmt.Fprintf(to, "%s=%s\n", k, val)
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (e *Environment) Get(name string) (Object, bool) {
	if name == "info" {
		return e.Info(), true
	}
	obj, ok := e.store[name]
	if ok || e.outer == nil {
		return obj, ok
	}
	log.Debugf("Get miss (%s) called at %d %v", name, e.depth, e.cacheKey)
	e.getMiss++
	for e.outer != nil {
		obj, ok = e.outer.store[name]
		if ok {
			for obj.Type() == REFERENCE { // for is in theory not needed as we deref new refs.
				obj = obj.(Reference).Value()
			}
			return Reference{Name: name, RefEnv: e.outer}, true
		}
		e = e.outer
	}
	return nil, false
}

// TriggerNoCache is used prevent this call stack from caching.
// Meant to be used by extensions that for instance return random numbers or change state.
func (e *Environment) TriggerNoCache() {
	log.Debugf("TriggerNoCache() called at %d %v", e.depth, e.cacheKey)
	e.getMiss++
}

// GetMisses returns the cumulative number of get misses (a function tried to access up stack, so can't be cached).
func (e *Environment) GetMisses() int64 {
	return e.getMiss
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

// NumSet returns the cumulative number of set operations done in the toplevel environment so far.
// It can be used to avoid calling SaveGlobals if nothing has changed since the last time.
func (e *Environment) NumSet() int64 {
	return e.numSet
}

func (e *Environment) IsRef(name string) (*Environment, string) {
	log.Debugf("IsRef(%s) called at %d %v", name, e.depth, e.cacheKey)
	r, ok := e.store[name]
	if !ok {
		return nil, ""
	}
	if rr, ok := r.(Reference); ok {
		log.Debugf("IsRef(%s) true: %s in %v", name, rr.Name, rr.RefEnv.depth)
		return rr.RefEnv, rr.Name
	}
	return nil, ""
}

func (e *Environment) SetNoChecks(name string, val Object) Object {
	if r, n := e.IsRef(name); r != nil {
		e = r
		name = n
	}
	if e.depth == 0 {
		e.numSet++
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
	if IsExtraFunction(name) {
		return Error{Value: fmt.Sprintf("attempt to change internal function %s to %s", name, val.Inspect())}
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
	env := &Environment{
		store:    make(map[string]Object),
		stack:    current,
		outer:    parent,
		cacheKey: fn.CacheKey,
		depth:    parent.depth + 1,
		function: &fn,
	}
	return env, sameFunction
}

// Frame/stack name.
func (e *Environment) Name() string {
	if e.function == nil {
		return ""
	}
	return e.function.Inspect()
}

// Allows eval and others to walk up the stack of envs themselves
// (using Name() to produce a stack trace for instance).
func (e *Environment) StackParent() *Environment {
	return e.stack
}
