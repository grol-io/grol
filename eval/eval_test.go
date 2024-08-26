package eval_test

import (
	"os"
	"testing"

	"grol.io/grol/ast"
	"grol.io/grol/eval"
	"grol.io/grol/extensions"
	"grol.io/grol/lexer"
	"grol.io/grol/object"
	"grol.io/grol/parser"
	"grol.io/grol/repl"
)

func TestMain(m *testing.M) {
	err := extensions.Init(nil)
	if err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{`f=func(x) {len(x)}; f([1,2,3])`, 3},
		// shorthand syntax for function declaration:
		{`func f(x) {len(x)}; f([1,2,3])`, 3},
		{"(3)\n(4)", 4}, // expression on new line should be... new.
		{"5 // is 5", 5},
		{"10", 10},
		{"-5", -5},
		{"-10", -10},
		{"5 + 5 + 5 + 5 - 10 /* some block comment */", 10},
		/* These don't work, we need to make comment a identity operator or prune them entirely from the AST. */
		// {"5 + /* block comment in middle of expression */ 2", 7},
		// {" - /* inline of prefix */ 5", -5},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"-50 + 1_0_0 + -50", 0},
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
		{`a=2; b=3; r=a+5*b++`, 17},
		{`a=2; b=3; r=a+5*b++;b`, 4},
		{`a=2; b=3; r=a+5*(b++)+b;`, 21}, // left to right eval, yet not that not well defined behavior.
		{`a=2; b=3; r=b+5*(b++)+a;`, 20}, // because that solo b is evaluated last, after the b++ - not well defined behavior.
		{`a=2; b=3; r=a+5*b+++b;`, 21},   // parentheses are not technically needed here though... this is rather un readable.
		{`i=1; i++ + ++i; i`, 3},
		{`i=1; i++ + ++i`, 4},
		{`i=1; i++-++i; i`, 3},
		{`i=1; i++-++i`, -2},
		{`i=1; i--+--i; i`, -1},
		{`i=1; i--+--i`, 0},
		// TODO: doesn't work without a space or ()
		// {`i=1; i+++++i; i`, 4},
		{`// IIFE factorial
func(n) {
	if n <= 1 {
		return 1
	}
	n * self(n - 1)
}(5)
`, 120},
		{`ONE=1;ONE`, 1},
		{`ONE=1;ONE=1`, 1}, // Ok to reassign CONSTANT if it's to same value.
		{`myid=23; func test(n) {if (n==2) {myid=42}; if (n==1) {return myid}; test(n-1)}; test(3)`, 42}, // was 23 before
		{
			`func FACT(n){if n<=1 {return 1}; n*FACT(n-1)};FACT(5)`, // Recursion on CONSTANT function should not error
			120,
		},
	}
	for i, tt := range tests {
		evaluated := testEval(t, tt.input)
		r := testIntegerObject(t, evaluated, tt.expected)
		if !r {
			t.Logf("test %d input: %s failed to eval to integer %d", i, tt.input, tt.expected)
		}
	}
}

