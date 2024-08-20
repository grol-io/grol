package object

import (
	"fmt"
	"math/bits"
	"runtime"
	"runtime/debug"
)

// Size of the Object interface in bytes.
const ObjectSize = 2 * bits.UintSize / 8 // also unsafe.Sizeof(interface) == 16 bytes (2 pointers == 2 ints)

// Returns the amount of free memory in bytes.
func FreeMemory() int64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	currentAlloc := memStats.HeapAlloc
	// retrieve the current limit.
	gomemlimit := debug.SetMemoryLimit(-1)
	return int64(gomemlimit) - int64(currentAlloc) //nolint:unconvert,gosec // necessary, can be negative.
}

func SizeOk(n int) (bool, int64) {
	if n <= 256 { // no checks for small slices (4k memory/one typical page)
		return true, 0
	}
	free := FreeMemory()
	return ((free >= 0) && ((int64(n) * ObjectSize) < free)), free
}

func MustBeOk(n int) {
	if ok, _ := SizeOk(n); ok {
		return
	}
	runtime.GC()
	if ok, free := SizeOk(n); !ok {
		panic(fmt.Sprintf("would exceed memory requesting %d objects, %d free", n, free))
	}
}

// Memory checking version of make(). To avoid OOM kills / fatal errors.
func MakeObjectSlice(n int) []Object {
	MustBeOk(n)
	return make([]Object, 0, n)
}
