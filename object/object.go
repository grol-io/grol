package object

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"fortio.org/log"
	"grol.io/grol/ast"
)

type Type uint8

type Object interface {
	Type() Type
	Inspect() string
	Unwrap() any
}

const (
	UNKNOWN Type = iota
	INTEGER
	FLOAT
	BOOLEAN
	NIL
	ERROR
	RETURN
	FUNC
	STRING
	ARRAY
	MAP
	QUOTE
	MACRO
	EXTENSION
	LAST
)

//go:generate stringer -type=Type
var _ = LAST.String() // force compile error if go generate is missing.

var (
	NULL  = Null{}
	TRUE  = Boolean{Value: true}
	FALSE = Boolean{Value: false}
)

/* Wish this could be used/useful:
type Number interface {
	Integer | Float
}
*/

func Hashable(o Object) bool {
	switch o.Type() { //nolint:exhaustive // We have all the types that are hashable + default for the others.
	case INTEGER, FLOAT, BOOLEAN, NIL, ERROR, RETURN, QUOTE, STRING:
		return true
	default:
		return false
	}
}

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
	case Integer:
		return NativeBoolToBooleanObject(left.Value == right.(Integer).Value)
	case String:
		return NativeBoolToBooleanObject(left.Value == right.(String).Value)
	case Boolean:
		return NativeBoolToBooleanObject(left.Value == right.(Boolean).Value)
	case Null:
		return TRUE
	case Array:
		return ArrayEquals(left.Elements, right.(Array).Elements)
	case Map:
		return MapEquals(left, right.(Map))
	default: /*	ERROR RETURN FUNC */
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

