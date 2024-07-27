package eval_test

import (
	"testing"

	"grol.io/grol/ast"
	"grol.io/grol/eval"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
)

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5 // is 5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 100 + -50", 0},
		{"5 * 2 + 10", 20},
		{"5 + 2 * 10", 25},
		{"20 + 2 * -10", 0},
		{"50 / 2 * 2 + 10", 60},
		{"2 * (5 + 10)", 30},
		{"3 * 3 * 3 + 10", 37},
		{"3 * (3 * 3) + 10", 37},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
		{"15 % 5", 0},
		{"16 % 5", 1},
		{"17 % 5", 2},
		{"18 % 5", 3},
		{"19 % 5", 4},
		{"20 % -5", 0},
		{"-21 % 5", -1},
		{`fact = func(n) {if (n<2) {return 1} n*fact(n-1)}; fact(5)`, 120},
	}

	for i, tt := range tests {
		evaluated := testEval(t, tt.input)
		r := testIntegerObject(t, evaluated, tt.expected)
		if !r {
			t.Logf("test %d input: %s failed integer %d", i, tt.input, tt.expected)
		}
	}
}

func testEval(t *testing.T, input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser has %d error(s): %v", len(p.Errors()), p.Errors())
	}
	s := eval.NewState() // each test starts anew.
	return s.Eval(program)
}

func testIntegerObject(t *testing.T, obj object.Object, expected int64) bool {
	result, ok := obj.(object.Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d",
			result.Value, expected)
		return false
	}

	return true
}

func TestEvalBooleanExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 < 1", false},
		{"1 > 1", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"1 == 2", false},
		{"1 != 2", true},
		{"true == true", true},
		{"false == false", true},
		{"true == false", false},
		{"true != false", true},
		{"false != true", true},
		{"(1 < 2) == true", true},
		{"(1 < 2) == false", false},
		{"(1 > 2) == true", false},
		{"(1 > 2) == false", true},
		{`"hello" == "world"`, false},
		{`"hello" == "hello"`, true},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestBangOperator(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"!true", false},
		{"!false", true},
		// {"!5", false},
		{"!!true", true},
		{"!!false", false},
		// {"!!5", true},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func testBooleanObject(t *testing.T, obj object.Object, expected bool) {
	result, ok := obj.(object.Boolean)
	if !ok {
		t.Errorf("object is not Boolean. got=%T (%+v)", obj, obj)
		return
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t",
			result.Value, expected)
	}
}

func TestIfElseExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"", nil},
		{"if (true) {return} else {return 3}", nil},
		{"if (true) {return 6}", 6},
		{"if (false) {return} else {return 3}", 3},
		{"if (false) { 10 }", nil},
		{"if (true) { 10 }", 10},
		//  {"if (1) { 10 }", 10},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 > 2) { 10 }", nil},
		{"if (1 > 2) { 10 } else { 20 }", 20},
		{"if (1 < 2) { 10 } else { 20 }", 10},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func testNullObject(t *testing.T, obj object.Object) {
	if obj != object.NULL {
		t.Errorf("object is not NULL. got=%#v", obj)
	}
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"return 10;", 10},
		{"return 10; 9;", 10},
		{"return 2 * 5; 9;", 10},
		{"9; return 2 * 5; 9;", 10},
		{
			`
		if (10 > 1) {
		  if (10 > 1) {
			return 10;
		  }

		  return 1;
		}`,
			10,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{
			"f=func(x,y) {x+y}; f(1)",
			"<wrong number of arguments. got=1, want=2>",
		},
		{
			"5 + true;",
			"<operation on non integers left=5 right=true>",
		},
		{
			"5 + true; 5;",
			"<operation on non integers left=5 right=true>",
		},
		{
			"-true",
			"<minus of true>",
		},
		{
			"true + false;",
			"<operation on non integers left=true right=false>",
		},
		{
			"5; true + false; 5",
			"<operation on non integers left=true right=false>",
		},
		{
			"if (10 > 1) { true + false; }",
			"<operation on non integers left=true right=false>",
		},
		{
			`
if (10 > 1) {
  if (10 > 1) {
    return true + false;
  }

  return 1;
}
`,
			"<operation on non integers left=true right=false>",
		},
		{
			"foobar",
			"<identifier not found: foobar>",
		},
		{
			`"Hello" - "World"`,
			"<unknown operator: STRING - STRING>",
		},
		{
			`{"name": "Monkey"}[func(x) { x }];`,
			"FUNC not usable as map key",
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)

		errObj, ok := evaluated.(object.Error)
		if !ok {
			t.Errorf("no error object returned. got=%T(%+v)",
				evaluated, evaluated)
			continue
		}

		if errObj.Value != tt.expectedMessage {
			t.Errorf("wrong error message. expected=%q, got=%q",
				tt.expectedMessage, errObj.Value)
		}
	}
}

func TestLetStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"a = 5; a;", 5},
		{"a = 5 * 5; a;", 25},
		{"a = 5; b = a; b;", 5},
		{"a = 5; b = a; c = a + b + 5; c;", 15},
		// and let-free:
		{"x = 3+2; x", 5},
		{`y = "ab"=="a"+"b"; if (y) {1} else {2}`, 1},
	}

	for _, tt := range tests {
		testIntegerObject(t, testEval(t, tt.input), tt.expected)
	}
}

func TestFunctionObject(t *testing.T) {
	input := "func(x) { x + 2; };"

	evaluated := testEval(t, input)
	fn, ok := evaluated.(object.Function)
	if !ok {
		t.Fatalf("object is not Function. got=%T (%+v)", evaluated, evaluated)
	}

	if len(fn.Parameters) != 1 {
		t.Fatalf("function has wrong parameters. Parameters=%+v",
			fn.Parameters)
	}

	if ast.DebugString(fn.Parameters[0]) != "x" {
		t.Fatalf("parameter is not 'x'. got=%q", fn.Parameters[0])
	}
	expectedBody := "x + 2\n"
	got := ast.DebugString(fn.Body)
	if got != expectedBody {
		t.Fatalf("body is not %q. got=%q", expectedBody, got)
	}
}

func TestFunctionApplication(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"identity = func(x) { x; }; identity(5);", 5},
		{"identity = func(x) { return x; }; identity(5);", 5},
		{"double = func(x) { x * 2; }; double(5);", 10},
		{"add = func(x, y) { x + y; }; add(5, 5);", 10},
		{"add = func(x, y) { x + y; }; add(5 + 5, add(5, 5));", 20},
		{"func(x) { x; }(5)", 5},
	}
	for _, tt := range tests {
		testIntegerObject(t, testEval(t, tt.input), tt.expected)
	}
}

func TestClosures(t *testing.T) {
	input := `
newAdder = func(x) {
  func(y) { x + y };
};

addTwo = newAdder(2);
addTwo(2);`

	testIntegerObject(t, testEval(t, input), 4)
}

func TestStringLiteral(t *testing.T) {
	input := `"Hello World!"`

	evaluated := testEval(t, input)
	str, ok := evaluated.(object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello World!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
}

func TestStringConcatenation(t *testing.T) {
	input := `"Hello" + " " + "World!"`

	evaluated := testEval(t, input)
	str, ok := evaluated.(object.String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}

	if str.Value != "Hello World!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
	// check quote on string rep/double eval
	inspect := str.Inspect()
	expected := `"Hello World!"`
	if inspect != expected {
		t.Errorf("String has wrong Inspect. got=%s want %s", inspect, expected)
	}
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{`len("")`, 0},
		{`len("four")`, 4},
		{`len("hello world")`, 11},
		{`len(1)`, "len: not supported on INTEGER"},
		{`len("one", "two")`, "len: wrong number of arguments. got=2, want=1"},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)

		switch expected := tt.expected.(type) {
		case int:
			testIntegerObject(t, evaluated, int64(expected))
		case string:
			errObj, ok := evaluated.(object.Error)
			if !ok {
				t.Errorf("object is not Error. got=%T (%+v)",
					evaluated, evaluated)
				continue
			}
			if errObj.Value != expected {
				t.Errorf("wrong error message. expected=%q, got=%q",
					expected, errObj.Value)
			}
		}
	}
}

func TestArrayLiterals(t *testing.T) {
	input := "[1, 2 * 2, 3 + 3]"

	evaluated := testEval(t, input)
	result, ok := evaluated.(object.Array)
	if !ok {
		t.Fatalf("object is not Array. got=%T (%+v)", evaluated, evaluated)
	}

	if len(result.Elements) != 3 {
		t.Fatalf("array has wrong num of elements. got=%d",
			len(result.Elements))
	}

	testIntegerObject(t, result.Elements[0], 1)
	testIntegerObject(t, result.Elements[1], 4)
	testIntegerObject(t, result.Elements[2], 6)
}

func TestArrayIndexExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			"[1, 2, 3][0]",
			1,
		},
		{
			"[1, 2, 3][1]",
			2,
		},
		{
			"[1, 2, 3][2]",
			3,
		},
		{
			"i = 0; [1][i]",
			1,
		},
		{
			"[1, 2, 3][1 + 1]",
			3,
		},
		{
			"myArray = [1, 2, 3]; myArray[2]",
			3,
		},
		{
			"myArray = [1, 2, 3]; myArray[0] + myArray[1] + myArray[2];",
			6,
		},
		{
			"myArray = [1, 2, 3]; i = myArray[0]; myArray[i]",
			2,
		},
		{
			"[1, 2, 3][3]",
			nil,
		},
		{
			"[1, 2, 3][-1]",
			nil,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestMapLiterals(t *testing.T) {
	input := `two = "two"
    {
        "one": 10 - 9,
        two: 1 + 1,
        "thr" + "ee": 6 / 2,
        4: 4,
        true: 5,
        false: 6
    }`

	evaluated := testEval(t, input)
	result, ok := evaluated.(object.Map)
	if !ok {
		t.Fatalf("Eval didn't return Map. got=%T (%+v)", evaluated, evaluated)
	}

	expected := map[object.Object]int64{
		object.String{Value: "one"}:   1,
		object.String{Value: "two"}:   2,
		object.String{Value: "three"}: 3,
		object.Integer{Value: 4}:      4,
		object.TRUE:                   5,
		object.FALSE:                  6,
	}

	if len(result) != len(expected) {
		t.Fatalf("Map has wrong num of pairs. got=%d", len(result))
	}

	for expectedKey, expectedValue := range expected {
		v, ok := result[expectedKey]
		if !ok {
			t.Errorf("no value for given key %#v in Pairs", expectedKey)
		}

		testIntegerObject(t, v, expectedValue)
	}
}

func TestMapIndexExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{
			`{"foo": 5}["foo"]`,
			5,
		},
		{
			`{"foo": 5}["bar"]`,
			nil,
		},
		{
			`key = "foo"; {"foo": 5}[key]`,
			5,
		},
		{
			`{}["foo"]`,
			nil,
		},
		{
			`{5: 5}[5]`,
			5,
		},
		{
			`{true: 5}[true]`,
			5,
		},
		{
			`{false: 5}[false]`,
			5,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestQuote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`quote(5)`,
			`5`,
		},
		{
			`quote(5 + 8)`,
			`5 + 8`,
		},
		{
			`quote(foobar)`,
			`foobar`,
		},
		{
			`quote(foobar + barfoo)`,
			`foobar + barfoo`,
		},
	}

	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		quote, ok := evaluated.(object.Quote)
		if !ok {
			t.Fatalf("expected *object.Quote. got=%T (%+v)",
				evaluated, evaluated)
		}

		if quote.Node == nil {
			t.Fatalf("quote.Node is nil")
		}

		got := ast.DebugString(quote.Node)
		if got != tt.expected {
			t.Errorf("not equal. got=%q, want=%q", got, tt.expected)
		}
	}
}

func TestQuoteUnquote(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			`quote(unquote(4))`,
			`4`,
		},
		{
			`quote(unquote(4 + 4))`,
			`8`,
		},
		{
			`quote(8 + unquote(4 + 4))`,
			`8 + 8`,
		},
		{
			`quote(unquote(4 + 4) + 8)`,
			`8 + 8`,
		},
		{
			`foobar = 8;
            quote(foobar)`,
			`foobar`,
		},
		{
			`foobar = 8;
            quote(unquote(foobar))`,
			`8`,
		},
		{
			`quote(unquote(true))`,
			`true`,
		},
		{
			`quote(unquote(true == false))`,
			`false`,
		},
		{
			`quote(unquote(quote(4 + 4)))`,
			`4 + 4`,
		},
		{
			`quotedInfixExpression = quote(4 + 4);
            quote(unquote(4 + 4) + unquote(quotedInfixExpression))`,
			`8 + (4 + 4)`,
		},
	}
	for _, tt := range tests {
		evaluated := testEval(t, tt.input)
		quote, ok := evaluated.(object.Quote)
		if !ok {
			t.Fatalf("expected *object.Quote. got=%T (%+v)",
				evaluated, evaluated)
		}

		if quote.Node == nil {
			t.Fatalf("quote.Node is nil")
		}

		got := ast.DebugString(quote.Node)
		if got != tt.expected {
			t.Errorf("not equal. got=%q, want=%q", got, tt.expected)
		}
	}
}

func TestEvalFloatExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{".5 // is 0.5", 0.5},
		{"3./2", 1.5},
		{".1", 0.1},
		{".2", 0.2},
		{".3", 0.3},
		{"0.5*3", 1.5},
		{"0.5*6", 3},
	}

	for i, tt := range tests {
		evaluated := testEval(t, tt.input)
		r := testFloatObject(t, evaluated, tt.expected)
		if !r {
			t.Logf("test %d input: %s failed float %f", i, tt.input, tt.expected)
		}
	}
}

// == on float is usually not a good thing...
func testFloatObject(t *testing.T, obj object.Object, expected float64) bool {
	result, ok := obj.(object.Float)
	if !ok {
		t.Errorf("object is not float. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%f, want=%f", result.Value, expected)
		return false
	}

	return true
}
