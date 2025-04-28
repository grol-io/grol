package parser

import (
	"strconv"
	"testing"

	"fortio.org/log"
	"grol.io/grol/ast"
	"grol.io/grol/lexer"
)

// Heavily modified from book version to match much improved design.

func TestLetStatements(t *testing.T) {
	/* debug some test(s):
	log.SetLogLevel(log.Debug)
	log.Config.ForceColor = true
	log.SetColorMode()
	*/
	tests := []struct {
		input              string
		expectedIdentifier string
		expectedValue      interface{}
	}{
		{"x = 5;", "x", 5},
		{"y = true;", "y", true},
		{"foobar = y;", "foobar", "y"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain 1 statements. got=%d",
				len(program.Statements))
		}
		stmt := program.Statements[0]
		if !CheckLetStatement(t, stmt, tt.expectedIdentifier) {
			return
		}

		val := stmt.(*ast.InfixExpression).Right
		if !testLiteralExpression(t, val, tt.expectedValue) {
			log.Errf("testLiteralExpression failed for %s got %#v", tt.input, stmt)
			return
		}
	}
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input         string
		expectedValue interface{}
	}{
		{"return 5;", 5},
		{"return true;", true},
		{"return foobar;", "foobar"},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain 1 statements. got=%d",
				len(program.Statements))
		}

		stmt := program.Statements[0]
		returnStmt, ok := stmt.(*ast.ReturnStatement)
		if !ok {
			t.Fatalf("stmt not *ast.ReturnStatement. got=%T", stmt)
		}
		if returnStmt.Literal() != "return" {
			t.Fatalf("returnStmt.Literal not 'return', got %q",
				returnStmt.Literal())
		}
		if testLiteralExpression(t, returnStmt.ReturnValue, tt.expectedValue) {
			return
		}
	}
}

func TestIdentifierExpression(t *testing.T) {
	input := "foobar;"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

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
		t.Errorf("ident.Literal not %s. got=%s", "foobar", ident.Literal())
	}
	if ident.Literal() != "foobar" {
		t.Errorf("ident.Literal not %s. got=%s", "foobar",
			ident.Literal())
	}
}

func TestIntegerLiteralExpression(t *testing.T) {
	input := "5;"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}
	literal, ok := program.Statements[0].(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IntegerLiteral. got=%T",
			program.Statements[0])
	}
	if literal.Val != 5 {
		t.Errorf("literal.Value not %d. got=%d", 5, literal.Val)
	}
	if literal.Literal() != "5" {
		t.Errorf("literal.Literal not %s. got=%s", "5",
			literal.Literal())
	}
}

func TestParsingPrefixExpressions(t *testing.T) {
	prefixTests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"!5;", "!", 5},
		{"-15;", "-", 15},
		{"!foobar;", "!", "foobar"},
		{"-foobar;", "-", "foobar"},
		{"!true;", "!", true},
		{"!false;", "!", false},
	}

	for _, tt := range prefixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
				1, len(program.Statements))
		}

		exp, ok := program.Statements[0].(*ast.PrefixExpression)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.PrefixExpression. got=%T",
				program.Statements[0])
		}
		if exp.Literal() != tt.operator {
			t.Fatalf("exp.Literal() is not '%s'. got=%s",
				tt.value, exp.Literal())
		}
		if !testLiteralExpression(t, exp.Right, tt.value) {
			return
		}
	}
}

