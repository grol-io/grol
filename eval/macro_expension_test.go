package eval

import (
	"testing"

	"grol.io/grol/ast"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
)

func TestDefineMacros(t *testing.T) {
	// TODO: make it work without `let` or... keep just for macros?
	input := `number = 1
    function = func(x, y) { x + y }
    mymacro = macro(x, y) { x + y; }
	number + 1`

	state := NewState()
	env := state.env
	program := testParseProgram(input)

	state.DefineMacros(program)

	if len(program.Statements) != 3 {
		t.Fatalf("Wrong number of statements. got=%d: %+v",
			len(program.Statements), program.Statements)
	}

	_, ok := env.Get("number")
	if ok {
		t.Fatalf("number should not be defined")
	}
	_, ok = env.Get("function")
	if ok {
		t.Fatalf("function should not be defined")
	}

	obj, ok := env.Get("mymacro")
	if !ok {
		t.Fatalf("macro not in environment.")
	}

	macro, ok := obj.(*object.Macro)
	if !ok {
		t.Fatalf("object is not Macro. got=%T (%+v)", obj, obj)
	}

	if len(macro.Parameters) != 2 {
		t.Fatalf("Wrong number of macro parameters. got=%d",
			len(macro.Parameters))
	}

	if macro.Parameters[0].String() != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", macro.Parameters[0])
	}
	if macro.Parameters[1].String() != "y" {
		t.Fatalf("parameter is not 'y'. got=%q", macro.Parameters[1])
	}

	expectedBody := "{\n(x + y)\n}"

	if macro.Body.String() != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, macro.Body.String())
	}
}

func testParseProgram(input string) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)
	return p.ParseProgram()
}

func TestExpandMacros(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`
            infixExpression = macro() { quote(1 + 2); };

            infixExpression();
            `,
			`(1 + 2)`,
		},
		{
			`
            reverse = macro(a, b) { quote(unquote(b) - unquote(a)); };

            reverse(2 + 2, 10 - 5);
            `,
			`(10 - 5) - (2 + 2)`,
		},
		{
			`
            unless = macro(condition, consequence, alternative) {
                quote(if (!(unquote(condition))) {
                    unquote(consequence);
                } else {
                    unquote(alternative);
                });
            };

            unless(10 > 5, puts("not greater"), puts("greater"));
            `,
			`if (!(10 > 5)) { puts("not greater") } else { puts("greater") }`,
		},
	}

	for _, tt := range tests {
		expected := testParseProgram(tt.expected)
		program := testParseProgram(tt.input)

		state := NewState()
		state.DefineMacros(program)
		expanded := state.ExpandMacros(program)

		if expanded.String() != expected.String() {
			t.Errorf("not equal. want=%q, got=%q",
				expected.String(), expanded.String())
		}
	}
}
