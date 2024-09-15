package repl_test

import (
	"context"
	"os"
	"testing"

	"grol.io/grol/eval"
	"grol.io/grol/object"
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
println("Factorial of 5 is", result) // print to stdout
result`
	expected := `called fact 5
called fact 4
called fact 3
called fact 2
called fact 1
Factorial of 5 is 120` + "\n120\n" // there is an extra space before \n that vscode wants to remove
	if got, errs, _ := repl.EvalString(s); got != expected || len(errs) > 0 {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, got, expected)
	}
}

func TestEvalMemoPrint(t *testing.T) {
	s := `
fact=func(n) {
	log("logger fact", n) // should be actual executions of the function only
    println("print fact", n) // should get recorded
    if (n<=1) {
        return 1
    }
    n*self(n-1)
}
fact(3)
print("---")
println()
result = fact(5)
println("Factorial of 5 is", result) // print to stdout
result`
	expected := `logger fact 3
logger fact 2
logger fact 1
print fact 3
print fact 2
print fact 1
---
logger fact 5
logger fact 4
print fact 5
print fact 4
print fact 3
print fact 2
print fact 1
Factorial of 5 is 120
120
`
	if got, errs, _ := repl.EvalString(s); got != expected || len(errs) > 0 {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, got, expected)
	}
}

func TestEvalString50(t *testing.T) {
	s := `
fact=func(n) {        // function
    if (n<=1) {
        return 1
    }
    n*fact(n-1)
}
fact(50.)`
	expected := "30414093201713376000000000000000000000000000000000000000000000000\n"
	got, errs, formatted := repl.EvalString(s)
	if got != expected || len(errs) > 0 {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, got, expected)
	}
	// This tests that expression nesting is reset in function call list (ie formatted to `fact(n-1)` instead of `fact((n-1))`)
	// and indirectly the handling of comments on same line as first statement in block.
	expected = `fact = func(n) { // function
	if n <= 1 {
		return 1
	}
	n * fact(n - 1)
}
fact(50.)
`
	if formatted != expected {
		t.Errorf("EvalString() formatted\n---\n%s\n---want---\n%s\n---", formatted, expected)
	}
}

func TestEvalStringParsingError(t *testing.T) {
	s := `	  .`
	expected := ""
	res, errs, formatted := repl.EvalString(s)
	if len(errs) == 0 {
		t.Errorf("EvalString() got no errors (res %q), expected some", res)
	}
	if res != expected {
		t.Errorf("EvalString() got (%v) %q vs %q", errs, res, expected)
	}
	if formatted != s {
		t.Errorf("EvalString() reformatted %q vs %q", formatted, s)
	}
}

func TestEvalStringEvalError(t *testing.T) {
	s := `	 y


	`
	expected := "<err: identifier not found: y>"
	res, errs, formatted := repl.EvalString(s)
	if len(errs) == 0 {
		t.Fatalf("EvalString() got no errors (res %q), expected some", res)
	}
	if errs[0] != expected {
		t.Errorf("EvalString() errors\n---\n%s\n---want---\n%s\n---", errs[0], expected)
	}
	if res != "" {
		t.Errorf("EvalString() result %v\n---\n%s\n---want empty on error---", errs, res)
	}
	if formatted != "y\n" {
		t.Errorf("EvalString() formatted %q expected just \"y\"", formatted)
	}
}

func TestEvalStringPrintNoNil(t *testing.T) {
	s := ` print("ab\nc")  `
	res, errs, formatted := repl.EvalString(s)
	if len(errs) != 0 {
		t.Fatalf("EvalString() got errors expected none %v", errs)
	}
	expected := "ab\nc"
	if res != expected {
		t.Errorf("EvalString() result\n---\n%s\n---want---\n%s\n---", res, expected)
	}
	expected = "print(\"ab\\nc\")\n"
	if formatted != expected {
		t.Errorf("EvalString() formatted %q unexpected vs %q", formatted, expected)
	}
}

func TestPreInputHook(t *testing.T) {
	opts := repl.EvalStringOptions()
	opts.PanicOk = true
	opts.PreInput = func(s *eval.State) {
		s.Extensions["testHook"] = object.Extension{
			Name: "testHook",
			Callback: func(cdata any, _ string, _ []object.Object) object.Object {
				intVal := cdata.(int64)
				return &object.Integer{Value: intVal}
			},
			ClientData: int64(42),
		}
	}
	inp := `testHook()`
	expected := "42\n"
	res, errs, _ := repl.EvalStringWithOption(context.Background(), opts, inp)
	if res != expected || len(errs) > 0 {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, res, expected)
	}
}

func TestLikeDiscordBot(t *testing.T) {
	input := "1"
	expected := "1\n"
	cfg := repl.EvalStringOptions()
	cfg.AutoLoad = true
	// create a .gr file:
	content := `x=()=>{return}`
	os.WriteFile(".gr", []byte(content), 0o644)
	evalres, errs, _ := repl.EvalStringWithOption(context.Background(), cfg, input)
	if evalres != expected {
		t.Errorf("EvalString() got %v\n---\n%s\n---want---\n%s\n---", errs, evalres, expected)
	}
}