func TestParsingInfixExpressions(t *testing.T) {
	infixTests := []struct {
		input      string
		leftValue  interface{}
		operator   string
		rightValue interface{}
	}{
		{"5 + 5;", 5, "+", 5},
		{"5 - 5;", 5, "-", 5},
		{"5 * 5;", 5, "*", 5},
		{"5 / 5;", 5, "/", 5},
		{"5 > 5;", 5, ">", 5},
		{"5 < 5;", 5, "<", 5},
		{"5 == 5;", 5, "==", 5},
		{"5 != 5;", 5, "!=", 5},
		{"foobar + barfoo;", "foobar", "+", "barfoo"},
		{"foobar - barfoo;", "foobar", "-", "barfoo"},
		{"foobar * barfoo;", "foobar", "*", "barfoo"},
		{"foobar / barfoo;", "foobar", "/", "barfoo"},
		{"foobar > barfoo;", "foobar", ">", "barfoo"},
		{"foobar < barfoo;", "foobar", "<", "barfoo"},
		{"foobar == barfoo;", "foobar", "==", "barfoo"},
		{"foobar != barfoo;", "foobar", "!=", "barfoo"},
		{"true == true", true, "==", true},
		{"true != false", true, "!=", false},
		{"false == false", false, "==", false},
	}

	for _, tt := range infixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
				1, len(program.Statements))
		}

		if !testInfixExpression(t, program.Statements[0], tt.leftValue,
			tt.operator, tt.rightValue) {
			return
		}
	}
}

func TestOperatorPrecedenceParsing(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"1/(a*b)",
			"1 / (a * b)", // not 1 / a * b (!)
		},
		{
			"a--",
			"a--",
		},
		{
			"-(5*5)",
			"-(5 * 5)",
		},
		{
			"-5*5",
			"-5 * 5",
		},
		{
			"-a * b",
			"-a * b",
		},
		{
			"!-a",
			"!(-a)",
		},
		{
			"true",
			"true",
		},
		{
			"false",
			"false",
		},
		{
			"3 > 5 == false",
			"3 > 5 == false",
		},
		{
			"3 < 5 == true",
			"3 < 5 == true",
		},
		{
			"1 + (2 + 3) + 4",
			"1 + 2 + 3 + 4",
		},
		{
			"(5 + 5) * 2",
			"(5 + 5) * 2",
		},
		{
			"2 / (5 + 5)",
			"2 / (5 + 5)",
		},
		{
			"(5 + 5) * 2 * (5 + 5)",
			"(5 + 5) * 2 * (5 + 5)",
		},
		{
			"-(5 + 5)",
			"-(5 + 5)",
		},
		{
			"!(true == true)",
			"!(true == true)",
		},
		{
			"a + add((b * c)) + d",
			"a + add(b * c) + d",
		},
		{
			"add(a, b, 1, 2 * 3, 4 + 5, add(6, 7 * 8))",
			"add(a, b, 1, 2 * 3, 4 + 5, add(6, 7 * 8))",
		},
		{
			"add(a + b + c * d / f + g)",
			"add(a + b + c * d / f + g)",
		},
		{
			"a * [1, 2, 3, 4][b * c] * d",
			"a * [1, 2, 3, 4][b * c] * d",
		},
		{
			"add(a * b[2], b[1], 2 * [1, 2][1])",
			"add(a * b[2], b[1], 2 * [1, 2][1])",
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

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

func TestBooleanExpression(t *testing.T) {
	tests := []struct {
		input           string
		expectedBoolean bool
	}{
		{"true;", true},
		{"false;", false},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("program has not enough statements. got=%d",
				len(program.Statements))
		}

		boolean, ok := program.Statements[0].(*ast.Boolean)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.Boolean. got=%T",
				program.Statements[0])
		}
		if boolean.Val != tt.expectedBoolean {
			t.Errorf("boolean.Value not %t. got=%t", tt.expectedBoolean,
				boolean.Val)
		}
	}
}

func TestIfExpression(t *testing.T) {
	input := `if (x < y) { x }`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	exp, ok := program.Statements[0].(*ast.IfExpression)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfExpression. got=%T",
			program.Statements[0])
	}
	if !testInfixExpression(t, exp.Condition, "x", "<", "y") {
		return
	}

	if len(exp.Consequence.Statements) != 1 {
		t.Errorf("consequence is not 1 statements. got=%d\n",
			len(exp.Consequence.Statements))
	}

	consequence, ok := exp.Consequence.Statements[0].(*ast.Identifier)
	if !ok {
		t.Fatalf("Statements[0] is not ast.Identifier. got=%T",
			exp.Consequence.Statements[0])
	}

	if !testIdentifier(t, consequence, "x") {
		return
	}

	if exp.Alternative != nil {
		t.Errorf("exp.Alternative.Statements was not nil. got=%+v", exp.Alternative)
	}
}

