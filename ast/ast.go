package ast

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/ldemailly/gorepl/token"
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

func (rs *ReturnStatement) String() string {
	out := strings.Builder{}

	out.WriteString(rs.TokenLiteral())
	out.WriteString(" ")

	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}

	// out.WriteString(";")

	return out.String()
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

func (ls *LetStatement) String() string {
	out := strings.Builder{}

	out.WriteString(ls.TokenLiteral() + " ")
	out.WriteString(ls.Name.String())
	out.WriteString(" = ")

	if ls.Value != nil {
		out.WriteString(ls.Value.String())
	}

	// out.WriteString(";")

	return out.String()
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

type StringLiteral struct {
	Base
	Val string
}

func (s *StringLiteral) Value() Expression {
	return s
}

func (s *StringLiteral) String() string {
	return strconv.Quote(s.Literal)
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

type IfExpression struct {
	Base
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie *IfExpression) String() string {
	var out bytes.Buffer

	out.WriteString("if")
	out.WriteString(ie.Condition.String())
	out.WriteString(" ")
	out.WriteString(ie.Consequence.String())

	if ie.Alternative != nil {
		out.WriteString(" else ")
		out.WriteString(ie.Alternative.String())
	}

	return out.String()
}

func (ie *IfExpression) Value() Expression {
	return ie
}

type BlockStatement struct {
	Base // holds {
	Program
}

func (bs *BlockStatement) String() string {
	return "{\n" + bs.Program.String() + "\n}"
}

type Len struct {
	Base      // The 'len' token
	Parameter Expression
}

func (l *Len) Value() Expression {
	return l
}

func (l *Len) String() string {
	out := strings.Builder{}
	out.WriteString("len(")
	out.WriteString(l.Parameter.String())
	out.WriteString(")")
	return out.String()
}

type FunctionLiteral struct {
	Base       // The 'fn' token
	Parameters []*Identifier
	Body       *BlockStatement
}

func (fl *FunctionLiteral) String() string {
	out := strings.Builder{}
	params := []string{}
	for _, p := range fl.Parameters {
		params = append(params, p.String())
	}
	out.WriteString(fl.TokenLiteral())
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	out.WriteString(fl.Body.String())
	return out.String()
}

func (fl *FunctionLiteral) Value() Expression {
	return fl
}

type CallExpression struct {
	Base                 // The '(' token
	Function  Expression // Identifier or FunctionLiteral
	Arguments []Expression
}

func (ce *CallExpression) Value() Expression {
	return ce
}

func (ce *CallExpression) String() string {
	var out bytes.Buffer

	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}

	out.WriteString(ce.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")

	return out.String()
}
