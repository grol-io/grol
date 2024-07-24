package ast

import (
	"io"
	"strconv"
	"strings"

	"fortio.org/log"
	"grol.io/grol/token"
)

type PrintState struct {
	Out             io.Writer
	IndentLevel     int
	ExpressionLevel int
	IdentationDone  bool // already put N number of tabs, reset on each new line
}

func NewPrintState() *PrintState {
	return &PrintState{Out: &strings.Builder{}}
}

func (ps *PrintState) String() string {
	return ps.Out.(*strings.Builder).String()
}

// Will print indented to current level. with a newline if arguments are passed.
func (ps *PrintState) Println(str ...string) *PrintState {
	ps.Print(str...)
	_, _ = ps.Out.Write([]byte{'\n'})
	ps.IdentationDone = false
	return ps
}

func (ps *PrintState) Print(str ...string) *PrintState {
	if !ps.IdentationDone {
		_, _ = ps.Out.Write([]byte(strings.Repeat("\t", ps.IndentLevel)))
		ps.IdentationDone = true
	}
	for _, s := range str {
		_, _ = ps.Out.Write([]byte(s))
	}
	return ps
}

func (ps *PrintState) WriteString(str string) *PrintState {
	_, _ = ps.Out.Write([]byte(str))
	return ps
}

type Node interface {
	TokenType() token.Token
	PrettyPrint(ps *PrintState) *PrintState
}

// Common to all nodes that have a token and avoids repeating the same TokenLiteral() methods.
type Base struct {
	*token.Token
	Node
}

func (b Base) String() string {
	// TODO/wip: b.Node.PrettyPrint instead.
	return b.PrettyPrint(NewPrintState()).String()
}

func (b Base) PrettyPrint(ps *PrintState) *PrintState {
	log.Warnf("PrettyPrint not implemented for %T", b)
	return ps.Print(b.Literal(), " ", b.Type().String())
}

type ReturnStatement struct {
	Base
	ReturnValue Node
}

func (rs ReturnStatement) PrettyPrint(ps *PrintState) *PrintState {
	ps.Print(rs.Literal())
	if rs.ReturnValue != nil {
		ps.Print(" ")
		rs.ReturnValue.PrettyPrint(ps)
	}
	return ps.Println()
}

type Program struct {
	Base
	Statements []Node
}

func (p Program) String() string {
	return p.PrettyPrint(NewPrintState()).String()
}

func (p Program) PrettyPrint(ps *PrintState) *PrintState {
	if len(p.Statements) == 0 {
		ps.Print("<empty>")
		return ps
	}
	for _, s := range p.Statements {
		s.PrettyPrint(ps)
	}
	return ps
}

type Identifier struct {
	Base
}

func (i Identifier) PrettyPrint(out *PrintState) *PrintState {
	out.Print(i.Literal())
	return out
}

type Comment struct {
	Base
}

func (c Comment) String() string {
	return c.PrettyPrint(NewPrintState()).String()
}

// TODO: probably refactor/merge/flatten with Node

type ExpressionStatement struct {
	Base
	Val Node
}

func (e ExpressionStatement) Value() Node {
	return e.Val
}

func (e ExpressionStatement) String() string {
	return e.Val.PrettyPrint(NewPrintState()).String()
}

type IntegerLiteral struct {
	Base
	Val int64
}

func (i IntegerLiteral) String() string {
	return i.Literal()
}

type FloatLiteral struct {
	Base
	Val float64
}

func (i FloatLiteral) String() string {
	return i.Literal()
}

type StringLiteral struct {
	Base
	// Val string // Literal is enough to store the string value.
}

func (s StringLiteral) String() string {
	return strconv.Quote(s.Literal())
}

type PrefixExpression struct {
	Base
	Right Node
}

func (p PrefixExpression) Value() Node {
	return p.Right
}

func (p PrefixExpression) PrettyPrint(out *PrintState) *PrintState {
	out.Print("(")
	out.Print(p.Literal())
	p.Right.PrettyPrint(out)
	out.Print(")")
	return out
}

type InfixExpression struct {
	Base
	Left  Node
	Right Node
}

