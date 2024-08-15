package object

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

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
	FLOAT // These 2 must stay in that order for areIntFloat to work.
	BOOLEAN
	NIL
	ERROR
	RETURN
	FUNC
	STRING
	ARRAY
	MAP // Ordered map and allows any key type including more maps/arrays/functions/...
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

// Hashable in tem of Go map for cache key.
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
		return false // int and float aren't the same even though they can Cmp to the same value.
	}
	return Cmp(left, right) == 0
}

func Cmp(ei, ej Object) int {
	ti := ei.Type()
	tj := ej.Type()
	if areIntFloat(ti, tj) {
		// We have float and integer, let's sort them together.
		var v1, v2 float64
		if ti == INTEGER {
			v1 = float64(ei.(Integer).Value)
			v2 = ej.(Float).Value
		} else {
			v1 = ei.(Float).Value
			v2 = float64(ej.(Integer).Value)
		}
		return cmp.Compare(v1, v2)
	}
	if ti < tj {
		return -1
	}
	if ti > tj {
		return 1
	}
	// same types at this point.
	switch ti {
	case EXTENSION:
		return cmp.Compare(ei.(Extension).Name, ej.(Extension).Name)
	case FUNC:
		return cmp.Compare(ei.(Function).CacheKey, ej.(Function).CacheKey)
	case MAP:
		m1 := ei.(*Map)
		m2 := ej.(*Map)
		if len(m1.kv) < len(m2.kv) {
			return -1
		}
		if len(m1.kv) > len(m2.kv) {
			return 1
		}
		for i, kv := range m1.kv {
			if c := Cmp(kv.Key, m2.kv[i].Key); c != 0 {
				return c
			}
			if c := Cmp(kv.Value, m2.kv[i].Value); c != 0 {
				return c
			}
		}
		return 0
	case ARRAY:
		a1 := ei.(Array)
		a2 := ej.(Array)
		if len(a1.Elements) < len(a2.Elements) {
			return -1
		}
		if len(a1.Elements) > len(a2.Elements) {
			return 1
		}
		for i, l := range a1.Elements {
			if c := Cmp(l, a2.Elements[i]); c != 0 {
				return c
			}
		}
		return 0
	case ERROR:
		return cmp.Compare(ei.(Error).Value, ej.(Error).Value)
	case NIL:
		return 0
	case INTEGER:
		return cmp.Compare(ei.(Integer).Value, ej.(Integer).Value)
	case FLOAT:
		return cmp.Compare(ei.(Float).Value, ej.(Float).Value)
	case BOOLEAN:
		bi := ei.(Boolean).Value
		bj := ej.(Boolean).Value
		if bi == bj {
			return 0
		}
		if bi {
			return 1
		}
		return -1
	case STRING:
		return cmp.Compare(ei.(String).Value, ej.(String).Value)

	// RETURN, QUOTE, MACRO, ANY aren't expected to be compared.
	case RETURN, QUOTE, MACRO, UNKNOWN, ANY:
		panic(fmt.Sprintf("Unexpected type in Cmp: %s", ti))
	}
	return 1
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

func MapEquals(left, right *Map) bool {
	if len(left.kv) != len(right.kv) {
		return false
	}
	for i, kv := range left.kv {
		if !Equals(kv.Key, right.kv[i].Key) {
			return false
		}
		if !Equals(kv.Value, right.kv[i].Value) {
			return false
		}
	}
	return true
}

func CompareKeys(a, b KV) int {
	return Cmp(a.Key, b.Key)
}

func (m *Map) Get(key Object) (Object, bool) {
	kv := KV{Key: key}
	i, ok := slices.BinarySearchFunc(m.kv, kv, CompareKeys) // log(n) search as we keep it sorted.
	if !ok {
		return NULL, false
	}
	return m.kv[i].Value, true
}

func (m *Map) Set(key, value Object) {
	kv := KV{Key: key, Value: value}
	i, ok := slices.BinarySearchFunc(m.kv, kv, CompareKeys)
	if ok {
		m.kv[i].Value = value
		return
	}
	m.kv = slices.Insert(m.kv, i, kv)
}

func (m *Map) Len() int {
	return len(m.kv)
}

func (m *Map) KV() []KV {
	return m.kv
}

var (
	KeyKey   = String{Value: "key"}
	ValueKey = String{Value: "value"}
)

func (m *Map) First() Object {
	if len(m.kv) == 0 {
		return NULL
	}
	return &Map{kv: []KV{
		{Key: KeyKey, Value: m.kv[0].Key},
		{Key: ValueKey, Value: m.kv[0].Value},
	}}
}

func (m *Map) Rest() Object {
	if len(m.kv) <= 1 {
		return NULL
	}
	res := &Map{kv: m.kv[1:]}
	return res
}

// Creates a new Map appending the right map to the left map.
func (m *Map) Append(right *Map) *Map {
	// important to avoid underlying array mutation.
	// note: when we do map mutations, will need to copy instead.
	res := &Map{kv: m.kv[:len(m.kv):len(m.kv)]}
	for _, kv := range right.kv {
		res.Set(kv.Key, kv.Value)
	}
	return res
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

// KeyValue pairs, what we have inside Map.
type KV struct {
	Key   Object
	Value Object
}

// Sorted KV pairs, O(n) insert O(log n) access/same key mutations.
type Map struct {
	kv []KV
}

func NewMap() Map {
	return Map{kv: make([]KV, 0, 4)}
}

func (ao Array) Len() int {
	return len(ao.Elements)
}

func areIntFloat(a, b Type) bool {
	l := min(a, b)
	h := max(a, b)
	return l == INTEGER && h == FLOAT
}

func (ao Array) Less(i, j int) bool {
	return Cmp(ao.Elements[i], ao.Elements[j]) < 0
}

func (ao Array) Swap(i, j int) {
	ao.Elements[i], ao.Elements[j] = ao.Elements[j], ao.Elements[i]
}

func (m *Map) Unwrap() any {
	res := make(map[any]any, len(m.kv))
	for _, kv := range m.kv {
		res[kv.Key.Unwrap()] = kv.Value.Unwrap()
	}
	return res
}

func (m *Map) Type() Type { return MAP }

func (m *Map) Inspect() string {
	out := strings.Builder{}
	out.WriteString("{")
	for i, kv := range m.kv {
		if i != 0 {
			out.WriteString(",")
		}
		out.WriteString(kv.Key.Inspect())
		out.WriteString(":")
		out.WriteString(kv.Value.Inspect())
	}
	out.WriteString("}")
	return out.String()
}

func (m *Map) JSON(w io.Writer) error {
	_, _ = w.Write([]byte("{"))
	for i, kv := range m.kv {
		if i > 0 {
			_, _ = w.Write([]byte(","))
		}
		if _, ok := kv.Key.(String); !ok {
			_, _ = fmt.Fprintf(w, "%q", kv.Key.Inspect()) // as JSON keys must be strings
		} else {
			_, _ = w.Write([]byte(kv.Key.Inspect()))
		}
		_, _ = w.Write([]byte(":"))
		err := kv.Value.JSON(w)
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
	case e.MinArgs == 0 && e.MaxArgs == 1:
		arg := "arg"
		if len(e.ArgTypes) > 0 {
			arg = strings.ToLower(e.ArgTypes[0].String())
		}
		out.WriteString("[") // to indicate optional
		out.WriteString(arg)
		out.WriteString("]")
	case e.MaxArgs > e.MinArgs:
		prefix := ", "
		if e.MinArgs == 0 {
			prefix = ""
		}
		out.WriteString(fmt.Sprintf("%sarg%d..arg%d", prefix, e.MinArgs+1, e.MaxArgs))
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
