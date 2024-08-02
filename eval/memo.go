package eval

import (
	"grol.io/grol/object"
)

const MaxArgs = 6

type CacheKey struct {
	Fn   string
	Args [MaxArgs]object.Object
}

type Cache map[CacheKey]object.Object

func NewCache() Cache {
	return make(Cache)
}

func (c Cache) Get(fn string, args []object.Object) (object.Object, bool) {
	key := CacheKey{Fn: fn}
	for i, v := range args {
		// Can't hash functions arguments (yet).
		if _, ok := v.(object.Function); ok {
			return nil, false
		}
		key.Args[i] = v
	}
	result, ok := c[key]
	return result, ok
}

func (c Cache) Set(fn string, args []object.Object, result object.Object) {
	key := CacheKey{Fn: fn}
	for i, v := range args {
		// Can't hash functions arguments (yet).
		if _, ok := v.(object.Function); ok {
			return
		}
		key.Args[i] = v
	}
	c[key] = result
}
