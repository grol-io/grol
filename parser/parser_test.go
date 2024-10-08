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
			"(1+2)*3",
			"(1 + 2) * 3",
		},
		{
			"1+2 + 3",
			"1 + 2 + 3",
		},
		{
			"   1+2*3   ",
			"1 + 2 * 3",
		},
		{
			"-a * b",
			"-a * b",
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
			"a + b + c",
		},
		{
			"a + b - c",
			"a + b - c",
		},
		{
			"a * b * c",
			"a * b * c",
		},
		{
			"a * b / c",
			"a * b / c",
		},
		{
			"a + b / c",
			"a + b / c",
		},
		{
			"a + b * c + d / e - f",
			"a + b * c + d / e - f",
		},
		{
			"3 + 4; -5 * 5",
			"3 + 4\n-5 * 5", // fixed from the original in the book that was missing the newline
		},
		{
			"5 > 4 == 3 < 4",
			"5 > 4 == 3 < 4",
		},
		{
			"5 < 4 != 3 > 4",
			"5 < 4 != 3 > 4",
		},
		{
			"3 + 4 * 5 == 3 * 1 + 4 * 5",
			"3 + 4 * 5 == 3 * 1 + 4 * 5",
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

		actual := program.PrettyPrint(ast.NewPrintState()).String()
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

func TestFormat(t *testing.T) { //nolint:funlen // long table driven tests.
	tests := []struct {
		input    string
		expected string
		compact  string
	}{
		{
			`a=>b=>a+b`,
			"a => {\n\tb => {\n\t\ta + b\n\t}\n}",
			"a=>{b=>{a+b}}",
		},
		{
			"a=>{b=>{a+b}}", // check it parses back fine, unlike before https://github.com/grol-io/grol/issues/208 fix
			"a => {\n\tb => {\n\t\ta + b\n\t}\n}",
			"a=>{b=>{a+b}}",
		},
		{
			`a=[1,2,3]
a[1]
info["tokens"]`,
			`a = [1, 2, 3]
a[1]
info["tokens"]`,
			`a=[1,2,3]a[1]info["tokens"]`,
		},
		{
			`func fact(a, b, ..) {println(a, b,..)}`,
			"func fact(a, b, ..) {\n\tprintln(a, b, ..)\n}",
			"func fact(a,b,..){println(a,b,..)}",
		},
		{
			`a = 1 /* inline */ b = 2`,
			`a = 1 /* inline */ b = 2`,
			`a=1 b=2`,
		},
		{
			`/* line1 */
			a=1 /* inline */ 2`,
			"/* line1 */\na = 1 /* inline */ 2",
			"a=1 2",
		},
		{ // variant of above at indent level > 0
			`
			func () {
				/* line1 */
				a=1 /* inline */ 2
			}
			`,
			"func() {\n\t/* line1 */\n\ta = 1 /* inline */ 2\n}",
			"func(){a=1 2}",
		},
		{
			`a=1
	/* bc */ b=2`,
			"a = 1\n/* bc */ b = 2",
			"a=1 b=2",
		},
		{
			"a=((1+2)*3)",
			"a = (1 + 2) * 3",
			"a=(1+2)*3",
		},
		{
			"    //    a comment   ", // Should trim right whitespaces (but not ones between // and the comment)
			"//    a comment",
			"",
		},
		{
			"   a = 1+2    // interesting comment about a\nb = 23",
			"a = 1 + 2 // interesting comment about a\nb = 23",
			"a=1+2 b=23",
		},
		{
			"  a = 1+2    // interesting comment about a\n// and one for below:\nb=23",
			"a = 1 + 2 // interesting comment about a\n// and one for below:\nb = 23",
			"a=1+2 b=23",
		},
		{
			`fact=func(n) {    // function example
log("called fact ", n)  // log output
}`,
			"fact = func(n) { // function example\n\tlog(\"called fact \", n) // log output\n}",
			"fact=func(n){log(\"called fact \",n)}",
		},
		{
			`m = {1: "a", "b": 2} c = 3; d=[4,5,6] e = "f"`,
			`m = {1:"a", "b":2}
c = 3
d = [4, 5, 6]
e = "f"`,
			`m={1:"a","b":2}c=3 d=[4,5,6]e="f"`,
		},
		{
			`if (i>3) {10} else if (i>2) {20} else {30}`,
			`if i > 3 {
	10
} else if i > 2 {
	20
} else {
	30
}`,
			`if i>3{10}else if i>2{20}else{30}`,
		},
		{
			`func fact(n) {if (n<=1) {return 1} n*fact(n-1)}`,
			"func fact(n) {\n\tif n <= 1 {\n\t\treturn 1\n\t}\n\tn * fact(n - 1)\n}",
			"func fact(n){if n<=1{return 1}n*fact(n-1)}",
		},
		{
			"1+2+3+4*5==a[i+2]",
			"1 + 2 + 3 + 4 * 5 == a[i + 2]",
			"1+2+3+4*5==a[i+2]",
		},
		{
			"a [1]",
			"a\n[1]",
			"a [1]",
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
			t.Errorf("test [%d] failing for long form\n---input---\n%s\n---expected---\n%s\n---actual---\n%s\n---",
				i, tt.input, tt.expected, actual)
			// sometime differences are tabs or newline so print escaped versions too:
			t.Errorf("test [%d] failing for long form expected %q got %q", i, tt.expected, actual)
		}
		ps := ast.NewPrintState()
		ps.Compact = true
		compact := program.PrettyPrint(ps).String()
		if compact != tt.compact {
			t.Errorf("test [%d] failing for compact\n---input---\n%s\n---expected---\n%s\n---actual---\n%s\n---",
				i, tt.input, tt.compact, compact)
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

func TestInvalidToken(t *testing.T) {
	// This turns into the invalid token which in turn can't be found in prefix map,
	// it used to crash in earlier versions, this is the regression for that crash.
	// it also checks the error message in first line + first column case.
	inp := "\n\n@" // 3rd line.
	l := lexer.New(inp)
	p := parser.New(l)
	_ = p.ParseProgram()
	errs := p.Errors()
	if len(errs) != 1 {
		t.Fatalf("expecting 1 error, got %d", len(errs))
	}
	expected := "3: no prefix parse function for `@` found:\n@\n^"
	if errs[0] != expected {
		t.Errorf("unexpected error: wanted %q got %q", expected, errs[0])
	}
}

// Happened through accidental paste of fib body, panic/crashed.
func TestOddPanic(t *testing.T) {
	inp := `(x) {1`
	l := lexer.New(inp)
	p := parser.New(l)
	statements := p.ParseProgram()
	errs := p.Errors()
	if len(errs) != 1 {
		t.Fatalf("expecting 1 error, got %v", errs)
	}
	out := ast.NewPrintState()
	statements.PrettyPrint(out)
	expected := "x\n"
	actual := out.String()
	if actual != expected {
		t.Errorf("expected %q got %q", expected, actual)
	}
}
