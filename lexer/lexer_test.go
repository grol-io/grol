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
"foo\nbar\t\\"
[1, 2];
a2=3
{"foo": "bar"}
return // nil return
macro(x, y) { x + y }
a3:=5
4>=3.1
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
		{token.STRING, "foo\nbar\t\\"},
		{token.LBRACKET, "["},
		{token.INT, "1"},
		{token.COMMA, ","},
		{token.INT, "2"},
		{token.RBRACKET, "]"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "a2"},
		{token.ASSIGN, "="},
		{token.INT, "3"},
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
		{token.ASSIGN, "="}, // `:=` changed to `=`.
		{token.INT, "5"},
		{token.INT, "4"},
		{token.GTEQ, ">="},
		{token.FLOAT, "3.1"},
		{token.ILLEGAL, "@"},
		{token.EOF, ""},
	}

	l := New(input)

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
	}
}

func TestNextTokenEOLMode(t *testing.T) {
	input := `if .5 {`
	l := NewLineMode(input)
	tests := []struct {
		expectedType    token.Type
		expectedLiteral string
	}{
		{token.IF, "if"},
		{token.FLOAT, ".5"},
		{token.LBRACE, "{"},
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
	}
}