func (i InfixExpression) PrettyPrint(out *PrintState) *PrintState {
	if out.ExpressionLevel > 0 {
		out.Print("(")
	}
	out.ExpressionLevel++
	i.Left.PrettyPrint(out)
	out.Print(" ", i.Literal(), " ")
	i.Left.PrettyPrint(out)
	out.ExpressionLevel--
	if out.ExpressionLevel > 0 {
		out.Print(")")
	}
	return out
}

type Boolean struct {
	Base
	Val bool
}

func (b Boolean) String() string {
	return b.Literal()
}

type IfExpression struct {
	Base
	Condition   Node
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (ie IfExpression) PrettyPrint(out *PrintState) *PrintState {
	out.Print("if ")
	ie.Condition.PrettyPrint(out)
	out.Print(" ")
	ie.Consequence.PrettyPrint(out)

	if ie.Alternative != nil {
		out.Print(" else ")
		ie.Alternative.PrettyPrint(out)
	}
	return out
}

type BlockStatement struct {
	// initially had: Base // holds {
	Program
}

// needed so dumping if and function bodies sort of look like the original.
func (bs BlockStatement) String() string {
	return bs.PrettyPrint(NewPrintState()).String()
}

func (bs BlockStatement) PrettyPrint(ps *PrintState) *PrintState {
	ps.WriteString("{")
	ps.IndentLevel++
	bs.Program.PrettyPrint(ps)
	ps.IndentLevel--
	ps.Println("}")

	return ps
}

// Could specialize the TokenLiteral but... we'll use program's.
/*
func (bs BlockStatement) TokenLiteral() string {
	return "PROGRAM"
}
*/

func PrintList(out *PrintState, list []Node, sep string) {
	for i, p := range list {
		if i > 0 {
			out.Print(sep)
		}
		p.PrettyPrint(out)
	}
}

// Similar to CallExpression.
type Builtin struct {
	Base       // The 'len' or 'first' or... core builtin token
	Parameters []Node
}

func (b Builtin) PrettyPrint(out *PrintState) *PrintState {
	out.Print(b.Literal())
	PrintList(out, b.Parameters, ", ")
	out.Print(")")
	return out
}

type FunctionLiteral struct {
	Base       // The 'func' token
	Parameters []Node
	Body       *BlockStatement
}

func (fl FunctionLiteral) PrettyPrint(out *PrintState) *PrintState {
	out.Print(fl.Literal())
	out.Print("(")
	PrintList(out, fl.Parameters, ", ")
	out.Print(") ")
	out.Print(fl.Body.String())
	return out
}

type CallExpression struct {
	Base           // The '(' token
	Function  Node // Identifier or FunctionLiteral
	Arguments []Node
}

func (ce CallExpression) PrettyPrint(out *PrintState) *PrintState {
	ce.Function.PrettyPrint(out)
	out.Print("(")
	PrintList(out, ce.Arguments, ", ")
	out.Print(")")
	return out
}

type ArrayLiteral struct {
	Base     // The [ token
	Elements []Node
}

func (al ArrayLiteral) PrettyPrint(out *PrintState) *PrintState {

	out.Print("[")
	PrintList(out, al.Elements, ", ")
	out.Print("]")

	return out
}

type IndexExpression struct {
	Base
	Left  Node
	Index Node
}

func (ie IndexExpression) PrettyPrint(out *PrintState) *PrintState {

	out.Print("(")
	ie.Left.PrettyPrint(out)
	out.Print("[")
	ie.Index.PrettyPrint(out)
	out.Print("])")

	return out
}

type MapLiteral struct {
	Base  // the '{' token
	Pairs map[Node]Node
}

func (hl MapLiteral) PrettyPrint(out *PrintState) *PrintState {

	out.Print("{")
	first := true
	for key, value := range hl.Pairs {
		if !first {
			out.Print(", ")
		}
		first = false
		key.PrettyPrint(out)
		out.Print(":")
		value.PrettyPrint(out)
	}
	out.Print("}")
	return out
}

type MacroLiteral struct {
	Base
	Parameters []Node
	Body       *BlockStatement
}

func (ml MacroLiteral) PrettyPrint(out *PrintState) *PrintState {

	out.Print(ml.Literal())
	out.Print("(")
	PrintList(out, ml.Parameters, ", ")
	out.Print(") ")
	out.Print(ml.Body.String())
	return out
}
