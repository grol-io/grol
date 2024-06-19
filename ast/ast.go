package ast

import (
	"github.com/ldemailly/gorpl/token"
)

type Node interface {
	TokenLiteral() string
}

type Expression interface {
	Node
	Value() Expression
}

// Common to all nodes that have a token and avoids repeating the same TokenLiteral() methods.
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
	Val string
}

func (i *Identifier) Value() Expression {
	return i
}

// TODO: probably refactor.

type ExpressionStatement struct {
	Base
	Val Expression
}

func (e *ExpressionStatement) Value() Expression {
	return e.Val
}

type IntegerLiteral struct {
	Base
	Val int64
}

func (i *IntegerLiteral) Value() Expression {
	return i
}

type PrefixExpression struct {
	Base
	Operator string
	Right    Expression
}

func (p *PrefixExpression) Value() Expression {
	return p.Right
}

type InfixExpression struct {
	Base
	Left     Expression
	Operator string
	Right    Expression
}

func (i *InfixExpression) Value() Expression {
	return i
}
