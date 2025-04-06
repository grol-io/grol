// There are 2 types of Token, constant ones (with no "value") and the ones with attached
// value that is variable (e.g. IDENT, INT, FLOAT, STRING, *COMMENT).
// We might have used the upcoming unique https://tip.golang.org/doc/go1.23#new-unique-package
// but we want this to run on 1.22 and earlier and rolled our own, not multi threaded.
package token

import (
	"fmt"
	"strconv"
	"strings"

	"fortio.org/sets"
)

// 'noCopy' Stolen from
// https://cs.opensource.google/go/go/+/master:src/sync/atomic/type.go;l=224-235?q=noCopy&ss=go%2Fgo
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type Type uint8

type Token struct {
	// Allows go vet to flag accidental copies of this type,
	// though with an error about lock value which can be confusing
	_         noCopy
	tokenType Type
	literal   string
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
	return InternToken(&Token{tokenType: t, literal: literal})
}

func ResetInterning() {
	interning = make(map[Token]*Token)
}

const (
	ILLEGAL Type = iota
	EOL

	startValueTokens

	// Identifiers + literals. with attached value.
	IDENT  // add, foobar, x, y, ...
	INT    // 1343456
	FLOAT  // .5, 3.14159,...
	STRING // "foo bar" or `foo bar`
	LINECOMMENT
	BLOCKCOMMENT
	REGISTER // not used for parsing, only to tag object.Register as ast node of unique type.

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
	BITAND
	BITOR
	BITXOR
	BITNOT

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
	DOT

	endSingleCharTokens

	// 2 char/string constant token range.
	startMultiCharTokens

	// LT, GT or equal variants.
	LTEQ
	GTEQ
	EQ
	NOTEQ
	INCR
	DECR
	DOTDOT
	OR
	AND
	LEFTSHIFT
	RIGHTSHIFT
	LAMBDA // => lambda short cut: `a,b => a+b` alias for `func(a,b) {a+b}`
	DEFINE // := (force create new variable instead of possible ref to upper scope)

	endMultiCharTokens

	startIdentityTokens // Tokens whose literal is the lowercase of the Type.String()

	// Keywords.
	FUNC
	TRUE
	FALSE
	IF
	ELSE
	RETURN
	FOR
	BREAK
	CONTINUE
	// Macro magic.
	MACRO
	QUOTE
	UNQUOTE
	// Built-in functions.
	LEN
	FIRST
	REST
	PRINT
	PRINTLN
	LOG
	ERROR
	CATCH
	DEL

	endIdentityTokens

	EOF
)

var (
	EOLT   = &Token{tokenType: EOL}
	EOFT   = &Token{tokenType: EOF}
	TRUET  *Token
	FALSET *Token
)

var (
	keywords map[string]*Token
	cTokens  map[byte]*Token
	c2Tokens map[[2]byte]*Token
	tToChar  map[Type]byte
	tToT     map[Type]*Token // for all token that are constant.
)

func init() {
	Init()
}

func assoc(t Type, c byte) {
	tToChar[t] = c
	tok := &Token{tokenType: t, literal: string(c)}
	cTokens[c] = tok
	tToT[t] = tok
	info.Tokens.Add(tok.literal)
}

func assocS(t Type, s string) *Token {
	tok := &Token{tokenType: t, literal: s}
	old := InternToken(tok)
	if old != tok {
		panic("duplicate token for " + s)
	}
	tToT[t] = tok
	return tok
}

func assocKeywords(t Type, s string) *Token {
	info.Keywords.Add(s)
	return assocS(t, s)
}

// Functions().
func assocBuiltins(t Type, s string) *Token {
	info.Builtins.Add(s)
	return assocS(t, s)
}

func assocC2(t Type, str string) {
	if len(str) != 2 {
		panic("assocC2: expected 2 char string")
	}
	tok := &Token{tokenType: t, literal: str}
	old := InternToken(tok)
	if old != tok {
		panic("duplicate token for " + str)
	}
	tToT[t] = tok
	c2Tokens[[2]byte{str[0], str[1]}] = tok
	info.Tokens.Add(str)
}

func Init() {
	ResetInterning()
	info.Keywords = sets.New[string]()
	info.Builtins = sets.New[string]()
	info.Tokens = sets.New[string]()
	keywords = make(map[string]*Token)
	cTokens = make(map[byte]*Token)
	c2Tokens = make(map[[2]byte]*Token)
	tToChar = make(map[Type]byte)
	tToT = make(map[Type]*Token)
	for i := startIdentityTokens + 1; i < endIdentityTokens; i++ {
		var t *Token
		if i >= MACRO {
			t = assocBuiltins(i, strings.ToLower(i.String()))
		} else {
			t = assocKeywords(i, strings.ToLower(i.String()))
		}
		keywords[t.literal] = t
	}
	TRUET = tToT[TRUE]
	FALSET = tToT[FALSE]
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
	assoc(DOT, '.')
	assoc(BITAND, '&')
	assoc(BITOR, '|')
	assoc(BITXOR, '^')
	assoc(BITNOT, '~')
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
		if v.tokenType != i {
			panic("mismatched single character token for " + i.String() + ":" + v.tokenType.String())
		}
		if v.literal != string(b) {
			panic(fmt.Sprintf("unexpected literal for single character token for %q: %q vs %q",
				i.String(), v.literal, string(b)))
		}
		tok, ok := tToT[i]
		if !ok {
			panic("missing single character token for " + i.String())
		}
		if tok.tokenType != i {
			panic("mismatched single character token for " + i.String() + ":" + tok.tokenType.String())
		}
	}
	// Multi character non identity tokens.
	assocC2(LTEQ, "<=")
	assocC2(GTEQ, ">=")
	assocC2(EQ, "==")
	assocC2(NOTEQ, "!=")
	assocC2(INCR, "++")
	assocC2(DECR, "--")
	assocC2(DOTDOT, "..")
	assocC2(OR, "||")
	assocC2(AND, "&&")
	assocC2(LEFTSHIFT, "<<")
	assocC2(RIGHTSHIFT, ">>")
	assocC2(LAMBDA, "=>")
	assocC2(DEFINE, ":=")
}

//go:generate stringer -type=Type
var _ = EOF.String() // force compile error if go generate is missing.

func LookupIdent(ident string) *Token {
	// constant/identity ones:
	if t, ok := keywords[ident]; ok {
		return t
	}
	return InternToken(&Token{tokenType: IDENT, literal: ident})
}

// ByType is the cheapest lookup for all the tokens whose type
// only have one possible instance/value
// (ie all the tokens except for the first 4 value tokens).
// TODO: codegen all the token constants to avoid needing this function.
// (even though that's better than string comparisons).
func ByType(t Type) *Token {
	return tToT[t]
}

func (t *Token) Literal() string {
	return t.literal
}

func (t *Token) Type() Type {
	return t.tokenType
}

func ConstantTokenChar(literal byte) *Token {
	return cTokens[literal]
}

func ConstantTokenChar2(c1, c2 byte) *Token {
	return c2Tokens[[2]byte{c1, c2}]
}

func (t *Token) DebugString() string {
	return t.Type().String() + ":" + strconv.Quote(t.Literal())
}
