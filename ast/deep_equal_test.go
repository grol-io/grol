package ast_test

import (
	"reflect"
	"testing"
)

type DeepEqualTest struct {
	a, b any
	eq   bool
}

type deepEqualInterface interface {
	Foo()
}
type deepEqualConcrete struct {
	int
}

func (deepEqualConcrete) Foo() {}

var (
	deepEqualConcrete1 = deepEqualConcrete{1}
	deepEqualConcrete2 = deepEqualConcrete{2}
)

var deepEqualTests = []DeepEqualTest{
	// Equalities
	{map[deepEqualInterface]string{deepEqualConcrete1: "a"}, map[deepEqualInterface]string{deepEqualConcrete1: "a"}, true},

	// Inequalities
	{map[deepEqualInterface]string{deepEqualConcrete1: "a"}, map[deepEqualInterface]string{deepEqualConcrete2: "a"}, false},
}

func TestDeepEqual(t *testing.T) {
	for _, test := range deepEqualTests {
		if r := reflect.DeepEqual(test.a, test.b); r != test.eq {
			t.Errorf("DeepEqual(%#v, %#v) = %v, want %v", test.a, test.b, r, test.eq)
		}
	}
}
