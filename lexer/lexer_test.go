package lexer

import (
	"testing"

	"grol.io/grol/token"
)

func TestNextToken(t *testing.T) { //nolint:funlen // this is a test function with many cases back to back.
	input := `five = 5;
ten = 10;

add = func(x, y) {
x + y;
};

result = add(five, ten);
!-/%*5;
5 < 10 > 5;

if (5 < 10) {
	return true;
} else {
	return false;
}

10 == 10;
10 != 9;
"foobar"
"foo bar"
"foo\"bar"
"foo\r\nbar\t\\"
"x\x41y\u263Az\U0001F600"
[1, 2];
a2=.3
{"foo": "bar"}
return // nil return
macro(x, y) { x + y }
a3:=5
4>=3.1
i++
j--
/*/*/
/* This is a
   multiline comment */
..
a.b
@
`
	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.IDENT, "five"},
		{token.ASSIGN, "="},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "ten"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "add"},
		{token.ASSIGN, "="},
		{token.FUNC, "func"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.COMMA, ","},
		{token.IDENT, "y"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.IDENT, "y"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "result"},
		{token.ASSIGN, "="},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "five"},
		{token.COMMA, ","},
		{token.IDENT, "ten"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.BANG, "!"},
		{token.MINUS, "-"},
		{token.SLASH, "/"},
		{token.PERCENT, "%"},
		{token.ASTERISK, "*"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.GT, ">"},
		{token.INT, "5"},
		{token.SEMICOLON, ";"},
		{token.IF, "if"},
		{token.LPAREN, "("},
		{token.INT, "5"},
		{token.LT, "<"},
		{token.INT, "10"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.TRUE, "true"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.ELSE, "else"},
		{token.LBRACE, "{"},
		{token.RETURN, "return"},
		{token.FALSE, "false"},
		{token.SEMICOLON, ";"},
		{token.RBRACE, "}"},
		{token.INT, "10"},
		{token.EQ, "=="},
		{token.INT, "10"},
		{token.SEMICOLON, ";"},
		{token.INT, "10"},
		{token.NOTEQ, "!="},
		{token.INT, "9"},
		{token.SEMICOLON, ";"},
		{token.STRING, "foobar"},
		{token.STRING, "foo bar"},
		{token.STRING, `foo"bar`},
		{token.STRING, "foo\r\nbar\t\\"},
		{token.STRING, "xAy☺z😀"},
		{token.LBRACKET, "["},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.INT, "2"},
		{token.RBRACKET, "]"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "a2"},
		{token.ASSIGN, "="},
		{token.FLOAT, ".3"},
		{token.LBRACE, "{"},
		{token.STRING, "foo"},
		{token.COLON, ":"},
		{token.STRING, "bar"},
		{token.RBRACE, "}"},
		{token.RETURN, "return"},
		{token.LINECOMMENT, "// nil return"},
		{token.MACRO, "macro"},
		{token.LPAREN, "("},
		{token.IDENT, "x"},
		{token.COMMA, ","},
		{token.IDENT, "y"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.IDENT, "x"},
		{token.PLUS, "+"},
		{token.IDENT, "y"},
		{token.RBRACE, "}"},
		{token.IDENT, "a3"},
		{token.DEFINE, ":="},
		{token.INT, "5"},
		{token.INT, "4"},
		{token.GTEQ, ">="},
		{token.FLOAT, "3.1"},
		{token.IDENT, "i"},
		{token.INCR, "++"},
		{token.IDENT, "j"},
		{token.DECR, "--"},
		{token.BLOCKCOMMENT, "/*/*/"},
		{token.BLOCKCOMMENT, "/* This is a\n   multiline comment */"},
		{token.DOTDOT, ".."},
		{token.IDENT, "a"},
		{token.DOT, "."},
		{token.IDENT, "b"},
		{token.ILLEGAL, "@"},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		t.Logf("test %d: %v", i, tt)
		tok := l.NextToken()

		if tok.Type() != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q and %q, got=%v",
				i, tt.expectedType, tt.expectedLiteral, tok)
		}

		if tok.Literal() != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal())
		}
	}
}

