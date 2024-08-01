package parser_test

import (
	"testing"

	"grol.io/grol/ast"
	"grol.io/grol/lexer"
	"grol.io/grol/parser"
)

func Test_LetStatements(t *testing.T) {
	input := `
x = 5;
y = 10;
foobar = 838383;
`
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()
	checkParserErrors(t, input, p)
	if program == nil {
		t.Fatalf("ParseProgram() returned nil")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d",
			len(program.Statements))
	}

	tests := []struct {
		expectedIdentifier string
	}{
		{"x"},
		{"y"},
		{"foobar"},
	}

	for i, tt := range tests {
		stmt := program.Statements[i]
		if !parser.CheckLetStatement(t, stmt, tt.expectedIdentifier) {
			return
		}
	}
}

func checkParserErrors(t *testing.T, input string, p *parser.Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d error(s) for %q", len(errors), input)
	for _, msg := range errors {
		t.Errorf("parser error: %s", msg)
	}
	t.FailNow()
}

func Test_ReturnStatements(t *testing.T) {
	input := `
return 5;
return 10;
return 993322;
`
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()
	checkParserErrors(t, input, p)

	if len(program.Statements) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d",
			len(program.Statements))
	}

	for _, stmt := range program.Statements {
		returnStmt, ok := stmt.(*ast.ReturnStatement)
		if !ok {
			t.Errorf("stmt not *ast.ReturnStatement. got=%T", stmt)
			continue
		}
		if returnStmt.Literal() != "return" {
			t.Errorf("returnStmt.TokenLiteral not 'return', got %q",
				returnStmt.Literal())
		}
	}
}

func Test_IdentifierExpression(t *testing.T) {
	input := "foobar;"

	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	checkParserErrors(t, input, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}
	ident, ok := program.Statements[0].(*ast.Identifier)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.Identifier. got=%T",
			program.Statements[0])
	}
	if ident.Literal() != "foobar" {
		t.Errorf("ident.Value not %s. got=%s", "foobar", ident.Literal())
	}
	if ident.Literal() != "foobar" {
		t.Errorf("ident.TokenLiteral not %s. got=%s", "foobar",
			ident.Literal())
	}
}

func Test_OperatorPrecedenceParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"1+2 + 3",
			"(1 + 2) + 3",
		},
		{
			"   1+2*3   ",
			"1 + (2 * 3)",
		},
		{
			"-a * b",
			"(-a) * b",
		},
		{
			"!-a",
			"!(-a)", // or maybe !-a - it's more compact but... less readable?
		},
		{
			"-(-a)",
			"-(-a)",
		},
		{
			"a--",
			"a--",
		},
		{
			"a + b + c",
			"(a + b) + c",
		},
		{
			"a + b - c",
			"(a + b) - c",
		},
		{
			"a * b * c",
			"(a * b) * c",
		},
		{
			"a * b / c",
			"(a * b) / c",
		},
		{
			"a + b / c",
			"a + (b / c)",
		},
		{
			"a + b * c + d / e - f",
			"((a + (b * c)) + (d / e)) - f",
		},
		{
			"3 + 4; -5 * 5",
			"3 + 4\n(-5) * 5", // fixed from the original in the book that was missing the newline
		},
		{
			"5 > 4 == 3 < 4",
			"(5 > 4) == (3 < 4)",
		},
		{
			"5 < 4 != 3 > 4",
			"(5 < 4) != (3 > 4)",
		},
		{
			"3 + 4 * 5 == 3 * 1 + 4 * 5",
			"(3 + (4 * 5)) == ((3 * 1) + (4 * 5))",
		},
		{
			"x = 41 * 6",
			"x = 41 * 6", // = doesn't trigger expression level so it's more natural to read.
		},
		{
			"foo = func(a,b) {return a+b}",
			"foo = func(a, b) {\n\treturn a + b\n}",
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		program := p.ParseProgram()
		checkParserErrors(t, tt.input, p)

		actual := ast.DebugString(program)
		last := actual[len(actual)-1]
		if actual[len(actual)-1] != '\n' {
			t.Errorf("expecting newline at end of program output, not found, got %q", last)
		} else {
			actual = actual[:len(actual)-1] // remove the last newline
		}
		if actual != tt.expected {
			t.Errorf("expected=%q, got=%q", tt.expected, actual)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`a = 1 /* inline */ b = 2`,
			`a = 1 /* inline */ b = 2`,
		},
		{
			`/* line1 */
			a=1 /* inline */ 2`,
			"/* line1 */\na = 1 /* inline */ 2",
		},
		{ // variant of above at indent level > 0
			`
			func () {
				/* line1 */
				a=1 /* inline */ 2
			}
			`,
			"func() {\n\t/* line1 */\n\ta = 1 /* inline */ 2\n}",
		},
		{
			`a=1
	/* bc */ b=2`,
			"a = 1\n/* bc */ b = 2",
		},
		{
			"a=((1+2)*3)",
			"a = (1 + 2) * 3",
		},
		{
			"    //    a comment   ", // Should trim right whitespaces (but not ones between // and the comment)
			"//    a comment",
		},
		{
			"   a = 1+2    // interesting comment about a\nb = 23",
			"a = 1 + 2 // interesting comment about a\nb = 23",
		},
		{
			"  a = 1+2    // interesting comment about a\n// and one for below:\nb=23",
			"a = 1 + 2 // interesting comment about a\n// and one for below:\nb = 23",
		},
		{
			`fact=func(n) {    // function example
log("called fact ", n)  // log output
}`,
			"fact = func(n) { // function example\n\tlog(\"called fact \", n) // log output\n}",
		},
	}
	for i, tt := range tests {
		l := lexer.New(tt.input)
		p := parser.New(l)
		program := p.ParseProgram()
		checkParserErrors(t, tt.input, p)
		if p.ContinuationNeeded() {
			t.Errorf("[%d] expecting no continuation needed, got true", i)
		}
		actual := program.PrettyPrint(ast.NewPrintState()).String()
		last := actual[len(actual)-1]
		if actual[len(actual)-1] != '\n' {
			t.Errorf("[%d] expecting newline at end of program output, not found, got %q", i, last)
		} else {
			actual = actual[:len(actual)-1] // remove the last newline
		}
		if actual != tt.expected {
			t.Errorf("test [%d] failing for\n---input---\n%s\n---expected---\n%s\n---actual---\n%s\n---",
				i, tt.input, tt.expected, actual)
		}
	}
}

func TestIncompleteBlockComment(t *testing.T) {
	tests := []struct {
		input    string
		complete bool
	}{
		{
			"a = 42 /* start of block\n\n",
			false,
		},
	}
	for i, tt := range tests {
		l := lexer.NewLineMode(tt.input)
		p := parser.New(l)
		_ = p.ParseProgram()
		if tt.complete {
			checkParserErrors(t, tt.input, p)
			if p.ContinuationNeeded() {
				t.Errorf("[%d] expecting no continuation needed, got true", i)
			}
		} else if !p.ContinuationNeeded() {
			t.Errorf("[%d] expecting continuation needed, got false", i)
		}
	}
}
