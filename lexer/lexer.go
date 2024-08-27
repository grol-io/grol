package lexer

import (
	"bytes"
	"strings"

	"grol.io/grol/token"
)

type Lexer struct {
	input         []byte
	pos           int
	lineMode      bool
	hadWhitespace bool
	hadNewline    bool // newline was seen before current token
	lastNewLine   int  // position just after most recent newline
	lineNumber    int
}

// Mode with input expected the be complete (multiline/file).
func New(input string) *Lexer {
	return NewBytes([]byte(input))
}

// Line by line mode, with possible continuation needed.
func NewLineMode(input string) *Lexer {
	return &Lexer{input: []byte(input), lineMode: true, lineNumber: 1}
}

// Bytes based full input mode.
func NewBytes(input []byte) *Lexer {
	return &Lexer{input: input, lineNumber: 1}
}

func (l *Lexer) EOLEOF() *token.Token {
	if l.lineMode {
		return token.EOLT
	}
	return token.EOFT
}

func (l *Lexer) Pos() int {
	return l.pos
}

func (l *Lexer) LastNewLine() int {
	return l.lastNewLine
}

// For error handling, somewhat expensive.
// Returns the current line, the current position relative in that line
// and the current line number.
func (l *Lexer) CurrentLine() (string, int, int) {
	p := min(l.pos, len(l.input))
	nextNewline := bytes.IndexByte(l.input[p:], '\n')
	if nextNewline == -1 {
		nextNewline = len(l.input) - p
	}
	return string(l.input[l.lastNewLine : p+nextNewline]), p - l.lastNewLine, l.lineNumber
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
		if nextChar == '>' && ch == '=' { // => lambda
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar)
		}
		return token.ConstantTokenChar(ch)
	case '+', '-':
		if nextChar == ch {
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar) // increment/decrement
		}
		return token.ConstantTokenChar(ch)
	case '%', '*', ';', ',', '{', '}', '(', ')', '[', ']', '^', '~':
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
	case '|', '&':
		if nextChar == ch {
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar)
		}
		return token.ConstantTokenChar(ch)

	case '<', '>':
		if nextChar == ch { // << and >>
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar)
		}
		if nextChar == '=' {
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar)
		}
		return token.ConstantTokenChar(ch)
	case '"', '`':
		str, ok := l.readString(ch)
		if !ok {
			return l.EOLEOF()
		}
		return token.Intern(token.STRING, str)
	case 0:
		return l.EOLEOF()
	case '.':
		if nextChar == '.' { // DOTDOT
			l.pos++
			return token.ConstantTokenChar2(ch, nextChar)
		}
		if !isDigit(nextChar) {
			return token.ConstantTokenChar(ch) // DOT token
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
			l.lastNewLine = l.pos + 1
			l.lineNumber++
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

func hexCharToHex(ch byte) byte {
	switch {
	case '0' <= ch && ch <= '9':
		return ch - '0'
	case 'a' <= ch && ch <= 'f':
		return ch - 'a' + 10
	case 'A' <= ch && ch <= 'F':
		return ch - 'A' + 10
	}
	return 0
}

func (l *Lexer) readHex() byte {
	hb := hexCharToHex(l.readChar()) << 4
	lb := hexCharToHex(l.readChar())
	return hb | lb
}

func (l *Lexer) readUnicode16() rune {
	hb := int(l.readHex()) << 8
	lb := int(l.readHex())
	return rune(hb | lb)
}

func (l *Lexer) readUnicode32() rune {
	hb := l.readUnicode16() << 16
	lb := l.readUnicode16()
	return hb | lb
}

func (l *Lexer) readString(sep byte) (string, bool) {
	doubleQuotes := (sep == '"')
	buf := strings.Builder{}
	for {
		ch := l.readChar()
		switch {
		case doubleQuotes && ch == '\\':
			ch = l.readChar()
			switch ch {
			case 'r':
				ch = '\r'
			case 'n':
				ch = '\n'
			case 't':
				ch = '\t'
			case 'u':
				buf.WriteRune(l.readUnicode16())
				continue
			case 'U':
				buf.WriteRune(l.readUnicode32())
				continue
			case 'x':
				ch = l.readHex()
			}
		case ch == sep:
			return buf.String(), true
		case ch == 0:
			return buf.String(), false
		}
		buf.WriteByte(ch)
	}
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
	for IsAlphaNum(l.peekChar()) {
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
	dotSeen := false
	hasDigits := true
	// Integer part or leading dot for fractional part
	if ch == '.' {
		t = token.FLOAT
		hasDigits = false
		dotSeen = true
	}
	pos := l.pos - 1
	if ch == '0' && (l.peekChar() == 'x') {
		l.pos++
		for isHexDigit(l.peekChar()) {
			l.pos++
		}
		return t, string(l.input[pos:l.pos])
	}
	if ch == '0' && (l.peekChar() == 'b') {
		l.pos++
		for isBinaryDigit(l.peekChar()) {
			l.pos++
		}
		return t, string(l.input[pos:l.pos])
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

func isHexDigit(ch byte) bool {
	return isDigitOrUnderscore(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

func isBinaryDigit(ch byte) bool {
	return ch == '0' || ch == '1' || ch == '_'
}

func isLetter(ch byte) bool {
	return ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z') || ch == '_'
}

func IsAlphaNum(ch byte) bool {
	return isLetter(ch) || isDigit(ch)
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isDigitOrUnderscore(ch byte) bool {
	return isDigit(ch) || ch == '_'
}
