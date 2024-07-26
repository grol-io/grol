// Abstract Syntax Tree for the GROL language.
// Everything is Node. Has a Token() and can be PrettyPrint'ed back to source
// that would parse to the same AST.
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
	IndentationDone bool // already put N number of tabs, reset on each new line
}

func NewPrintState() *PrintState {
	return &PrintState{Out: &strings.Builder{}}
}

func (ps *PrintState) String() string {
	return ps.Out.(*strings.Builder).String()
}

// Will print indented to current level. with a newline at the end.
// Only a single indentation per line.
func (ps *PrintState) Println(str ...string) *PrintState {
	ps.Print(str...)
	_, _ = ps.Out.Write([]byte{'\n'})
	ps.IndentationDone = false
	return ps
}

func (ps *PrintState) Print(str ...string) *PrintState {
	if !ps.IndentationDone {
		_, _ = ps.Out.Write([]byte(strings.Repeat("\t", ps.IndentLevel)))
		ps.IndentationDone = true
	}
	for _, s := range str {
		_, _ = ps.Out.Write([]byte(s))
	}
	return ps
}

// --- AST nodes

// Everything in the tree is a Node.
type Node interface {
	Value() *token.Token
	PrettyPrint(ps *PrintState) *PrintState
}

// Common to all nodes that have a token and avoids repeating the same TokenLiteral() methods.
type Base struct {
	*token.Token
	Node // TBD on assignment to self/chaining.
}

func (b Base) Value() *token.Token {
	return b.Token
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

type BlockStatement = Program //  TODO rename both to Statetements later

func (p Program) String() string {
	return p.PrettyPrint(NewPrintState()).String()
}

func (p Program) PrettyPrint(ps *PrintState) *PrintState {
	if ps.IndentLevel > 0 {
		ps.Println("{")
	}
	ps.IndentLevel++
	for _, s := range p.Statements {
		s.PrettyPrint(ps)
	}
	ps.IndentLevel--
	if ps.IndentLevel > 0 {
		ps.Println("}")
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

type StringLiteral struct {
	Base
	// Val string // Base.Token.Literal is enough to store the string value.
}

func (s StringLiteral) String() string {
	return strconv.Quote(s.Literal())
}

type PrefixExpression struct {
	Base
	Right Node
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
