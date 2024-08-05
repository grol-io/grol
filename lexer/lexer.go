package lexer

import (
	"strings"

	"grol.io/grol/token"
)

type Lexer struct {
	input         []byte
	pos           int
	lineMode      bool
	hadWhitespace bool
	hadNewline    bool // newline was seen before current token
}

// Mode with input expected the be complete (multiline/file).
func New(input string) *Lexer {
	return NewBytes([]byte(input))
}

func NewLineMode(input string) *Lexer {
	return &Lexer{input: []byte(input), lineMode: true}
}

// Bytes based full input mode.
func NewBytes(input []byte) *Lexer {
	return &Lexer{input: input}
}

func (l *Lexer) NextToken() *token.Token {
	l.skipWhitespace()
	ch := l.readChar()
	nextChar := l.peekChar()
	switch ch { // Maybe benchmark and do our own lookup table?
	case '=', '!', ':':
		if nextChar == '=' {
			l.pos++
			// := is aliased directly to ASSIGN (with = as literal), a bit hacky but
			// so we normalize := like it didn't exist.
			return token.ConstantTokenChar2(ch, nextChar)
		}
		return token.ConstantTokenChar(ch)
	case '+', '-':
		if nextChar == ch {
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar) // increment/decrement
		}
		return token.ConstantTokenChar(ch)
	case '%', '*', ';', ',', '{', '}', '(', ')', '[', ']':
		// TODO maybe reorder so it's a continuous range for pure single character tokens
		return token.ConstantTokenChar(ch)
	case '/':
		if nextChar == '/' {
			return token.Intern(token.LINECOMMENT, l.readLineComment())
		}
		if nextChar == '*' {
			return token.Intern(token.BLOCKCOMMENT, l.readBlockComment())
		}
		return token.ConstantTokenChar(ch)
	case '<', '>':
		if nextChar == '=' {
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar)
		}
		return token.ConstantTokenChar(ch)
	case '"':
		return token.Intern(token.STRING, l.readString())
	case 0:
		if l.lineMode {
			return token.EOLT
		} else {
			return token.EOFT
		}
	case '.':
		if nextChar == '.' { // DOTDOT
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar)
		}
		// number can start with . eg .5
		return token.Intern(l.readNumber(ch))
	default:
		switch {
		case isLetter(ch):
			return token.LookupIdent(l.readIdentifier())
		case isDigit(ch):
			return token.Intern(l.readNumber(ch))
		default:
			return token.Intern(token.ILLEGAL, string(ch))
		}
	}
}

func isWhiteSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func (l *Lexer) HadWhitespace() bool {
	return l.hadWhitespace
}

func (l *Lexer) HadNewline() bool {
	return l.hadNewline
}

func (l *Lexer) skipWhitespace() {
	l.hadWhitespace = false
	l.hadNewline = false
	// while whitespace, read next char
	for {
		ch := l.peekChar()
		if !isWhiteSpace(ch) {
			break
		}
		if ch == '\n' {
			l.hadNewline = true
		}
		l.hadWhitespace = true
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
	return string(l.input[pos:l.pos])
}

func notEOL(ch byte) bool {
	return ch != '\n' && ch != 0
}

func (l *Lexer) readLineComment() string {
	pos := l.pos - 1
	for notEOL(l.peekChar()) {
		l.pos++
	}
	return strings.TrimSpace(string(l.input[pos:l.pos]))
}

func (l *Lexer) endBlockComment(ch byte) bool {
	return ch == '*' && l.peekChar() == '/'
}

func (l *Lexer) readBlockComment() string {
	pos1 := l.pos - 1
	l.pos++
	ch := l.readChar()
	for ch != 0 && !l.endBlockComment(ch) {
		ch = l.readChar()
	}
	if ch == 0 {
		l.pos--
	} else {
		l.pos++
	}
	return string(l.input[pos1:l.pos])
}

func (l *Lexer) readNumber(ch byte) (token.Type, string) {
	t := token.INT
	pos := l.pos - 1
	dotSeen := false
	hasDigits := true
	// Integer part or leading dot for fractional part
	if ch == '.' {
		t = token.FLOAT
		hasDigits = false
		dotSeen = true
	}
	for isDigitOrUnderscore(l.peekChar()) {
		hasDigits = true
		l.pos++
	}
	// Fractional part
	if l.peekChar() == '.' {
		if dotSeen {
			// Stop if we see another dot
			return t, string(l.input[pos : l.pos-1])
		}
		t = token.FLOAT
		l.pos++
		for isDigitOrUnderscore(l.peekChar()) {
			hasDigits = true
			l.pos++
		}
	}
	// Exponent part
	peek := l.peekChar()
	if peek != 'e' && peek != 'E' {
		return t, string(l.input[pos:l.pos])
	}
	errPos := l.pos
	if !hasDigits {
		// Invalid number, stop here if no digits seen before exponent
		return t, string(l.input[pos:errPos])
	}
	l.pos++
	peek = l.peekChar()
	if peek == '+' || peek == '-' {
		l.pos++
	}
	if !isDigit(l.peekChar()) {
		// Invalid exponent, stop here
		return t, string(l.input[pos:errPos])
	}
	t = token.FLOAT
	// Read exponent
	for isDigitOrUnderscore(l.peekChar()) {
		l.pos++
	}
	return t, string(l.input[pos:l.pos])
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

func isDigitOrUnderscore(ch byte) bool {
	return isDigit(ch) || ch == '_'
}
