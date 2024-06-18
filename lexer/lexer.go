package lexer

import "github.com/ldemailly/gorpl/token"

type Lexer struct {
	input string
	pos   int
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	return l
}

func (l *Lexer) NextToken() token.Token {
	l.skipWhitespace()

	ch := l.readChar()
	switch ch {
	case '=':
		if l.peekChar() == '=' {
			nextChar := l.readChar()
			literal := string(ch) + string(nextChar)
			return token.Token{Type: token.EQ, Literal: literal}
		}
		return newToken(token.ASSIGN, ch)
	case '+':
		return newToken(token.PLUS, ch)
	case '-':
		return newToken(token.MINUS, ch)
	case '!':
		if l.peekChar() == '=' {
			nextChar := l.readChar()
			literal := string(ch) + string(nextChar)
			return token.Token{Type: token.NOTEQ, Literal: literal}
		} else {
			return newToken(token.BANG, ch)
		}
	case '/':
		return newToken(token.SLASH, ch)
	case '*':
		return newToken(token.ASTERISK, ch)
	case '<':
		return newToken(token.LT, ch)
	case '>':
		return newToken(token.GT, ch)
	case ';':
		return newToken(token.SEMICOLON, ch)
	case ',':
		return newToken(token.COMMA, ch)
	case '{':
		return newToken(token.LBRACE, ch)
	case '}':
		return newToken(token.RBRACE, ch)
	case '(':
		return newToken(token.LPAREN, ch)
	case ')':
		return newToken(token.RPAREN, ch)
	case 0:
		return token.Token{Type: token.EOF, Literal: ""}
	default:
		tok := token.Token{}
		switch {
		case isLetter(ch):
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		case isDigit(ch):
			tok.Type = token.INT
			tok.Literal = l.readNumber()
			return tok
		default:
			return newToken(token.ILLEGAL, ch)
		}
	}
}

func isWhiteSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func (l *Lexer) skipWhitespace() {
	// while whitespace, read next char
	for isWhiteSpace(l.peekChar()) {
		l.pos++
	}
}

func (l *Lexer) readChar() byte {
	ch := l.peekChar()
	l.pos++
	return ch
}

func (l *Lexer) peekChar() byte {
	if l.pos < 0 {
		panic("Lexer position is negative")
	}
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) readIdentifier() string {
	pos := l.pos - 1
	for isLetter(l.peekChar()) {
		l.pos++
	}
	return l.input[pos:l.pos]
}

func (l *Lexer) readNumber() string {
	pos := l.pos - 1
	for isDigit(l.peekChar()) {
		l.pos++
	}
	return l.input[pos:l.pos]
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func newToken(typ token.Type, ch byte) token.Token {
	return token.Token{Type: typ, Literal: string(ch)}
}
