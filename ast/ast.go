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

type Priority int8

const (
	_ Priority = iota
	LOWEST
	ASSIGN      // =
	OR          // ||
	AND         // &&
	LAMBDA      // =>
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	DIVIDE      // /
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
	DOTINDEX    // map.str access
)

var Precedences = map[token.Type]Priority{
	token.DEFINE:     ASSIGN,
	token.SUMASSIGN:  ASSIGN,
	token.SUBASSIGN:  ASSIGN,
	token.DIVASSIGN:  ASSIGN,
	token.PRODASSIGN: ASSIGN,
	token.ASSIGN:     ASSIGN,
	token.OR:         OR,
	token.AND:        AND,
	token.COLON:      AND, // range operator and maps (lower than lambda)
	token.EQ:         EQUALS,
	token.NOTEQ:      EQUALS,
	token.LAMBDA:     LAMBDA,
	token.LT:         LESSGREATER,
	token.GT:         LESSGREATER,
	token.LTEQ:       LESSGREATER,
	token.GTEQ:       LESSGREATER,
	token.PLUS:       SUM,
	token.MINUS:      SUM,
	token.BITOR:      SUM,
	token.BITXOR:     SUM,
	token.BITAND:     PRODUCT,
	token.ASTERISK:   PRODUCT,
	token.PERCENT:    PRODUCT,
	token.LEFTSHIFT:  PRODUCT,
	token.RIGHTSHIFT: PRODUCT,
	token.SLASH:      DIVIDE,
	token.INCR:       PREFIX,
	token.DECR:       PREFIX,
	token.LPAREN:     CALL,
	token.LBRACKET:   INDEX,
	token.DOT:        DOTINDEX,
}

//go:generate stringer -type=Priority
var _ = DOTINDEX.String() // force compile error if go generate is missing.

type PrintState struct {
	Out                  io.Writer
	IndentLevel          int
	ExpressionPrecedence Priority
	IndentationDone      bool // already put N number of tabs, reset on each new line
	Compact              bool // don't indent at all (compact mode), no newlines, fewer spaces, no comments
	AllParens            bool // print all expressions fully parenthesized.
	prev                 Node
	last                 string
}

func DebugString(n Node) string {
	ps := NewPrintState()
	ps.Compact = true
	ps.AllParens = true
	n.PrettyPrint(ps)
	return ps.String()
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
	if !ps.Compact {
		_, _ = ps.Out.Write([]byte{'\n'})
	}
	ps.IndentationDone = false
	return ps
}