func TestIfElseExpression(t *testing.T) {
	input := `if (x < y) { x } else { y }`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	exp, ok := program.Statements[0].(*ast.IfExpression)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfExpression. got=%T",
			program.Statements[0])
	}
	if !testInfixExpression(t, exp.Condition, "x", "<", "y") {
		return
	}

	if len(exp.Consequence.Statements) != 1 {
		t.Errorf("consequence is not 1 statements. got=%d\n",
			len(exp.Consequence.Statements))
	}

	consequence, ok := exp.Consequence.Statements[0].(*ast.Identifier)
	if !ok {
		t.Fatalf("Statements[0] is not ast.Identifier. got=%T",
			exp.Consequence.Statements[0])
	}

	if !testIdentifier(t, consequence, "x") {
		return
	}

	if len(exp.Alternative.Statements) != 1 {
		t.Errorf("exp.Alternative.Statements does not contain 1 statements. got=%d\n",
			len(exp.Alternative.Statements))
	}

	alternative, ok := exp.Alternative.Statements[0].(*ast.Identifier)
	if !ok {
		t.Fatalf("Statements[0] is not ast.NodeStatement. got=%T",
			exp.Alternative.Statements[0])
	}

	if !testIdentifier(t, alternative, "y") {
		return
	}
}

func TestFunctionLiteralParsing(t *testing.T) {
	input := `func(x, y, ..) { x + y; }`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	function, ok := program.Statements[0].(*ast.FunctionLiteral)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionLiteral. got=%T",
			program.Statements[0])
	}
	if !function.Variadic {
		t.Errorf("function literal is not variadic. got=%t", function.Variadic)
	}
	if len(function.Parameters) != 3 {
		t.Fatalf("function literal parameters wrong. want 2, got=%d\n",
			len(function.Parameters))
	}

	testLiteralExpression(t, function.Parameters[0], "x")
	testLiteralExpression(t, function.Parameters[1], "y")

	if len(function.Body.Statements) != 1 {
		t.Fatalf("function.Body.Statements has not 1 statements. got=%d\n",
			len(function.Body.Statements))
	}

	bodyStmt, ok := function.Body.Statements[0].(*ast.InfixExpression)
	if !ok {
		t.Fatalf("function body stmt is not ast.InfixExpression. got=%T",
			function.Body.Statements[0])
	}

	testInfixExpression(t, bodyStmt, "x", "+", "y")
}

func TestFunctionParameterParsing(t *testing.T) {
	tests := []struct {
		input          string
		expectedParams []string
	}{
		{input: "func() {};", expectedParams: []string{}},
		{input: "func(x) {};", expectedParams: []string{"x"}},
		{input: "func(x, y, z) {};", expectedParams: []string{"x", "y", "z"}},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		function := program.Statements[0].(*ast.FunctionLiteral)

		if len(function.Parameters) != len(tt.expectedParams) {
			t.Errorf("length parameters wrong. want %d, got=%d\n",
				len(tt.expectedParams), len(function.Parameters))
		}

		for i, ident := range tt.expectedParams {
			testLiteralExpression(t, function.Parameters[i], ident)
		}
	}
}

func TestCallExpressionParsing(t *testing.T) {
	input := "add(1, 2 * 3, 4 + 5);"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	exp, ok := program.Statements[0].(*ast.CallExpression)
	if !ok {
		t.Fatalf("stmt is not ast.CallExpression. got=%T",
			program.Statements[0])
	}
	if !testIdentifier(t, exp.Function, "add") {
		return
	}

	if len(exp.Arguments) != 3 {
		t.Fatalf("wrong length of arguments. got=%d", len(exp.Arguments))
	}

	testLiteralExpression(t, exp.Arguments[0], 1)
	testInfixExpression(t, exp.Arguments[1], 2, "*", 3)
	testInfixExpression(t, exp.Arguments[2], 4, "+", 5)
}

