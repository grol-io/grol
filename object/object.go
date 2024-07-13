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
	NIL
	ERROR
	RETURN
	FUNCTION
	STRING
	ARRAY
	MAP
	LAST
)

//go:generate stringer -type=Type
var _ = LAST.String() // force compile error if go generate is missing.

var (
	NULL  = Null{}
	TRUE  = Boolean{Value: true}
	FALSE = Boolean{Value: false}
)

func NativeBoolToBooleanObject(input bool) Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func Equals(left, right Object) Object {
	if left.Type() != right.Type() {
		return FALSE
	}
	switch left := left.(type) {
	case *Integer:
		return NativeBoolToBooleanObject(left.Value == right.(*Integer).Value)
	case *String:
		return NativeBoolToBooleanObject(left.Value == right.(*String).Value)
	case *Boolean:
		return NativeBoolToBooleanObject(left.Value == right.(*Boolean).Value)
	case *Null:
		return TRUE
	case *Array:
		return ArrayEquals(left.Elements, right.(*Array).Elements)
	default: /*	ERROR RETURN FUNCTION */
		return FALSE
	}
}

func ArrayEquals(left, right []Object) Object {
	if len(left) != len(right) {
		return FALSE
	}
	for i, l := range left {
		if Equals(l, right[i]) == FALSE {
			return FALSE
		}
	}
	return TRUE
}

type Integer struct {
	Value int64
}

func (i Integer) Inspect() string {
	return strconv.FormatInt(i.Value, 10)
}

func (i Integer) Type() Type {
	return INTEGER
}

type Boolean struct {
	Value bool
}

func (b Boolean) Type() Type {
	return BOOLEAN
}

func (b Boolean) Inspect() string {
	return strconv.FormatBool(b.Value)
}

type String struct {
	Value string
}

func (s String) Type() Type {
	return STRING
}

func (s String) Inspect() string {
	return strconv.Quote(s.Value)
}

type Null struct{}

func (n Null) Type() Type      { return NIL }
func (n Null) Inspect() string { return "nil" }

type Error struct {
	Value string // message
}

func (e Error) Type() Type      { return ERROR }
func (e Error) Inspect() string { return "<err: " + e.Value + ">" }

type ReturnValue struct {
	Value Object
}

func (rv ReturnValue) Type() Type      { return RETURN }
func (rv ReturnValue) Inspect() string { return rv.Value.Inspect() }

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

func (f Function) Type() Type { return FUNCTION }
func (f Function) Inspect() string {
	out := strings.Builder{}

	out.WriteString("func")
	out.WriteString("(")
	ast.WriteStrings(&out, f.Parameters, ", ")
	out.WriteString(") ")
	out.WriteString(f.Body.String())
	return out.String()
}

type Array struct {
	Elements []Object
}

func (ao Array) Type() Type { return ARRAY }
func (ao Array) Inspect() string {
	out := strings.Builder{}
	WriteStrings(&out, ao.Elements, "[", ", ", "]")
	return out.String()
}

type Map struct {
	// possible optimization: us a map of any and put the inner value of object in there, would be faster than
	// wrapping strings etc into object.
	Pairs map[Object]Object
}

func NewMap() *Map {
	return &Map{Pairs: make(map[Object]Object)}
}

func (m Map) Type() Type { return MAP }

func (m Map) Inspect() string {
	out := strings.Builder{}
	out.WriteString("{")
	first := true
	for k, v := range m.Pairs {
		if !first {
			out.WriteString(", ")
		}
		first = false
		out.WriteString(k.Inspect())
		out.WriteString(":")
		out.WriteString(v.Inspect())
	}
	out.WriteString("}")
	return out.String()
}
