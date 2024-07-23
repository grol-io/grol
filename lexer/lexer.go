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

func (l *Lexer) NextToken() *token.Token { //nolint:funlen,gocyclo // many cases to lex.
	l.skipWhitespace()

	ch := l.readChar()
	switch ch {
	case '=', '!', ':':
		if l.peekChar() == '=' {
			nextChar := l.readChar()
			literal := string(ch) + string(nextChar)
			// := is aliased directly to ASSIGN (with = as literal), a bit hacky but
			// so we normalize := like it didn't exist.
			return token.ConstantTokenStr(literal)
		}
		return token.ConstantTokenChar(ch)

	case '%', '*', '+', ';', ',', '{', '}', '(', ')', '[', ']', '-':
		// TODO maybe reorder so it's a continuous range for pure single character tokens
		return token.ConstantTokenChar(ch)
	case '/':
		if l.peekChar() == '/' {
			return token.Intern(token.LINECOMMENT, l.readLineComment())
		}
		return token.ConstantTokenChar(ch)
	case '<', '>':
		if l.peekChar() == '=' {
			nextChar := l.readChar()
			literal := string(ch) + string(nextChar)
			return token.ConstantTokenStr(literal)
		}
		return token.ConstantTokenChar(ch)
	case '"':
		return token.Intern(token.STRING, l.readString())
	case 0:
		if l.lineMode {
			return token.EOL_TOKEN
		} else {
			return token.EOF_TOKEN
		}
	default:
		switch {
		case isLetter(ch):
			return token.LookupIdent(l.readIdentifier())
		case isDigit(ch) || ch == '.':
			// number can start with . eg .5
			return token.Intern(l.readNumber(ch))
		default:
			return token.Intern(token.ILLEGAL, string(ch))
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

func (l *Lexer) readNumber(ch byte) (token.Type, string) {
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
	return t, l.input[pos:l.pos]
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