func TestCallExpressionParameterParsing(t *testing.T) {
	tests := []struct {
		input         string
		expectedIdent string
		expectedArgs  []string
	}{
		{
			input:         "add();",
			expectedIdent: "add",
			expectedArgs:  []string{},
		},
		{
			input:         "add(1);",
			expectedIdent: "add",
			expectedArgs:  []string{"1"},
		},
		{
			input:         "add(1, 2 * 3, 4 + 5);",
			expectedIdent: "add",
			expectedArgs:  []string{"1", "(2*3)", "(4+5)"}, // Debug string is fully ()ized.
		},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		exp, ok := program.Statements[0].(*ast.CallExpression)
		if !ok {
			t.Fatalf("stmt.Expression is not ast.CallExpression. got=%T",
				program.Statements[0])
		}

		if !testIdentifier(t, exp.Function, tt.expectedIdent) {
			return
		}

		if len(exp.Arguments) != len(tt.expectedArgs) {
			t.Fatalf("wrong number of arguments. want=%d, got=%d",
				len(tt.expectedArgs), len(exp.Arguments))
		}

		for i, arg := range tt.expectedArgs {
			got := ast.DebugString(exp.Arguments[i])
			if got != arg {
				t.Errorf("argument %d wrong. want=%q, got=%q", i,
					arg, got)
			}
		}
	}
}

// Kept the name 'let*' but it's now just the `id = val` test.
func CheckLetStatement(t *testing.T, s ast.Node, name string) bool {
	/* What was that testing?
	if s.Literal() != name {
		t.Errorf("s.Literal not %q. got=%q", name, s.Literal())
		return false
	}
	*/
	// Expecting an expression statement containing an infix expression
	letStmt, ok := s.(*ast.InfixExpression)
	if !ok {
		t.Errorf("s not *ast.InfixExpression. got=%T", s)
		return false
	}
	if letStmt.Literal() != "=" {
		t.Errorf("letStmt.Literal() is not '='. got=%q", letStmt.Literal())
		return false
	}
	id, ok := letStmt.Left.(*ast.Identifier)
	if !ok {
		t.Errorf("letStmt.Left not *ast.Identifier. got=%T", letStmt.Left)
		return false
	}
	if id.Literal() != name {
		t.Errorf("letStmt.Name.Value not '%s'. got=%s", name, id.Literal())
		return false
	}

	if id.Literal() != name {
		t.Errorf("letStmt.Name.Literal() not '%s'. got=%s",
			name, id.Literal())
		return false
	}

	return true
}

func testInfixExpression(t *testing.T, exp ast.Node, left interface{},
	operator string, right interface{},
) bool {
	opExp, ok := exp.(*ast.InfixExpression)
	if !ok {
		t.Errorf("exp is not ast.InfixExpression. got=%T(%s)", exp, exp)
		return false
	}

	if !testLiteralExpression(t, opExp.Left, left) {
		return false
	}

	if opExp.Literal() != operator {
		t.Errorf("exp.Literal() is not '%s'. got=%q", operator, opExp.Literal())
		return false
	}

	if !testLiteralExpression(t, opExp.Right, right) {
		return false
	}

	return true
}

func testLiteralExpression(
	t *testing.T,
	exp ast.Node,
	expected interface{},
) bool {
	switch v := expected.(type) {
	case int:
		return testIntegerLiteral(t, exp, int64(v))
	case int64:
		return testIntegerLiteral(t, exp, v)
	case string:
		return testIdentifier(t, exp, v)
	case bool:
		return testBooleanLiteral(t, exp, v)
	}
	t.Errorf("type of exp not handled. got=%T", exp)
	return false
}

func testIntegerLiteral(t *testing.T, il ast.Node, value int64) bool {
	integ, ok := il.(*ast.IntegerLiteral)
	if !ok {
		t.Errorf("il not *ast.IntegerLiteral. got=%T", il)
		return false
	}

	if integ.Val != value {
		t.Errorf("integ.Value not %d. got=%d", value, integ.Val)
		return false
	}

	expectedString := strconv.FormatInt(value, 10)
	if integ.Literal() != expectedString {
		t.Errorf("integ.Literal not %d (%s). got=%s", value, expectedString,
			integ.Literal())
		return false
	}

	return true
}

