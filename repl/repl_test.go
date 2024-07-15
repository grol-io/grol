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
	if got := repl.EvalString(s); got != expected {
		t.Errorf("EvalString() got\n---\n%s\n---want---\n%s\n---", got, expected)
	}
}