func TestNextTokenEOLMode(t *testing.T) {
	input := "if .5 { x (  \n  "
	l := NewLineMode(input)
	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.IF, "if"},
		{token.FLOAT, ".5"},
		{token.LBRACE, "{"},
		{token.IDENT, "x"},
		{token.LPAREN, "("},
		{token.EOL, ""},
	}
	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type() != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q and %q, got=%v",
				i, tt.expectedType, tt.expectedLiteral, tok)
		}

		if tok.Literal() != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%v",
				i, tt.expectedLiteral, tok)
		}
		if i == 0 {
			if l.HadWhitespace() {
				t.Errorf("tests[%d] - didn't expected on first", i)
			}
		} else {
			if !l.HadWhitespace() {
				t.Errorf("tests[%d] - expected whitespace", i)
			}
		}
		if i == len(tests)-1 {
			if !l.HadNewline() {
				t.Errorf("last test (%d) - expected newline", i)
			}
		} else {
			if l.HadNewline() {
				t.Errorf("tests[%d] - didn't expect newline", i)
			}
		}
	}
}

func TestNextTokenCommentEOLMode(t *testing.T) {
	input := `/* incomplete`
	l := NewLineMode(input)
	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.BLOCKCOMMENT, "/* incomplete"},
		{token.EOL, ""},
	}
	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type() != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q and %q, got=%v",
				i, tt.expectedType, tt.expectedLiteral, tok)
		}

		if tok.Literal() != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal())
		}
	}
}

func TestReadFloatNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{".", "."},  // not a valid float number yet we consume and it'll fail to convert later.
		{".a", "."}, // not a valid float number yet we consume and it'll fail to convert later.
		{"123.", "123."},
		{".5", ".5"},
		{"100.56", "100.56"},
		{"1e3", "1e3"},
		{"1.23e-3", "1.23e-3"},
		{"1.23E+3", "1.23E+3"},
		{"1e-3", "1e-3"},
		{"12..3", "12."},
		{"1e+3", "1e+3"},
		{"1.23e", "1.23"},
		{"1.23e+", "1.23"},
		{"1.23e-", "1.23"},
		{"1.23e-abc", "1.23"},
		{"123..", "123."},
		{"1..23", "1."},
		{".e3", "."}, // that's now the DOT and not a (bad) float.
		{"100..", "100."},
		{"1000_000.5", "1000_000.5"},
		{"1000_000.5_6", "1000_000.5_6"},
		{"1.23e1_000", "1.23e1_000"},   // too big for float64, but "lexable".
		{"1.23e+1_000", "1.23e+1_000"}, // too big for float64, but "lexable".
		{"1.23E-1_000", "1.23E-1_000"}, // too big for float64, but "lexable".
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()
		tokT := tok.Type()
		result := tok.Literal()
		if tt.expected == "." {
			if tokT != token.DOT {
				t.Errorf("input: %q, expected a DOT, got: %#v", tt.input, tok.DebugString())
			}
			continue
		}
		isFloat := (tokT == token.FLOAT)
		if !isFloat {
			t.Errorf("input: %q, expected a float number, got: %#v", tt.input, tok.DebugString())
		}
		if result != tt.expected {
			t.Errorf("input: %q, expected: %q, got: %q", tt.input, tt.expected, result)
		}
	}
}

func TestReadIntNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1_2_3", "1_2_3"},
		{"5", "5"},
		{"100abc", "100"},
		{"1000_000", "1000_000"},
		{"0xe_f1Ag", "0xe_f1A"},
		{"0b1010_11112", "0b1010_1111"},
	}

	for _, tt := range tests {
		l := New(tt.input)
		tok := l.NextToken()
		tokT := tok.Type()
		result := tok.Literal()
		isFloat := (tokT == token.INT)
		if !isFloat {
			t.Errorf("input: %q, expected a int number, got: %#v", tt.input, tok.DebugString())
		}
		if result != tt.expected {
			t.Errorf("input: %q, expected: %q, got: %q", tt.input, tt.expected, result)
		}
	}
}
