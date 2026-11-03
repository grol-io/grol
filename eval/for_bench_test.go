package eval

import (
	"strconv"
	"testing"

	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
)

func BenchmarkArrayFor(b *testing.B) {
	for _, n := range []int{0, 1, 8, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			p := parser.New(lexer.New("for value := values {}"))
			program := p.ParseProgram()
			if errs := p.Errors(); len(errs) != 0 {
				b.Fatal(errs)
			}
			elements := make([]object.Object, n)
			for i := range elements {
				elements[i] = object.Integer{Value: int64(i)}
			}
			s := NewState()
			s.env.CreateOrSet("values", object.NewArray(elements), true)
			b.ReportAllocs()
			for b.Loop() {
				if result := s.EvalToplevel(program); result != object.NULL {
					b.Fatal(result.Inspect())
				}
			}
		})
	}
}
