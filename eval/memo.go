package eval

import (
	"grol.io/grol/object"
)

const MaxArgs = 4

type CacheKey struct {
	Fn   string
	Args [MaxArgs]object.Object
}

type Cache map[CacheKey]object.Object

func NewCache() Cache {
	return make(Cache)
}

func (c Cache) Get(fn string, args []object.Object) (object.Object, bool) {
	if len(args) > MaxArgs {
		return nil, false
	}
	key := CacheKey{Fn: fn}
	for i, v := range args {
		// Can't hash functions, arrays, maps arguments (yet).
		if !object.Hashable(v) {
			return nil, false
		}
		key.Args[i] = v
	}
	result, ok := c[key]
	return result, ok
}

func (c Cache) Set(fn string, args []object.Object, result object.Object) {
	if len(args) > MaxArgs {
		return
	}
	key := CacheKey{Fn: fn}
	for i, v := range args {
		// Can't hash functions arguments (yet).
		if !object.Hashable(v) {
			return
		}
		key.Args[i] = v
	}
	c[key] = result
}