func testEval(t *testing.T, input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parser has %d error(s) for %q: %v", len(p.Errors()), input, p.Errors())
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
		{`info["all_ids"] == info.all_ids`, true},
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
		{"i=1; if (i>=3) {10} else if (i>=2) {20} else {30}", 30},
		{"i=2; if (i>=3) {10} else if (i>=2) {20} else {30}", 20},
		{"i=3; if (i>=3) {10} else if (i>=2) {20} else {30}", 10},
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
			`C1=1;C1=2`,
			"attempt to change constant C1 from 1 to 2",
		},
		{
			`func x(FOO) {log("x",FOO,PI);if FOO<=1 {return FOO} x(FOO-1)};x(2)`,
			"attempt to change constant FOO from 2 to 1",
		},
		{
			`func FOO(x){x}; func FOO(x){x+1}`,
			"attempt to change constant FOO from func FOO(x){x} to func FOO(x){x+1}",
		},
		{
			`ONE=1;ONE=2`,
			"attempt to change constant ONE from 1 to 2",
		},
		{
			`ONE=1;ONE--`,
			"attempt to change constant ONE from 1 to 0",
		},
		{
			`PI++`,
			"attempt to change constant PI from 3.141592653589793 to 4.141592653589793",
		},
		{
			`ONE=1;func f(x){func ff(y) {ONE=y} ff(x)};f(3)`,
			"attempt to change constant ONE from 1 to 3",
		},
		{
			"myfunc=func(x,y) {x+y}; myfunc(1)",
			"wrong number of arguments for myfunc. got=1, want=2",
		},
		{
			"5 + true;",
			"operation on non integers left=5 right=true",
		},
		{
			"5 + true; 5;",
			"operation on non integers left=5 right=true",
		},
		{
			"-true",
			"minus of true",
		},
		{
			"true + false;",
			"operation on non integers left=true right=false",
		},
		{
			"5; true + false; 5",
			"operation on non integers left=true right=false",
		},
		{
			"if (10 > 1) { true + false; }",
			"operation on non integers left=true right=false",
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
			"operation on non integers left=true right=false",
		},
		{
			"foobar",
			"identifier not found: foobar",
		},
		{
			`"Hello" - "World"`,
			"unknown operator: STRING MINUS STRING",
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
	expectedBody := "x+2"
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
	result, ok := evaluated.(object.SmallArray)
	if !ok {
		t.Fatalf("object is not Array. got=%T (%+v)", evaluated, evaluated)
	}
	els := object.Elements(result)
	if len(els) != 3 {
		t.Fatalf("array has wrong num of elements. got=%d",
			len(els))
	}

	testIntegerObject(t, els[0], 1)
	testIntegerObject(t, els[1], 4)
	testIntegerObject(t, els[2], 6)
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
			3,
		},
		{
			"[1, 2, 3][-4]",
			nil,
		},
		{
			"len([1, 2, 3, 4][2:])",
			2,
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

	if result.Len() != len(expected) {
		t.Fatalf("Map has wrong num of pairs. got=%d", result.Len())
	}

	for expectedKey, expectedValue := range expected {
		v, ok := result.Get(expectedKey)
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
			`{"foo": 5}.foo`, // dot notation
			5,
		},
		{
			`{"foo": 5}["bar"]`,
			nil,
		},
		{
			`{"foo": 5}.bar`,
			nil,
		},
		{
			`key = "foo"; {"foo": 5}[key]`,
			5,
		},
		{
			`key = "foo"; {"foo": 5}.key`, // doesn't work (on puprpose), dot notation is string not eval.
			nil,
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
			`5+8`,
		},
		{
			`quote(foobar)`,
			`foobar`,
		},
		{
			`quote(foobar + barfoo)`,
			`foobar+barfoo`,
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
			`8+8`,
		},
		{
			`quote(unquote(4 + 4) + 8)`,
			`8+8`,
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
			`4+4`,
		},
		{
			`quotedInfixExpression = quote(4 + 4);
            quote(unquote(4 + 4) + unquote(quotedInfixExpression))`,
			`8+(4+4)`,
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
		{"-.4", -0.4},
		{"0.5*3", 1.5},
		{"0.5*6", 3},
		{`a=3.1; a--; a`, 2.1},
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

func TestExtension(t *testing.T) {
	err := extensions.Init(nil)
	if err != nil {
		t.Fatalf("extensions.Init() failed: %v", err)
	}
	input := `pow`
	evaluated := testEval(t, input)
	expected := "pow(float, float)"
	if evaluated.Inspect() != expected {
		t.Errorf("object has wrong value. got=%s, want=%s", evaluated.Inspect(), expected)
	}
	input = `pow(2,10)`
	evaluated = testEval(t, input)
	testFloatObject(t, evaluated, 1024)
	input = `round(2.7)`
	evaluated = testEval(t, input)
	testFloatObject(t, evaluated, 3)
	input = `cos(PI)` // somehow getting 'exact' -1 for cos() but some 1e-16 for sin().
	evaluated = testEval(t, input)
	testFloatObject(t, evaluated, -1)
	input = `sprintf("%d %s %g", 42, "ab\ncd", pow(2, 43))`
	evaluated = testEval(t, input)
	expected = "42 ab\ncd 8.796093022208e+12" // might be brittle the %g output of float64.
	actual, ok := evaluated.(object.String)
	if !ok {
		t.Errorf("object is not string. got=%T (%+v)", evaluated, evaluated)
	}
	if actual.Value != expected {
		t.Errorf("object has wrong value. got=%q, want=%q", actual, expected)
	}
	input = `m={1.5:"a",2: {"str": 42, 3: pow}, -3:42}; json(m)`
	evaluated = testEval(t, input)
	expected = `{"-3":42,"1.5":"a","2":{"3":{"gofunc":"pow(float, float)"},"str":42}}`
	actual, ok = evaluated.(object.String)
	if !ok {
		t.Errorf("object is not string. got=%T (%+v)", evaluated, evaluated)
	}
	if actual.Value != expected {
		t.Errorf("object has wrong value.got:\n%s\n---want--\n%s", actual.Value, expected)
	}
}

func TestNotCachingErrors(t *testing.T) {
	s := eval.NewState()
	_, err := eval.EvalString(s, `func x(n) {aa+n};x(3)`, false)
	if err == nil {
		t.Fatalf("should have errored out, got nil")
	}
	_, err = eval.EvalString(s, `aa=1;x(4)`, false)
	if err != nil {
		t.Errorf("should have not errored out after defining aa, got %v", err)
	}
	_, err = eval.EvalString(s, `x(3)`, false)
	if err != nil {
		t.Errorf("should have not cached the error, got %v", err)
	}
}

func TestNaNMapKey(t *testing.T) {
	// Also tests sorting order with ints mixed
	s := eval.NewState()
	_, err := eval.EvalString(s, `nan=0./0.`, false)
	if err != nil {
		t.Errorf("should have not errored out just defining NaN, got %v", err)
	}
	res, errs, _ := repl.EvalString(`nan=0./0; minf=-1/0.; pinf=1/0.
	m={-42.3: "about -42",42:"int 42", minf:"minf", pinf:"pinf", 42.1:"42.1", nan: "this is NaN"}
	println(m)`)
	if len(errs) != 0 {
		t.Errorf("should have no error trying to put a nan in map, got %v", errs)
	}
	expected := `{NaN:"this is NaN",-Inf:"minf",-42.3:"about -42",42:"int 42",42.1:"42.1",+Inf:"pinf"}` + "\n"
	if res != expected {
		t.Errorf("wrong result, got %s expected %s", res, expected)
	}
}

func TestBlankSlateEval(t *testing.T) {
	// log.SetLogLevel(log.Debug) // was used to confirm no extensions eval etc...
	inp := `func f(a){a+1}; f(1)`
	obj, err := eval.EvalString(nil, inp, true) // indirectly tests unjason which also uses "blank state"
	if err != nil {
		t.Errorf("should have not errored using blank slate eval: %v", err)
	}
	if obj.Type() != object.INTEGER {
		t.Fatalf("should have returned a string value, got %#v", obj)
	}
	if obj.Inspect() != "2" {
		t.Errorf("wrong value, got %q", obj.Inspect())
	}
	s := eval.NewState()
	s.MaxDepth = 10
	inp = `func f(a){if a==0 {return 0} f(a-1)}; f(20)`
	defer func() {
		if r := recover(); r != nil {
			t.Logf("recovered from panic as expected: %v", r)
			if r.(string) != "max depth 10 reached" {
				t.Fatalf("wrong panic message: %v", r)
			}
		}
	}()
	_, _ = eval.EvalString(s, inp, true)
	t.Fatalf("should have panicked and not reach")
}

func TestMapAccidentalMutation(t *testing.T) {
	inp := `m={1:1, nil:"foo"}; m+{nil:"bar"}; m`
	s := eval.NewState()
	res, err := eval.EvalString(s, inp, false)
	if err != nil {
		t.Fatalf("should not have errored: %v", err)
	}
	resStr := res.Inspect()
	expected := `{1:1,nil:"foo"}`
	if resStr != expected {
		t.Errorf("wrong result, got %q expected %q", resStr, expected)
	}
}

func TestSmallMapSorting(t *testing.T) {
	inp := `m={2:"b"};n={1:"a"};m+n`
	expected := `{1:"a",2:"b"}`
	s := eval.NewState()
	res, err := eval.EvalString(s, inp, false)
	if err != nil {
		t.Fatalf("should not have errored: %v", err)
	}
	resStr := res.Inspect()
	if resStr != expected {
		t.Errorf("wrong result, got %q expected %q", resStr, expected)
	}
}

func TestCrashKeys(t *testing.T) {
	inp := `keys(info.all_ids[0])`
	s := eval.NewState()
	_, err := eval.EvalString(s, inp, false)
	if err != nil {
		t.Errorf("should not have errored: %v", err)
	}
}

func TestParenInIf(t *testing.T) {
	inp := `if (1+2)==3 {42}`
	s := eval.NewState()
	res, err := eval.EvalString(s, inp, false)
	if err != nil {
		t.Fatalf("should not have errored: %v", err)
	}
	if res.Type() != object.INTEGER {
		t.Fatalf("should have returned an integer, got %#v", res)
	}
	if res.Inspect() != "42" {
		t.Errorf("wrong result, got %q", res.Inspect())
	}
}
