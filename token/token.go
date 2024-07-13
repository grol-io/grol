package token

import "fortio.org/log"

type Type uint8

type Token struct {
	Type    Type
	Literal string
}

const (
	ILLEGAL Type = iota
	EOF

	// Identifiers + literals.
	IDENT // add, foobar, x, y, ...
	INT   // 1343456

	// Operators.
	ASSIGN
	PLUS
	MINUS
	BANG
	ASTERISK
	SLASH
	PERCENT

	LT
	GT
	// or equal variants.
	LTEQ
	GTEQ

	EQ
	NOTEQ

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

	LINECOMMENT
	STARTCOMMENT
	ENDCOMMENT

	// Keywords.
	FUNCTION
	LET
	TRUE
	FALSE
	IF
	ELSE
	RETURN
	STRING
	// Built-in functions.
	LEN
	FIRST
	REST
	PRINT
	LOG
)

//go:generate stringer -type=Type
var _ = EOF.String() // force compile error if go generate is missing.

var keywords = map[string]Type{
	"func":   FUNCTION,
	"let":    LET,
	"true":   TRUE,
	"false":  FALSE,
	"if":     IF,
	"else":   ELSE,
	"return": RETURN,
	// built-in functions.
	"len":   LEN,
	"first": FIRST,
	"rest":  REST,
	"print": PRINT,
	"log":   LOG,
}

func LookupIdent(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		// Ensures compile error if go generate is missing
		log.Debugf("LookupIdent(%s) found %s", ident, tok.String())
		return tok
	}
	return IDENT
}