func testIdentifier(t *testing.T, exp ast.Node, value string) bool {
	ident, ok := exp.(*ast.Identifier)
	if !ok {
		t.Errorf("exp not *ast.Identifier. got=%T", exp)
		return false
	}

	if ident.Literal() != value {
		t.Errorf("ident.Value not %s. got=%s", value, ident.Literal())
		return false
	}

	if ident.Literal() != value {
		t.Errorf("ident.Literal not %s. got=%s", value,
			ident.Literal())
		return false
	}

	return true
}

func testBooleanLiteral(t *testing.T, exp ast.Node, value bool) bool {
	bo, ok := exp.(*ast.Boolean)
	if !ok {
		t.Errorf("exp not *ast.Boolean. got=%T", exp)
		return false
	}

	if bo.Val != value {
		t.Errorf("bo.Value not %t. got=%t", value, bo.Val)
		return false
	}

	expectedString := strconv.FormatBool(value)

	if bo.Literal() != expectedString {
		t.Errorf("bo.Literal not %t (%s). got=%s",
			value, expectedString, bo.Literal())
		return false
	}

	return true
}

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func TestStringLiteralExpression(t *testing.T) {
	input := `"hello\nworld\"abc";`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	literal, ok := program.Statements[0].(*ast.StringLiteral)
	if !ok {
		t.Fatalf("exp not *ast.StringLiteral. got=%T", program.Statements[0])
	}
	expected := "hello\nworld\"abc"
	if literal.Literal() != expected {
		t.Errorf("literal.Value not %q. got=%q", expected, literal.Literal())
	}
}

func TestParsingArrayLiterals(t *testing.T) {
	input := "[1, 2 * 2, 3 + 3]"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	array, ok := program.Statements[0].(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("exp not ast.ArrayLiteral. got=%T", program.Statements[0])
	}
	if len(array.Elements) != 3 {
		t.Fatalf("len(array.Elements) not 3. got=%d", len(array.Elements))
	}

	testIntegerLiteral(t, array.Elements[0], 1)
	testInfixExpression(t, array.Elements[1], 2, "*", 2)
	testInfixExpression(t, array.Elements[2], 3, "+", 3)
}

func TestParsingEmptyArrayLiteral(t *testing.T) {
	input := "[]"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	array, ok := program.Statements[0].(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("exp not ast.NodeStatement. got=%T", program.Statements[0])
	}
	if len(array.Elements) != 0 {
		t.Fatalf("len(array.Elements) not 0. got=%d", len(array.Elements))
	}
}

func TestParsingIndexExpressions(t *testing.T) {
	input := "myArray[1 + 1]"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	indexExp, ok := program.Statements[0].(*ast.IndexExpression)
	if !ok {
		t.Fatalf("exp not ast.IndexExpression. got=%T", program.Statements[0])
	}
	if !testIdentifier(t, indexExp.Left, "myArray") {
		return
	}

	if !testInfixExpression(t, indexExp.Index, 1, "+", 1) {
		return
	}
}

func TestParsingMapLiteralsStringKeys(t *testing.T) {
	input := `{"one": 1, "two": 2, "three": 3}`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	m, ok := program.Statements[0].(*ast.MapLiteral)
	if !ok {
		t.Fatalf("exp is not ast.MapLiteral. got=%T", program.Statements[0])
	}

	if len(m.Pairs) != 3 {
		t.Errorf("map.Pairs has wrong length. got=%d", len(m.Pairs))
	}

	expected := map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
	}

	for key, value := range m.Pairs {
		literal, ok := key.(*ast.StringLiteral)
		if !ok {
			t.Errorf("key is not ast.StringLiteral. got=%T", key)
		}

		expectedValue := expected[literal.Literal()]

		testIntegerLiteral(t, value, expectedValue)
	}
}

