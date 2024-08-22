package eval_test

import (
	"testing"

	"grol.io/grol/eval"
	"grol.io/grol/object"
)

func TestBigArrayNotHashable(t *testing.T) {
	// This test is to make sure that big arrays are showing as not hashable (so no crash when used later).
	oSlice := object.MakeObjectSlice(object.MaxSmallArray + 1)
	for i := range object.MaxSmallArray + 1 {
		oSlice = append(oSlice, object.Integer{Value: int64(i)})
	}
	a := object.NewArray(oSlice)
	if _, ok := a.(object.BigArray); !ok {
		t.Fatalf("expected big array")
	}
	if object.Hashable(a) {
		t.Fatalf("expected big array to be not hashable")
	}
	m := object.NewMap()
	m = m.Set(object.Integer{Value: 1}, a)
	if object.Hashable(m) {
		t.Fatalf("expected map with big array inside to be not hashable")
	}
	c := eval.NewCache()
	c.Get("func(){}", []object.Object{a})
}
