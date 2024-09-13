package eval

import (
	"testing"

	"grol.io/grol/ast"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
)

func TestDefineMacros(t *testing.T) {
	input := `number = 1
    function = func(x, y) { x + y }
    mymacro = macro(x, y) { x + y; }
	number + 1`

	state := NewState()
	program := testParseProgram(t, input)

	state.DefineMacros(program)

	if len(program.Statements) != 3 {
		t.Fatalf("Wrong number of statements. got=%d: %+v",
			len(program.Statements), program.Statements)
	}

	_, ok := state.macroState.Get("number")
	if ok {
		t.Fatalf("number should not be defined")
	}
	_, ok = state.macroState.Get("function")
	if ok {
		t.Fatalf("function should not be defined")
	}

	obj, ok := state.macroState.Get("mymacro")
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

	if macro.Parameters[0].Value().Literal() != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", macro.Parameters[0])
	}
	if macro.Parameters[1].Value().Literal() != "y" {
		t.Fatalf("parameter is not 'y'. got=%q", macro.Parameters[1])
	}

	expectedBody := "(x+y)" // DebugString adds parens.
	got := ast.DebugString(macro.Body)
	if got != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, got)
	}
}

func testParseProgram(t *testing.T, input string) *ast.Statements {
	l := lexer.New(input)
	p := parser.New(l)
	r := p.ParseProgram()
	errors := p.Errors()
	if len(errors) != 0 {
		t.Errorf("parser has %d error(s)", len(errors))
		for _, msg := range errors {
			t.Errorf("parser error: %s", msg)
		}
		t.FailNow()
	}
	return r
}

func TestExpandMacros(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`m=macro(){}; m()`, // bad macro
			`error("macro should return Quote. got=object.Null ({})")`,
		},
		{
			`
            infixExpression = macro() { quote(1 + 2); };

            infixExpression();
            `,
			`(1 + 2)`,
		},
		{ // bug where first macro use is all of them. #223.
			`
            reverse = macro(a, b) { quote(unquote(b) - unquote(a)) }
            reverse(x,y)
            reverse(2 + 2, 10 - 5)
            `,
			`(y-x) (10 - 5) - (2 + 2)`,
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
		expected := testParseProgram(t, tt.expected)
		program := testParseProgram(t, tt.input)

		state := NewBlankState()
		state.DefineMacros(program)
		expanded := state.ExpandMacros(program)

		expandedStr := ast.DebugString(expanded)
		expectedStr := ast.DebugString(expected)

		if expandedStr != expectedStr {
			t.Errorf("not equal. want=%q, got=%q",
				expectedStr, expandedStr)
		}
	}
}
