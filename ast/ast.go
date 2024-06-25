package ast

import (
	"strings"

	"github.com/ldemailly/gorpl/token"
)

type Node interface {
	TokenLiteral() string
	String() string // normalized string representation of the expression/statement.
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

func (b *Base) String() string {
	return b.Type.String() + " " + b.Literal
}

type ReturnStatement struct {
	Base
	ReturnValue Expression
}

type Program struct {
	Statements []Node
}

func (p *Program) String() string {
	if len(p.Statements) == 0 {
		return "<empty>"
	}
	// string buffer
	buf := strings.Builder{}
	for i, s := range p.Statements {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(s.String())
	}
	return buf.String()
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

func (i *Identifier) String() string {
	return i.Literal
}

// TODO: probably refactor.

type ExpressionStatement struct {
	Base
	Val Expression
}

func (e *ExpressionStatement) Value() Expression {
	return e.Val
}

func (e *ExpressionStatement) String() string {
	return e.Val.String()
}

type IntegerLiteral struct {
	Base
	Val int64
}

func (i *IntegerLiteral) Value() Expression {
	return i
}

func (i *IntegerLiteral) String() string {
	return i.Literal
}

type PrefixExpression struct {
	Base
	Operator string
	Right    Expression
}

func (p *PrefixExpression) Value() Expression {
	return p.Right
}

func (p *PrefixExpression) String() string {
	var out strings.Builder

	out.WriteString("(")
	out.WriteString(p.Operator)
	out.WriteString(p.Right.String())
	out.WriteString(")")

	return out.String()
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

func (i *InfixExpression) String() string {
	var out strings.Builder

	out.WriteString("(")
	out.WriteString(i.Left.String())
	out.WriteString(" ")
	out.WriteString(i.Operator)
	out.WriteString(" ")
	out.WriteString(i.Right.String())
	out.WriteString(")")

	return out.String()
}

type Boolean struct {
	Base
	Val bool
}

func (b *Boolean) Value() Expression {
	return b
}

func (b *Boolean) String() string {
	return b.Literal
}
