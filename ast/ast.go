package ast

import (
	"github.com/ldemailly/gorpl/token"
)

type Node interface {
	TokenLiteral() string
}

type Expression Node

// Common to all nodes that have a token and avoids repeating the same TokenLiteral() methods
type Base struct {
	token.Token
}

func (b *Base) TokenLiteral() string {
	return b.Literal
}

type ReturnStatement struct {
	Base
	ReturnValue Expression
}

type Program struct {
	Statements []Node
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) == 0 {
		return "<empty>"
	}
	return p.Statements[0].TokenLiteral() // uh? why just the first one?
}

type LetStatement struct {
	Base
	Name  *Identifier
	Value Expression
}

type Identifier struct {
	Base
	Value string
}
