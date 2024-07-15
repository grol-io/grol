package repl_test

import (
	"testing"

	"grol.io/grol/repl"
)

func TestEvalString(t *testing.T) {
	s := `
fact=func(n) { // function
    log("called fact", n) // log (timestamped stderr output)
    if (n<=1) {
        return 1
    }
    n*fact(n-1)
}
result = fact(5)
print("Factorial of 5 is", result, "\n") // print to stdout
result`
	expected := `called fact 5
called fact 4
called fact 3
called fact 2
called fact 1
Factorial of 5 is 120` + " \n120\n" // there is an extra space before \n that vscode wants to remove
	if got, errs := repl.EvalString(s); got != expected || len(errs) > 0 {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, got, expected)
	}
}

func TestEvalStringParsingError(t *testing.T) {
	s := `x:=3`
	expected := "\n"
	res, errs := repl.EvalString(s)
	if len(errs) == 0 {
		t.Errorf("EvalString() got no errors (res %q), expected some", res)
	}
	if res != expected {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, res, expected)
	}
}

func TestEvalStringEvalError(t *testing.T) {
	s := `y`
	expected := "<err: <identifier not found: y>>"
	res, errs := repl.EvalString(s)
	if len(errs) == 0 {
		t.Fatalf("EvalString() got no errors (res %q), expected some", res)
	}
	if errs[0] != expected {
		t.Errorf("EvalString() errors\n---\n%s\n---want---\n%s\n---", errs[0], expected)
	}
	expected += "\n" // in output, there is a newline at the end.
	if res != expected {
		t.Errorf("EvalString() result %v\n---\n%s\n---want---\n%s\n---", errs, res, expected)
	}
}
