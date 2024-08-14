package object

import (
	"fmt"
	"io"
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
	JSON(out io.Writer) error
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
	ANY // A marker, for extensions, not a real type.
)

//go:generate stringer -type=Type
var _ = ANY.String() // force compile error if go generate is missing.

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
	case INTEGER, FLOAT, BOOLEAN, NIL, ERROR, STRING:
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

func Equals(left, right Object) bool {
	if left.Type() != right.Type() {
		return false
	}
	switch left := left.(type) {
	case Integer:
		return left.Value == right.(Integer).Value
	case Float:
		return left.Value == right.(Float).Value
	case String:
		return left.Value == right.(String).Value
	case Boolean:
		return left.Value == right.(Boolean).Value
	case Null:
		return true
	case Array:
		return ArrayEquals(left.Elements, right.(Array).Elements)
	case Map:
		return MapEquals(left, right.(Map))
	case Error:
		return left.Value == right.(Error).Value
	case ReturnValue:
		return Equals(left.Value, right.(ReturnValue).Value)
	case Function:
		return left.CacheKey == right.(Function).CacheKey
	case Macro:
		return left.Env == right.(Macro).Env && left.Body == right.(Macro).Body
	case Extension:
		return left.Name == right.(Extension).Name // They are enforced to be constant by name.
	default:
		/* QUOTE should be the only one left from switch above... where is exhaustive linter when you need it */
		log.Warnf("Unexpected type in equals: %s", left.Inspect())
		return false
	}
}

func ArrayEquals(left, right []Object) bool {
	if len(left) != len(right) {
		return false
	}
	for i, l := range left {
		if !Equals(l, right[i]) {
			return false
		}
	}
	return true
}

func MapEquals(left, right Map) bool {
	if len(left) != len(right) {
		return false
	}
	for k, v := range left {
		if !Equals(v, right[k]) {
			return false
		}
	}
	return true
}

type Integer struct {
	Value int64
}

func (i Integer) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%d", i.Value)
	return err
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

func (f Float) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%f", f.Value)
	return err
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

func (b Boolean) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%t", b.Value)
	return err
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

func (s String) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, "%q", s.Value)
	return err
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

func (n Null) JSON(w io.Writer) error {
	_, err := w.Write([]byte("null"))
	return err
}
func (n Null) Unwrap() any     { return nil }
func (n Null) Type() Type      { return NIL }
func (n Null) Inspect() string { return "nil" }

type Error struct {
	Value string // message
}

func (e Error) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, `{"err":%q}`, e.Value)
	return err
}
func (e Error) Unwrap() any     { return e }
func (e Error) Error() string   { return e.Value }
func (e Error) Type() Type      { return ERROR }
func (e Error) Inspect() string { return "<err: " + e.Value + ">" }

type ReturnValue struct {
	Value Object
}

func (rv ReturnValue) JSON(w io.Writer) error {
	_, _ = w.Write([]byte(`{"return":`))
	err := rv.Value.JSON(w)
	_, _ = w.Write([]byte("}"))
	return err
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
	Variadic   bool
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

func (f Function) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, `{"func":%q}`, f.Inspect())
	return err
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

func (ao Array) JSON(w io.Writer) error {
	_, _ = w.Write([]byte("["))
	for i, p := range ao.Elements {
		if i > 0 {
			_, _ = w.Write([]byte(","))
		}
		err := p.JSON(w)
		if err != nil {
			return err
		}
	}
	_, err := w.Write([]byte("]"))
	return err
}

// possible optimization: us a map of any and put the inner value of object in there, would be faster than
// wrapping strings etc into object.
type Map map[Object]Object

func NewMap() Map {
	return make(map[Object]Object)
}

func (ao Array) Len() int {
	return len(ao.Elements)
}

