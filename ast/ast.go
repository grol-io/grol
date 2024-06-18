package ast

import (
	"github.com/ldemailly/gorpl/token"
)

type Node interface {
	TokenLiteral() string
}

// Common to all nodes that have a token and avoids repeating the same TokenLiteral() methods
type Base struct {
	token.Token
}

func (b *Base) TokenLiteral() string {
	return b.Literal
}

// BaseStatement and BaseExpression are used to avoid repeating the same marker for all statements and expressions

type BaseStatement struct {
	Base
}

func (b *BaseStatement) statementNode() {}

type BaseExpression struct {
	Base
}

func (b *BaseExpression) expressionNode() {}

type ReturnStatement struct {
	BaseStatement
	ReturnValue Expression
}

type Statement interface { // Do we need the interface or would the BaseStatement be enough?
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) == 0 {
		return ""
	}
	return p.Statements[0].TokenLiteral() // uh? why just the first one?
}

type LetStatement struct {
	BaseStatement
	Name  *Identifier
	Value Expression
}

type Identifier struct {
	Base
	Value string
}