func (ps *PrintState) Print(str ...string) *PrintState {
	if len(str) == 0 {
		return ps // So for instance Println() doesn't print \t\n.
	}
	if !ps.Compact && !ps.IndentationDone && ps.IndentLevel > 1 {
		_, _ = ps.Out.Write([]byte(strings.Repeat("\t", ps.IndentLevel-1)))
		ps.IndentationDone = true
	}
	for _, s := range str {
		_, _ = ps.Out.Write([]byte(s))
		ps.last = s
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
}

func (b Base) Value() *token.Token {
	return b.Token
}

func (b Base) PrettyPrint(ps *PrintState) *PrintState {
	// In theory should only be called for literals.
	// log.Debugf("PrettyPrint on base called for %T", b.Value())
	return ps.Print(b.Literal())
}

// Break or continue statement.
type ControlExpression struct {
	Base
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
	return ps
}

type Statements struct {
	Base
	Statements []Node
}

func keepSameLineAsPrevious(node Node) bool {
	switch n := node.(type) { //nolint:exahustive // we may add more later
	case *Comment:
		return n.SameLineAsPrevious
	default:
		return false
	}
}

func needNewLineAfter(node Node) bool {
	switch n := node.(type) { //nolint:exahustive // we may add more later
	case *Comment:
		return !n.SameLineAsNext
	default:
		return true
	}
}

func isComment(node Node) bool {
	_, ok := node.(*Comment)
	return ok
}

// Compact mode: Skip comments and decide if we need a space separator or not.
func prettyPrintCompact(ps *PrintState, s Node, i int) bool {
	if isComment(s) {
		return true
	}
	_, prevIsExpr := ps.prev.(*InfixExpression)
	_, curIsArray := s.(*ArrayLiteral)
	if curIsArray || (prevIsExpr && ps.last != "}" && ps.last != "]") {
		if i > 0 {
			_, _ = ps.Out.Write([]byte{' '})
		}
	} else if i > 0 {
		// Add space between identifiers and builtins/function calls
		_, isIdentifier := s.(*Identifier)
		_, isBuiltin := s.(*Builtin)
		_, isCall := s.(*CallExpression)
		_, prevIsIdentifier := ps.prev.(*Identifier)
		if (isIdentifier || isBuiltin || isCall) && prevIsIdentifier {
			_, _ = ps.Out.Write([]byte{' '})
		}
	}
	return false
}

// Normal/long form print: Decide if using new line or space as separator.
func prettyPrintLongForm(ps *PrintState, s Node, i int) {
	if i > 0 || ps.IndentLevel > 1 {
		if keepSameLineAsPrevious(s) || !needNewLineAfter(ps.prev) {
			log.Debugf("=> PrettyPrint adding just a space")
			_, _ = ps.Out.Write([]byte{' '})
			ps.IndentationDone = true
		} else {
			log.Debugf("=> PrettyPrint adding newline")
			ps.Println()
		}
	}
}

func (p Statements) PrettyPrint(ps *PrintState) *PrintState {
	oldExpressionPrecedence := ps.ExpressionPrecedence
	if ps.IndentLevel > 0 {
		ps.Print("{") // first statement might be a comment on same line.
	}
	ps.IndentLevel++
	ps.ExpressionPrecedence = LOWEST
	var i int
	for _, s := range p.Statements {
		if ps.Compact {
			if prettyPrintCompact(ps, s, i) {
				continue // skip comments entirely.
			}
		} else {
			prettyPrintLongForm(ps, s, i)
		}
		s.PrettyPrint(ps)
		ps.prev = s
		i++
	}
	ps.Println()
	ps.IndentLevel--
	ps.ExpressionPrecedence = oldExpressionPrecedence
	if ps.IndentLevel > 0 {
		ps.Print("}")
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
	SameLineAsPrevious bool
	SameLineAsNext     bool
}

func (c Comment) PrettyPrint(out *PrintState) *PrintState {
	out.Print(c.Literal())
	return out
}

type IntegerLiteral struct {
	Base
	Val int64
}

type FloatLiteral struct {
	Base
	Val float64
}

type StringLiteral struct {
	Base
	// Val string // Base.Token.Literal is enough to store the string value.
}

func (s StringLiteral) PrettyPrint(ps *PrintState) *PrintState {
	ps.Print(strconv.Quote(s.Literal()))
	return ps
}

type PrefixExpression struct {
	Base
	Right Node
}

func (ps *PrintState) needParen(t *token.Token) (bool, Priority) {
	newPrecedence, ok := Precedences[t.Type()]
	if !ok {
		panic("precedence not found for " + t.Literal())
	}
	oldPrecedence := ps.ExpressionPrecedence
	ps.ExpressionPrecedence = newPrecedence
	return ps.AllParens || newPrecedence < oldPrecedence, oldPrecedence
}

func (p PrefixExpression) PrettyPrint(out *PrintState) *PrintState {
	oldPrecedence := out.ExpressionPrecedence
	out.ExpressionPrecedence = PREFIX
	needParen := out.AllParens || PREFIX <= oldPrecedence // double prefix like -(-a) needs parens to not become --a prefix.
	if needParen {
		out.Print("(")
	}
	out.Print(p.Literal())
	p.Right.PrettyPrint(out)
	out.ExpressionPrecedence = oldPrecedence
	if needParen {
		out.Print(")")
	}
	return out
}

type PostfixExpression struct {
	Base
	Prev *token.Token
}

func (p PostfixExpression) PrettyPrint(out *PrintState) *PrintState {
	needParen, oldPrecedence := out.needParen(p.Token)
	if needParen {
		out.Print("(")
	}
	out.Print(p.Prev.Literal())
	out.Print(p.Literal())
	if needParen {
		out.Print(")")
	}
	out.ExpressionPrecedence = oldPrecedence
	return out
}

type InfixExpression struct {
	Base
	Left  Node
	Right Node
}

func (i InfixExpression) PrettyPrint(out *PrintState) *PrintState {
	needParen, oldPrecedence := out.needParen(i.Token)
	if needParen {
		out.Print("(")
	}
	i.Left.PrettyPrint(out)
	if out.Compact {
		out.Print(i.Literal())
	} else {
		out.Print(" ", i.Literal(), " ")
	}
	// Can be nil and shouldn't print nil for colon operator in slice expressions
	if i.Right != nil {
		i.Right.PrettyPrint(out)
	}
	if needParen {
		out.Print(")")
	}
	out.ExpressionPrecedence = oldPrecedence
	return out
}

type Boolean struct {
	Base
	Val bool
}

type ForExpression struct {
	Base
	Condition Node
	Body      *Statements
}

func (fe ForExpression) PrettyPrint(out *PrintState) *PrintState {
	out.Print("for ")
	fe.Condition.PrettyPrint(out)
	if !out.Compact {
		out.Print(" ")
	}
	fe.Body.PrettyPrint(out)
	return out
}

type IfExpression struct {
	Base
	Condition   Node
	Consequence *Statements
	Alternative *Statements
}

func (ie IfExpression) printElse(out *PrintState) {
	if out.Compact {
		out.Print("else")
	} else {
		out.Print(" else ")
	}
	if len(ie.Alternative.Statements) == 1 && ie.Alternative.Statements[0].Value().Type() == token.IF {
		// else if
		if out.Compact {
			out.Print(" ")
		}
		ie.Alternative.Statements[0].PrettyPrint(out)
		return
	}
	ie.Alternative.PrettyPrint(out)
}

func (ie IfExpression) PrettyPrint(out *PrintState) *PrintState {
	out.Print("if ")
	ie.Condition.PrettyPrint(out)
	if !out.Compact {
		out.Print(" ")
	}
	ie.Consequence.PrettyPrint(out)
	if ie.Alternative != nil {
		ie.printElse(out)
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
	out.Print("(")
	out.ComaList(b.Parameters)
	out.Print(")")
	return out
}

type FunctionLiteral struct {
	Base       // The 'func' or '=>' token
	Name       *Identifier
	Parameters []Node // last one might be `..` for variadic.
	Body       *Statements
	Variadic   bool
	IsLambda   bool
}

func (fl FunctionLiteral) lambdaPrint(out *PrintState) *PrintState {
	needParen := len(fl.Parameters) != 1
	if needParen {
		out.Print("(")
	}
	out.ComaList(fl.Parameters)
	if needParen {
		out.Print(")")
	}
	if out.Compact {
		out.Print("=>")
	} else {
		out.Print(" => ")
	}
	fl.Body.PrettyPrint(out)
	return out
}

func (fl FunctionLiteral) PrettyPrint(out *PrintState) *PrintState {
	if fl.IsLambda {
		return fl.lambdaPrint(out)
	}
	out.Print(fl.Literal())
	if fl.Name != nil {
		out.Print(" ")
		out.Print(fl.Name.Literal())
	}
	out.Print("(")
	out.ComaList(fl.Parameters)
	if out.Compact {
		out.Print(")")
	} else {
		out.Print(") ")
	}
	fl.Body.PrettyPrint(out)
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
	oldExpressionPrecedence := out.ExpressionPrecedence
	out.ExpressionPrecedence = LOWEST
	out.ComaList(ce.Arguments)
	out.ExpressionPrecedence = oldExpressionPrecedence
	out.Print(")")
	return out
}

type ArrayLiteral struct {
	Base     // The [ token
	Elements []Node
}

func (al ArrayLiteral) PrettyPrint(out *PrintState) *PrintState {
	out.Print("[")
	out.ComaList(al.Elements)
	out.Print("]")
	return out
}

type IndexExpression struct {
	Base
	Left  Node
	Index Node
}

func (ie IndexExpression) PrettyPrint(out *PrintState) *PrintState {
	needParen, oldExpressionPrecedence := out.needParen(ie.Token)
	if needParen {
		out.Print("(")
	}
	ie.Left.PrettyPrint(out)
	out.Print(ie.Literal())
	out.ExpressionPrecedence = LOWEST
	ie.Index.PrettyPrint(out)
	if ie.Token.Type() == token.LBRACKET {
		out.Print("]")
	}
	if needParen {
		out.Print(")")
	}
	out.ExpressionPrecedence = oldExpressionPrecedence
	return out
}

type MapLiteral struct {
	Base  // the '{' token
	Pairs map[Node]Node
	Order []Node // for pretty printing in same order as input
}

func (hl MapLiteral) PrettyPrint(out *PrintState) *PrintState {
	out.Print("{")
	sep := ", "
	if out.Compact {
		sep = ","
	}
	for i, key := range hl.Order {
		if i > 0 {
			out.Print(sep)
		}
		key.PrettyPrint(out)
		out.Print(":")
		hl.Pairs[key].PrettyPrint(out)
	}
	out.Print("}")
	return out
}

type MacroLiteral struct {
	Base
	Parameters []Node
	Body       *Statements
}

func (ml MacroLiteral) PrettyPrint(out *PrintState) *PrintState {
	out.Print(ml.Literal())
	out.Print("(")
	out.ComaList(ml.Parameters)
	if out.Compact {
		out.Print(")")
	} else {
		out.Print(") ")
	}
	ml.Body.PrettyPrint(out)
	return out
}

func (ps *PrintState) ComaList(list []Node) {
	sep := ", "
	if ps.Compact {
		sep = ","
	}
	PrintList(ps, list, sep)
}