func MapEquals(left, right Map) Object {
	if len(left) != len(right) {
		return FALSE
	}
	for k, v := range left {
		if Equals(v, right[k]) == FALSE {
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

func (i Integer) Unwrap() any {
	return i.Value
}

func (i Integer) Type() Type {
	return INTEGER
}

type Float struct {
	Value float64
}

func (f Float) Unwrap() any {
	return f.Value
}

func (f Float) Type() Type {
	return FLOAT
}

func (f Float) Inspect() string {
	return strconv.FormatFloat(f.Value, 'f', -1, 64)
}

type Boolean struct {
	Value bool
}

func (b Boolean) Unwrap() any {
	return b.Value
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

func (s String) Unwrap() any {
	return s.Value
}

func (s String) Type() Type {
	return STRING
}

func (s String) Inspect() string {
	return strconv.Quote(s.Value)
}

type Null struct{}

func (n Null) Unwrap() any     { return nil }
func (n Null) Type() Type      { return NIL }
func (n Null) Inspect() string { return "nil" }

type Error struct {
	Value string // message
}

func (e Error) Unwrap() any     { return e }
func (e Error) Error() string   { return e.Value }
func (e Error) Type() Type      { return ERROR }
func (e Error) Inspect() string { return "<err: " + e.Value + ">" }

type ReturnValue struct {
	Value Object
}

func (rv ReturnValue) Unwrap() any     { return rv.Value }
func (rv ReturnValue) Type() Type      { return RETURN }
func (rv ReturnValue) Inspect() string { return rv.Value.Inspect() }

type Function struct {
	Parameters []ast.Node
	Name       *ast.Identifier
	CacheKey   string
	Body       *ast.Statements
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

func (f Function) Unwrap() any { return f }
func (f Function) Type() Type  { return FUNC }

// Must be called after the function is fully initialized.
// Whether a function result should be cached doesn't depend on the Name,
// so it's not part of the cache key.
func (f *Function) SetCacheKey() string {
	out := strings.Builder{}
	out.WriteString("func")
	f.CacheKey = f.finishFuncOutput(&out)
	return f.CacheKey
}

// Common part of Inspect and SetCacheKey. Outputs the rest of the function.
func (f *Function) finishFuncOutput(out *strings.Builder) string {
	out.WriteString("(")
	ps := &ast.PrintState{Out: out, Compact: true}
	ps.ComaList(f.Parameters)
	out.WriteString("){")
	f.Body.PrettyPrint(ps)
	out.WriteString("}")
	return out.String()
}

func (f Function) Inspect() string {
	if f.Name == nil {
		return f.CacheKey
	}
	out := strings.Builder{}
	out.WriteString("func ")
	out.WriteString(f.Name.Literal())
	return f.finishFuncOutput(&out)
}

type Array struct {
	Elements []Object
}

func (ao Array) Unwrap() any { return Unwrap(ao.Elements) }
func (ao Array) Type() Type  { return ARRAY }
func (ao Array) Inspect() string {
	out := strings.Builder{}
	WriteStrings(&out, ao.Elements, "[", ",", "]")
	return out.String()
}

// possible optimization: us a map of any and put the inner value of object in there, would be faster than
// wrapping strings etc into object.
type Map map[Object]Object

func NewMap() Map {
	return make(map[Object]Object)
}

type MapKeys []Object

func (mk MapKeys) Len() int {
	return len(mk)
}

func (mk MapKeys) Less(i, j int) bool {
	ti := mk[i].Type()
	tj := mk[j].Type()
	if ti < tj {
		return true
	}
	if ti > tj {
		return false
	}
	switch ti { //nolint:exhaustive // We have all the types that exist and can be in a map.
	case INTEGER:
		return mk[i].(Integer).Value < mk[j].(Integer).Value
	case FLOAT:
		return mk[i].(Float).Value < mk[j].(Float).Value
	case BOOLEAN:
		bi := mk[i].(Boolean).Value
		bj := mk[j].(Boolean).Value
		if bi {
			return false
		}
		return bj
	case STRING:
		return mk[i].(String).Value < mk[j].(String).Value
	default:
		log.Warnf("Unexpected type in map keys: %s", ti)
		// UNKNOWN, NIL, ERROR, RETURN, FUNC, ARRAY, MAP, QUOTE, MACRO, LAST
	}
	return false
}

func (mk MapKeys) Swap(i, j int) {
	mk[i], mk[j] = mk[j], mk[i]
}

func (m Map) Unwrap() any {
	res := make(map[any]any, len(m))
	for k, v := range m {
		res[k.Unwrap()] = v.Unwrap()
	}
	return res
}

func (m Map) Type() Type { return MAP }

func (m Map) Inspect() string {
	out := strings.Builder{}
	out.WriteString("{")
	keys := make(MapKeys, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	// Sort the keys
	sort.Sort(keys)
	for i, k := range keys {
		if i != 0 {
			out.WriteString(",")
		}
		v := m[k]
		out.WriteString(k.Inspect())
		out.WriteString(":")
		out.WriteString(v.Inspect())
	}
	out.WriteString("}")
	return out.String()
}

type Quote struct {
	Node ast.Node
}

func (q Quote) Unwrap() any { return q.Node }
func (q Quote) Type() Type  { return QUOTE }
func (q Quote) Inspect() string {
	out := strings.Builder{}
	out.WriteString("quote(")
	q.Node.PrettyPrint(&ast.PrintState{Out: &out})
	out.WriteString(")")
	return out.String()
}

type Macro struct {
	Parameters []ast.Node
	Body       *ast.Statements
	Env        *Environment
}

func (m Macro) Unwrap() any { return m }
func (m Macro) Type() Type  { return MACRO }
func (m Macro) Inspect() string {
	out := strings.Builder{}
	out.WriteString("macro(")
	ps := &ast.PrintState{Out: &out, Compact: true}
	ps.ComaList(m.Parameters)
	out.WriteString("){")
	m.Body.PrettyPrint(ps)
	out.WriteString("}")
	return out.String()
}

// Extensions are functions implemented in go and exposed to grol.
type Extension struct {
	Name     string      // Name to make the function available as in grol.
	MinArgs  int         // Minimum number of arguments required.
	MaxArgs  int         // Maximum number of arguments allowed. -1 for unlimited.
	ArgTypes []Type      // Type of each argument, provided at least up to MinArgs.
	Callback ExtFunction // The go function or lambda to call when the grol by Name(...) is invoked.
}

// ExtFunction is the signature of what grol will call when the extension is invoked.
// Incoming arguments are validated for type and number of arguments based on [Extension].
type ExtFunction func(args []Object) Object

func (e *Extension) Usage(out *strings.Builder) {
	for i := 1; i <= e.MinArgs; i++ {
		if i > 1 {
			out.WriteString(", ")
		}
		t := strings.ToLower(e.ArgTypes[i-1].String())
		out.WriteString(t)
	}
	switch {
	case e.MaxArgs < 0:
		out.WriteString(", ...")
	case e.MaxArgs > e.MinArgs:
		out.WriteString(fmt.Sprintf(", arg%d...arg%d", e.MinArgs+1, e.MaxArgs))
	}
}

func (e Extension) Unwrap() any { return e }
func (e Extension) Type() Type  { return EXTENSION }
func (e Extension) Inspect() string {
	out := strings.Builder{}
	out.WriteString(e.Name)
	out.WriteString("(")
	e.Usage(&out)
	out.WriteString(")")
	return out.String()
}
