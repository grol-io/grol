package lexer

import (
	"strings"

	"grol.io/grol/token"
)

type Lexer struct {
	input    string
	pos      int
	lineMode bool
}

// Mode with input expected the be complete (multiline/file).
func New(input string) *Lexer {
	return &Lexer{input: input}
}

func NewLineMode(input string) *Lexer {
	return &Lexer{input: input, lineMode: true}
}

func (l *Lexer) NextToken() token.Token { //nolint:funlen,gocyclo // many cases to lex.
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
		if l.peekChar() == '/' {
			tok := token.Token{Type: token.LINECOMMENT}
			tok.Literal = l.readLineComment()
			return tok
		}
		return newToken(token.SLASH, ch)
	case '%':
		return newToken(token.PERCENT, ch)
	case '*':
		return newToken(token.ASTERISK, ch)
	case '<':
		if l.peekChar() == '=' {
			nextChar := l.readChar()
			literal := string(ch) + string(nextChar)
			return token.Token{Type: token.LTEQ, Literal: literal}
		}
		return newToken(token.LT, ch)
	case '>':
		if l.peekChar() == '=' {
			nextChar := l.readChar()
			literal := string(ch) + string(nextChar)
			return token.Token{Type: token.GTEQ, Literal: literal}
		}
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
	case '[':
		return newToken(token.LBRACKET, ch)
	case ']':
		return newToken(token.RBRACKET, ch)
	case ':':
		if l.peekChar() == '=' { // semi hacky treat := as = (without changing literal etc so tests work with either)
			nextChar := l.readChar()
			return newToken(token.ASSIGN, nextChar)
		}
		return newToken(token.COLON, ch)
	case '"':
		return token.Token{Type: token.STRING, Literal: l.readString()}
	case 0:
		if l.lineMode {
			return token.Token{Type: token.EOL}
		} else {
			return token.Token{Type: token.EOF}
		}
	default:
		tok := token.Token{}
		switch {
		case isLetter(ch):
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			return tok
		case isDigit(ch) || ch == '.':
			// number can start with . eg .5
			tok.Literal, tok.Type = l.readNumber(ch)
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

func (l *Lexer) readString() string {
	buf := strings.Builder{}
scanLoop:
	for {
		ch := l.readChar()
		switch ch {
		case '\\':
			ch = l.readChar()
			switch ch {
			case 'n':
				ch = '\n'
			case 't':
				ch = '\t'
			}
		case '"', 0:
			break scanLoop
		}
		buf.WriteByte(ch)
	}
	return buf.String()
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
	for isAlphaNum(l.peekChar()) {
		l.pos++
	}
	return l.input[pos:l.pos]
}

func notEOL(ch byte) bool {
	return ch != '\n' && ch != 0
}

func (l *Lexer) readLineComment() string {
	pos := l.pos - 1
	for notEOL(l.peekChar()) {
		l.pos++
	}
	return l.input[pos:l.pos]
}

func (l *Lexer) readNumber(ch byte) (string, token.Type) {
	t := token.INT
	if ch == '.' {
		t = token.FLOAT
	}
	pos := l.pos - 1
	for isDigit(l.peekChar()) {
		l.pos++
	}
	// if we haven't seen a dot at the start already.
	if t == token.INT && l.peekChar() == '.' {
		t = token.FLOAT
		l.pos++
		for isDigit(l.peekChar()) {
			l.pos++
		}
	}
	return l.input[pos:l.pos], t
}

func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isAlphaNum(ch byte) bool {
	return isLetter(ch) || isDigit(ch)
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func newToken(typ token.Type, ch byte) token.Token {
	return token.Token{Type: typ, Literal: string(ch)}
}
