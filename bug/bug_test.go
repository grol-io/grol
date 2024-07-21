package bug_test

import (
	"testing"

	"grol.io/grol/repl"
)

func TestEvalString50(t *testing.T) {
	s := `
fact=func(n) { // function
    if (n<=1) {
        return 1
    }
    n*fact(n-1)
}
fact(50.)`
	expected := "30414093201713376000000000000000000000000000000000000000000000000\n"
	if got, errs := repl.EvalString(s); got != expected || len(errs) > 0 {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, got, expected)
	}
}
