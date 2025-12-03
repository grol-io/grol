package object

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"sort"

	"fortio.org/cli"
	"fortio.org/log"
	"fortio.org/sets"
	"grol.io/grol/ast"
	"grol.io/grol/token"
	"grol.io/grol/trie"
)

const NumRegisters = 8

type Environment struct {
	store     map[string]Object
	outer     *Environment
	stack     *Environment // Different from outer when we attach to top level lambdas. see logic in NewFunctionEnvironment.
	depth     int
	cacheKey  string
	ids       *trie.Trie
	numSet    int64
	getMiss   int64
	cantCache bool
	function  *Function
	registers [NumRegisters]int64
	numReg    int
	// Output buffering state
	OutputBuffer *bytes.Buffer
	PrevOut      io.Writer
}

// NewMacroEnvironment creates a truly empty store suitable for macros storage.
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
	baseInfo.kv = make([]keyValuePair, 0, 8) // 6 here + globals + stack
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
	allKeys := make([]Object, e.depth)
	info := e.BaseInfo()
	for {
		keys := make([]string, 0, len(e.store))
		for k := range e.store {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		set := NewMapSize(len(keys))
		for _, k := range keys {
			set = set.Set(String{Value: k}, TRUE)
		}
		log.LogVf("Info() for %d - %d - keys %v set %v", e.depth, len(e.store), keys, set)
		if e.depth == 0 {
			info.Set(String{"globals"}, set) // 0 == globals
		} else {
			allKeys[e.depth-1] = set
		}
		if e.outer == nil {
			break
		}
		e = e.outer
	}
	info.Set(String{"stack"}, NewArray(allKeys))
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

// RegisterTrie records the current toplevel ids and functions as well
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

// SaveGlobals saves and returns the number of ids written. maxValueLen <= 0 means no limit.
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

func (e *Environment) HasRegisters() bool {
	return e.numReg < NumRegisters
}

func (e *Environment) MakeRegister(originalName string, v int64) Register {
	if !e.HasRegisters() {
		panic(fmt.Sprintf("No more registers available for %s (%d) have %v", originalName, v, e.registers))
	}
	e.registers[e.numReg] = v
	tok := token.Intern(token.REGISTER, originalName)
	r := Register{RefEnv: e, Idx: e.numReg, Base: ast.Base{Token: tok}}
	e.numReg++
	return r
}

func (e *Environment) ReleaseRegister(register Register) {
	if register.Idx != e.numReg-1 {
		panic(fmt.Sprintf("Releasing non last register %s %d != %d", register.Literal(), register.Idx, e.numReg-1))
	}
	e.numReg--
}

func (e *Environment) makeRef(name string) (*Reference, bool) {
	orig := e
	for e.outer != nil {
		obj, ok := e.outer.store[name]
		if !ok {
			e = e.outer
			continue
		}
		ref := Reference{Name: name, RefEnv: e.outer}
		if r, isRef := obj.(Reference); isRef {
			log.Debugf("makeRef(%s) found ref %s in %d", name, r.Name, r.RefEnv.depth)
			ref = r // set and return the original ref instead of ref of ref.
		}
		orig.store[name] = ref
		if !Constant(name) && obj.Type() != FUNC {
			orig.getMiss++ // creating a ref to a non constant is a miss.
			log.Debugf("makeRef(%s) GETMISS %d", name, orig.getMiss)
		}
		return &ref, true
	}
	return nil, false
}

func (e *Environment) Get(name string) (Object, bool) {
	if name == "info" {
		e.TriggerNoCache()
		return e.Info(), true
	}
	if name == "self" {
		if e.function != nil {
			return *e.function, true
		}
		return nil, false
	}
	if e.function != nil && e.function.Name != nil && name == e.function.Name.Literal() {
		return *e.function, true
	}
	obj, ok := e.store[name]
	if ok {
		// using references to non constant (extensions are constants) implies uncacheable.
		if r, ok := obj.(Reference); ok && !Constant(r.Name) && r.ObjValue().Type() != FUNC {
			e.getMiss++
			log.Debugf("get(%s) GETMISS %d", name, e.getMiss)
		}
		return obj, true
	}
	if e.outer == nil {
		return nil, false
	}
	log.Debugf("Get miss (%s) called at %d %v", name, e.depth, e.cacheKey)
	if ref, ok := e.makeRef(name); ok {
		return *ref, true
	}
	return nil, false
}

// Delete removes the first entry found under that name from the environment.
// TODO: check if references need special handling / if variable is in use/referenced somewhere.
// (had a SEGV in https://github.com/grol-io/grol/issues/335 ).
func (e *Environment) Delete(name string) Object {
	if e.depth == 0 {
		e.numSet++
	}
	if _, ok := e.store[name]; ok {
		delete(e.store, name)
		log.Debugf("Delete(%s) found at %d %v", name, e.depth, e.cacheKey)
		return TRUE
	}
	if e.outer != nil {
		return e.outer.Delete(name)
	}
	return FALSE
}

// TriggerNoCache is used prevent this call stack from caching.
// Meant to be used by extensions that for instance return random numbers or change state.
func (e *Environment) TriggerNoCache() {
	log.Debugf("TriggerNoCache() GETMISS called at %d %v", e.depth, e.cacheKey)
	e.cantCache = true
	e.getMiss++
}

// GetMisses returns the cumulative number of get misses (a function tried to access up stack, so can't be cached).
func (e *Environment) GetMisses() int64 {
	return e.getMiss
}

// CantCache returns true if a non-cacheable extension (like rand()) was called.
func (e *Environment) CantCache() bool {
	return e.cantCache
}

// Constant defines constant as all CAPS (with _ ok in the middle) identifiers.
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

func (e *Environment) create(name string, val Object) Object {
	if e.depth == 0 {
		e.numSet++
		record(e.ids, name, val.Type())
	}
	val = Value(val)
	e.store[name] = val
	return val
}

func (e *Environment) update(name string, found, val Object) Object {
	// Dereference registers and references to store actual values, not pointers.
	val = Value(val)
	if rr, ok := found.(Reference); ok {
		log.Debugf("SetNoChecks(%s) updating ref %s in %d", name, rr.Name, rr.RefEnv.depth)
		e = rr.RefEnv
		name = rr.Name
	}
	e.store[name] = val
	if e.depth == 0 {
		e.numSet++
	}
	return val
}

// SetNoChecks forces the creation of a new entry, even if had a previous value or ref.
// (eg. function parameters are always new).
func (e *Environment) SetNoChecks(name string, val Object, create bool) Object {
	if create {
		log.Debugf("SetNoChecks(%s) forced create to %d", name, e.depth)
		return e.create(name, val)
	}
	r, ok := e.store[name] // is this an update? possibly of an existing ref.
	if ok {
		return e.update(name, r, val)
	}
	// New name... let's see if it's really new or making it a ref.
	if ref, ok := e.makeRef(name); ok {
		log.Debugf("SetNoChecks(%s) created ref %s in %d", name, ref.Name, ref.RefEnv.depth)
		val = Value(val) // Dereference to store actual value, not a register/reference pointer.
		ref.RefEnv.store[ref.Name] = val
		return val
	}
	log.Debugf("SetNoChecks(%s) brand new to %d and above", name, e.depth)
	return e.create(name, val)
}

func (e *Environment) Set(name string, val Object) Object {
	return e.CreateOrSet(name, val, false)
}

func (e *Environment) CreateOrSet(name string, val Object, create bool) Object {
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
	return e.SetNoChecks(name, val, create)
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := &Environment{store: make(map[string]Object), outer: outer}
	return env
}

// NewFunctionEnvironment creates a new environment either based on original function definitions' environment
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

// Name returns the name of the frame or stack.
func (e *Environment) Name() string {
	if e.function == nil {
		return ""
	}
	return e.function.Inspect()
}

// StackParent allows eval and others to walk up the stack of envs themselves
// (using Name() to produce a stack trace for instance).
func (e *Environment) StackParent() *Environment {
	return e.stack
}
