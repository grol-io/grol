package object

import (
	"strconv"
	"strings"

	"github.com/ldemailly/gorepl/ast"
)

type Type uint8

type Object interface {
	Type() Type
	Inspect() string
}

const (
	UNKNOWN Type = iota
	INTEGER
	BOOLEAN
	NULL
	ERROR
	RETURN
	FUNCTION
	STRING
	ARRAY
	LAST
)

//go:generate stringer -type=Type
var _ = LAST.String() // force compile error if go generate is missing.

type Integer struct {
	Value int64
}

func (i *Integer) Inspect() string {
	return strconv.FormatInt(i.Value, 10)
}

func (i *Integer) Type() Type {
	return INTEGER
}

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() Type {
	return BOOLEAN
}

func (b *Boolean) Inspect() string {
	return strconv.FormatBool(b.Value)
}

type String struct {
	Value string
}

func (s *String) Type() Type {
	return STRING
}

func (s *String) Inspect() string {
	return strconv.Quote(s.Value)
}

type Null struct{}

func (n *Null) Type() Type      { return NULL }
func (n *Null) Inspect() string { return "<null>" }

type Error struct {
	Value string // message
}

func (e *Error) Type() Type      { return ERROR }
func (e *Error) Inspect() string { return "<err: " + e.Value + ">" }

type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() Type      { return RETURN }
func (rv *ReturnValue) Inspect() string { return rv.Value.Inspect() }

type Function struct {
	Parameters []*ast.Identifier
	Body       *ast.BlockStatement
	Env        *Environment
}

func WriteStrings(out *strings.Builder, list []Object, before, sep, after string) {
	out.WriteString(before)
	for i, p := range list {
		if i > 0 {
			out.WriteString(sep)
		}
		out.WriteString(p.Inspect())
	}
	out.WriteString(after)
}

func (f *Function) Type() Type { return FUNCTION }
func (f *Function) Inspect() string {
	out := strings.Builder{}

	out.WriteString("fn")
	out.WriteString("(")
	ast.WriteStrings(&out, f.Parameters, ", ")
	out.WriteString(") ")
	out.WriteString(f.Body.String())
	return out.String()
}

type Array struct {
	Elements []Object
}

func (ao *Array) Type() Type { return ARRAY }
func (ao *Array) Inspect() string {
	out := strings.Builder{}
	WriteStrings(&out, ao.Elements, "[", ", ", "]")
	return out.String()
}
