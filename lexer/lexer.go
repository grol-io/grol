package lexer

import "github.com/ldemailly/gorpl/token"

type Lexer struct {
	input string
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	return l
}

func (l *Lexer) NextToken() token.Token {
	return token.Token{Type: token.ILLEGAL, Literal: "not implemented yet"}
}