func TestParsingEmptyMapLiteral(t *testing.T) {
	input := "{}"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	m, ok := program.Statements[0].(*ast.MapLiteral)
	if !ok {
		t.Fatalf("exp is not ast.MapLiteral. got=%T", program.Statements[0])
	}

	if len(m.Pairs) != 0 {
		t.Errorf("map.Pairs has wrong length. got=%d", len(m.Pairs))
	}
}

func TestParsingMapLiteralsWithExpressions(t *testing.T) {
	input := `{"one": 0 + 1, "two": 10 - 8, "three": 15 / 5}`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	m, ok := program.Statements[0].(*ast.MapLiteral)
	if !ok {
		t.Fatalf("exp is not ast.MapLiteral. got=%T", program.Statements[0])
	}
	if len(m.Pairs) != 3 {
		t.Errorf("map.Pairs has wrong length. got=%d", len(m.Pairs))
	}

	tests := map[string]func(ast.Node){
		"one": func(e ast.Node) {
			testInfixExpression(t, e, 0, "+", 1)
		},
		"two": func(e ast.Node) {
			testInfixExpression(t, e, 10, "-", 8)
		},
		"three": func(e ast.Node) {
			testInfixExpression(t, e, 15, "/", 5)
		},
	}

	for key, value := range m.Pairs {
		literal, ok := key.(*ast.StringLiteral)
		if !ok {
			t.Errorf("key is not ast.StringLiteral. got=%T", key)
			continue
		}

		testFunc, ok := tests[literal.Literal()]
		if !ok {
			t.Errorf("No test function for key %q found", ast.DebugString(literal))
			continue
		}

		testFunc(value)
	}
}

func TestMacroLiteralParsing(t *testing.T) {
	input := `macro(x, y) { x + y; }`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	macro, ok := program.Statements[0].(*ast.MacroLiteral)
	if !ok {
		t.Fatalf("statement is not ast.MacroLiteral. got=%T",
			program.Statements[0])
	}
	if len(macro.Parameters) != 2 {
		t.Fatalf("macro literal parameters wrong. want 2, got=%d\n",
			len(macro.Parameters))
	}

	testLiteralExpression(t, macro.Parameters[0], "x")
	testLiteralExpression(t, macro.Parameters[1], "y")

	if len(macro.Body.Statements) != 1 {
		t.Fatalf("macro.Body.Statements has not 1 statements. got=%d\n",
			len(macro.Body.Statements))
	}

	bodyStmt, ok := macro.Body.Statements[0].(*ast.InfixExpression)
	if !ok {
		t.Fatalf("macro body stmt is not ast.InfixExpression. got=%T",
			macro.Body.Statements[0])
	}

	testInfixExpression(t, bodyStmt, "x", "+", "y")
}

func TestParsingArrayLiteralsWithComments(t *testing.T) {
	input := `[1, // first
		2, // second
		3 // last one
	]`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	array, ok := program.Statements[0].(*ast.ArrayLiteral)
	if !ok {
		t.Fatalf("exp not ast.ArrayLiteral. got=%T", program.Statements[0])
	}
	if len(array.Elements) != 3 {
		t.Fatalf("len(array.Elements) not 3. got=%d", len(array.Elements))
	}

	testIntegerLiteral(t, array.Elements[0], 1)
	testIntegerLiteral(t, array.Elements[1], 2)
	testIntegerLiteral(t, array.Elements[2], 3)
}

func TestParsingFunctionCallsWithComments(t *testing.T) {
	input := `foo(1, // first
		2, // second
		3 // last one
	)`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	call, ok := program.Statements[0].(*ast.CallExpression)
	if !ok {
		t.Fatalf("exp not ast.CallExpression. got=%T", program.Statements[0])
	}
	if len(call.Arguments) != 3 {
		t.Fatalf("len(call.Arguments) not 3. got=%d", len(call.Arguments))
	}

	testIntegerLiteral(t, call.Arguments[0], 1)
	testIntegerLiteral(t, call.Arguments[1], 2)
	testIntegerLiteral(t, call.Arguments[2], 3)
}
