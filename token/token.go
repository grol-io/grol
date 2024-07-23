// There are 2 types of Token, constant ones (with no "value") and the ones with attached
// value that is variable (e.g. IDENT, INT, FLOAT, STRING, COMMENT).
// We'd use the upcoming unique https://tip.golang.org/doc/go1.23#new-unique-package
// but we want this to run on 1.22 and earlier.
package token

import (
	"fmt"
	"strings"
)

type Type uint8

type Token struct {
	Type    Type
	literal string
}

// Single threaded (famous last words), no need for sync.Map.
var interning map[Token]*Token

// Lookup a unique pointer to a token of same values, if it exists,
// otherwise store the passed in one for future lookups.
func InternToken(t *Token) *Token {
	ptr, ok := interning[*t]
	if ok {
		return ptr
	}
	interning[*t] = t
	return t
}

func Intern(t Type, literal string) *Token {
	return InternToken(&Token{Type: t, literal: literal})
}

func ResetInterning() {
	interning = make(map[Token]*Token)
}

const (
	ILLEGAL Type = iota
	EOL

	startValueTokens

	// Identifiers + literals. with attached value.
	IDENT // add, foobar, x, y, ...
	INT   // 1343456
	FLOAT // 1. 1e3
	LINECOMMENT

	endValueTokens

	startSingleCharTokens

	// Single character operators.
	ASSIGN
	PLUS
	MINUS
	BANG
	ASTERISK
	SLASH
	PERCENT
	LT
	GT
	// Delimiters.
	COMMA
	SEMICOLON

	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACKET
	RBRACKET
	COLON

	endSingleCharTokens

	// 2 char/string constant token range.
	startMultiCharTokens

	// LT, GT or equal variants.
	LTEQ
	GTEQ
	EQ
	NOTEQ

	endMultiCharTokens

	startIdentityTokens // Tokens whose literal is the lowercase of the Type.String()

	// Keywords.
	FUNC
	TRUE
	FALSE
	IF
	ELSE
	RETURN
	STRING
	MACRO
	// Built-in functions.
	LEN
	FIRST
	REST
	PRINT
	LOG
	ERROR

	endIdentityTokens

	EOF
)

var (
	EOLT = &Token{Type: EOL}
	EOFT = &Token{Type: EOF}
)

var (
	keywords map[string]*Token
	cTokens  map[byte]*Token
	tToChar  map[Type]byte
	sTokens  map[string]*Token
)

func init() {
	Init()
}

func assoc(t Type, c byte) {
	tToChar[t] = c
	cTokens[c] = &Token{Type: t, literal: string(c)}
}

func assocS(t Type, s string) *Token {
	tok := &Token{Type: t, literal: s}
	old := InternToken(tok)
	if old != tok {
		panic("duplicate token for " + s)
	}
	sTokens[s] = tok
	return tok
}

func Init() {
	ResetInterning()
	keywords = make(map[string]*Token)
	cTokens = make(map[byte]*Token)
	tToChar = make(map[Type]byte)
	sTokens = make(map[string]*Token)
	for i := startIdentityTokens + 1; i < endIdentityTokens; i++ {
		t := assocS(i, strings.ToLower(i.String()))
		keywords[t.literal] = t
	}
	// Single character tokens:
	assoc(ASSIGN, '=')
	assoc(PLUS, '+')
	assoc(MINUS, '-')
	assoc(BANG, '!')
	assoc(ASTERISK, '*')
	assoc(SLASH, '/')
	assoc(PERCENT, '%')
	assoc(LT, '<')
	assoc(GT, '>')
	assoc(COMMA, ',')
	assoc(SEMICOLON, ';')
	assoc(LPAREN, '(')
	assoc(RPAREN, ')')
	assoc(LBRACE, '{')
	assoc(RBRACE, '}')
	assoc(LBRACKET, '[')
	assoc(RBRACKET, ']')
	assoc(COLON, ':')
	// Verify we have all of them.
	for i := startSingleCharTokens + 1; i < endSingleCharTokens; i++ {
		b, ok := tToChar[i]
		if !ok {
			panic("missing single character token char lookup for " + i.String())
		}
		v, ok := cTokens[b]
		if !ok {
			panic("missing single character token for " + i.String())
		}
		if v.Type != i {
			panic("mismatched single character token for " + i.String() + ":" + v.Type.String())
		}
		if v.literal != string(b) {
			panic(fmt.Sprintf("unexpected literal for single character token for %q: %q vs %q",
				i.String(), v.literal, string(b)))
		}
	}
	// Multi character non identity tokens.
	assocS(LTEQ, "<=")
	assocS(GTEQ, ">=")
	assocS(EQ, "==")
	assocS(NOTEQ, "!=")
	// Special alias for := to be same as ASSIGN.
	sTokens[":="] = cTokens['=']
}

//go:generate stringer -type=Type
var _ = EOF.String() // force compile error if go generate is missing.

func LookupIdent(ident string) *Token {
	// constant/identity ones:
	if t, ok := keywords[ident]; ok {
		return t
	}
	return InternToken(&Token{Type: IDENT, literal: ident})
}

func (t Token) Literal() string {
	return t.literal
}

func ConstantTokenChar(literal byte) *Token {
	return cTokens[literal]
}

func ConstantTokenStr(literal string) *Token {
	return sTokens[literal]
}