func (ao Array) Less(i, j int) bool {
	ti := ao.Elements[i].Type()
	tj := ao.Elements[j].Type()
	if ti < tj {
		return true
	}
	if ti > tj {
		return false
	}
	switch ti { //nolint:exhaustive // We have all the types that exist and can be in a map.
	case INTEGER:
		return ao.Elements[i].(Integer).Value < ao.Elements[j].(Integer).Value
	case FLOAT:
		return ao.Elements[i].(Float).Value < ao.Elements[j].(Float).Value
	case BOOLEAN:
		bi := ao.Elements[i].(Boolean).Value
		bj := ao.Elements[j].(Boolean).Value
		if bi {
			return false
		}
		return bj
	case STRING:
		return ao.Elements[i].(String).Value < ao.Elements[j].(String).Value
	default:
		log.Warnf("Unexpected type in map keys: %s", ti)
		// UNKNOWN, NIL, ERROR, RETURN, FUNC, ARRAY, MAP, QUOTE, MACRO, LAST
	}
	return false
}

func (ao Array) Swap(i, j int) {
	ao.Elements[i], ao.Elements[j] = ao.Elements[j], ao.Elements[i]
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
	keys := make([]Object, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	arr := Array{Elements: keys}
	// Sort the keys
	sort.Sort(arr)
	for i, k := range arr.Elements {
		if i != 0 {
			out.WriteString(",")
		}
		v, ok := m[k]
		if !ok {
			// We got a NaN.
			v = Error{Value: "NaN in map"}
		}
		out.WriteString(k.Inspect())
		out.WriteString(":")
		out.WriteString(v.Inspect())
	}
	out.WriteString("}")
	return out.String()
}

func (m Map) JSON(w io.Writer) error {
	_, _ = w.Write([]byte("{"))
	keys := make([]Object, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	arr := Array{Elements: keys}
	// Sort the keys
	sort.Sort(arr)
	for i, k := range arr.Elements {
		if i > 0 {
			_, _ = w.Write([]byte(","))
		}
		if _, ok := k.(String); !ok {
			_, _ = fmt.Fprintf(w, "%q", k.Inspect()) // as JSON keys must be strings
		} else {
			_, _ = w.Write([]byte(k.Inspect()))
		}
		_, _ = w.Write([]byte(":"))
		v := m[k]
		err := v.JSON(w)
		if err != nil {
			return err
		}
	}
	_, err := w.Write([]byte("}"))
	return err
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

func (q Quote) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, `{"quote":%q}`, q.Inspect())
	return err
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

func (m Macro) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, `{"macro":%q}`, m.Inspect())
	return err
}

// Extensions are functions implemented in go and exposed to grol.
type Extension struct {
	Name     string      // Name to make the function available as in grol.
	MinArgs  int         // Minimum number of arguments required.
	MaxArgs  int         // Maximum number of arguments allowed. -1 for unlimited.
	ArgTypes []Type      // Type of each argument, provided at least up to MinArgs.
	Callback ExtFunction // The go function or lambda to call when the grol by Name(...) is invoked.
	Variadic bool        // MaxArgs > MinArgs
}

// Adapter for functions that only need the argumants.
func ShortCallback(f ShortExtFunction) ExtFunction {
	return func(_ any, _ string, args []Object) Object {
		return f(args)
	}
}

// ShortExtFunction is the signature for callbacks that do not need more than the arguments (like math functions).
type ShortExtFunction func(args []Object) Object

// ExtFunction is the signature of what grol will call when the extension is invoked.
// Incoming arguments are validated for type and number of arguments based on [Extension].
// eval is the opaque state passed from the interpreter, it can be used with eval.Eval etc
// name is the function name as registered under.
type ExtFunction func(eval any, name string, args []Object) Object

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
		out.WriteString(", ..")
	case e.MaxArgs > e.MinArgs:
		out.WriteString(fmt.Sprintf(", arg%d..arg%d", e.MinArgs+1, e.MaxArgs))
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

func (e Extension) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, `{"gofunc":%q}`, e.Inspect())
	return err
}
