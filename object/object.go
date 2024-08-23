package object

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"grol.io/grol/ast"
	"grol.io/grol/token"
)

type Type uint8

type Object interface {
	Type() Type
	Inspect() string
	// ForceStringMapKeys makes maps[string]any instead of map[any]any irrespective of the key type.
	// This is used for go marshaler based JSON for instance.
	Unwrap(forceStringMapKeys bool) any
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
	case ARRAY:
		if sa, ok := o.(SmallArray); ok {
			for _, el := range sa.smallArr[:sa.len] {
				if !Hashable(el) {
					return false
				}
			}
			return true
		}
	case MAP:
		if sm, ok := o.(SmallMap); ok {
			for _, kv := range sm.smallKV[:sm.len] {
				if !Hashable(kv.Key) || !Hashable(kv.Value) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func UnwrapHashable(o Object) any {
	switch o.Type() {
	case INTEGER, FLOAT, BOOLEAN, NIL, ERROR, STRING:
		return o.Unwrap(false)
	default:
		return o.Inspect()
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
		m1 := ei.(Map)
		m2 := ej.(Map)
		if m1.Len() < m2.Len() {
			return -1
		}
		if m1.Len() > m2.Len() {
			return 1
		}
		m2Els := m2.mapElements()
		for i, kv := range m1.mapElements() {
			if c := Cmp(kv.Key, m2Els[i].Key); c != 0 {
				return c
			}
			if c := Cmp(kv.Value, m2Els[i].Value); c != 0 {
				return c
			}
		}
		return 0
	case ARRAY:
		a1 := ei.(Array)
		a2 := ej.(Array)
		if a1.Len() < a2.Len() {
			return -1
		}
		if a1.Len() > a2.Len() {
			return 1
		}
		a2Els := a2.Elements()
		for i, l := range a1.Elements() {
			if c := Cmp(l, a2Els[i]); c != 0 {
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

func CompareKeys(a, b keyValuePair) int {
	return Cmp(a.Key, b.Key)
}

func (m *BigMap) Get(key Object) (Object, bool) {
	kv := keyValuePair{Key: key}
	i, ok := slices.BinarySearchFunc(m.kv, kv, CompareKeys) // log(n) search as we keep it sorted.
	if !ok {
		return NULL, false
	}
	return m.kv[i].Value, true
}

func (m *BigMap) Set(key, value Object) Map {
	kv := keyValuePair{Key: key, Value: value}
	i, ok := slices.BinarySearchFunc(m.kv, kv, CompareKeys)
	if ok {
		m.kv[i].Value = value
		return m
	}
	m.kv = slices.Insert(m.kv, i, kv)
	return m
}

func (m *BigMap) Len() int {
	return len(m.kv)
}

var (
	KeyKey   = String{Value: "key"}
	ValueKey = String{Value: "value"}
)

func makeFirst(kv keyValuePair) Map {
	return MakeQuad(KeyKey, kv.Key, ValueKey, kv.Value)
}

// Make a (small) map with a single key value pair entry.
func MakePair(key, value Object) Map {
	r := SmallMap{len: 1}
	r.smallKV[0] = keyValuePair{Key: key, Value: value}
	return r
}

// Makes a map with 2 key value pairs. Make sure the keys are sorted before calling this.
// otherwise just use NewMap() + Set() twice.
func MakeQuad(key1, value1, key2, value2 Object) Map {
	r := SmallMap{len: 2}
	r.smallKV[0] = keyValuePair{Key: key1, Value: value1}
	r.smallKV[1] = keyValuePair{Key: key2, Value: value2}
	return r
}

func (m *BigMap) First() Object {
	if len(m.kv) == 0 {
		return NULL
	}
	return makeFirst(m.kv[0])
}

func (m *BigMap) Rest() Object {
	if len(m.kv) <= 1 {
		return NULL
	}
	nl := len(m.kv) - 1
	if nl > MaxSmallMap {
		return &BigMap{kv: m.kv[1:]}
	}
	res := SmallMap{len: nl}
	copy(res.smallKV[:nl], m.kv[1:])
	return res
}

func (m *BigMap) Range(l, r int64) Object {
	nl := r - l
	if nl > MaxSmallMap {
		return &BigMap{kv: m.kv[l:r]}
	}
	res := SmallMap{len: int(nl)}
	copy(res.smallKV[:nl], m.kv[l:r])
	return res
}

func NewMapSize(size int) Map {
	if size <= MaxSmallMap {
		return SmallMap{}
	}
	return &BigMap{kv: make([]keyValuePair, 0, size)}
}

func (m SmallMap) Get(key Object) (Object, bool) {
	for i := range m.len {
		if Cmp(m.smallKV[i].Key, key) == 0 {
			return m.smallKV[i].Value, true
		}
	}
	return NULL, false
}

func (m SmallMap) Set(key, value Object) Map {
	kv := keyValuePair{Key: key, Value: value}
	i, ok := slices.BinarySearchFunc(m.smallKV[:m.len], kv, CompareKeys)
	if ok {
		m.smallKV[i].Value = value
		return m
	}
	r := slices.Insert(m.smallKV[0:m.len:MaxSmallMap], i, kv)
	if m.len < MaxSmallMap {
		copy(m.smallKV[0:MaxSmallMap], r)
		m.len++
		return m
	}
	// We need to switch to a real map.
	res := &BigMap{kv: r}
	return res
}

func (m SmallMap) Len() int {
	return m.len
}

func (m SmallMap) First() Object {
	if m.len == 0 {
		return NULL
	}
	return makeFirst(m.smallKV[0])
}

func (m SmallMap) Rest() Object {
	if m.len <= 1 {
		return NULL
	}
	res := SmallMap{len: m.len - 1}
	copy(res.smallKV[:m.len-1], m.smallKV[1:m.len])
	return res
}

func (m SmallMap) Range(l, r int64) Object {
	res := SmallMap{len: int(r - l)}
	copy(res.smallKV[:], m.smallKV[l:r])
	return res
}

func (m SmallMap) Append(right Map) Map {
	if right.Len() <= MaxSmallMap { // Maybe same keys, try to keep it a SmallMap
		res := SmallMap{len: m.len}
		var ires Map = &res
		copy(res.smallKV[:m.len], m.smallKV[:m.len])
		for _, kv := range right.mapElements() {
			ires = ires.Set(kv.Key, kv.Value)
		}
		return ires
	}
	// allocate for case of all unique keys.
	nl := m.len + right.Len()
	MustBeOk(2 * nl) // KV is 2 Objects.
	res := &BigMap{kv: make([]keyValuePair, 0, nl)}
	res.kv = append(res.kv, m.smallKV[:m.len]...)
	for _, kv := range right.mapElements() {
		res.Set(kv.Key, kv.Value)
	}
	return res
}

// Creates a new Map appending the right map to the left map.
func (m *BigMap) Append(right Map) Map {
	// allocate for case of all unique keys.
	nl := len(m.kv) + right.Len()
	MustBeOk(2 * nl) // KV is 2 Objects.
	res := &BigMap{kv: make([]keyValuePair, 0, nl)}
	res.kv = append(res.kv, m.kv...)
	for _, kv := range right.mapElements() {
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

func (i Integer) Unwrap(_ bool) any {
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

func (f Float) Unwrap(_ bool) any {
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

func (b Boolean) Unwrap(_ bool) any {
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

func (s String) Unwrap(_ bool) any {
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
func (n Null) Unwrap(_ bool) any { return nil }
func (n Null) Type() Type        { return NIL }
func (n Null) Inspect() string   { return "nil" }

type Error struct {
	Value string // message
}

func (e Error) JSON(w io.Writer) error {
	_, err := fmt.Fprintf(w, `{"err":%q}`, e.Value)
	return err
}
func (e Error) Unwrap(_ bool) any { return e }
func (e Error) Error() string     { return e.Value }
func (e Error) Type() Type        { return ERROR }
func (e Error) Inspect() string   { return "<err: " + e.Value + ">" }

type ReturnValue struct {
	Value Object
}

func (rv ReturnValue) JSON(w io.Writer) error {
	_, _ = w.Write([]byte(`{"return":`))
	err := rv.Value.JSON(w)
	_, _ = w.Write([]byte("}"))
	return err
}
func (rv ReturnValue) Unwrap(_ bool) any { return rv.Value }
func (rv ReturnValue) Type() Type        { return RETURN }
func (rv ReturnValue) Inspect() string   { return rv.Value.Inspect() }

type Function struct {
	Parameters []ast.Node
	Name       *ast.Identifier
	CacheKey   string
	Body       *ast.Statements
	Env        *Environment
	Variadic   bool
	Lambda     bool // i.e. has no name.
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

func (f Function) Unwrap(_ bool) any { return f }
func (f Function) Type() Type        { return FUNC }

// Must be called after the function is fully initialized.
// Whether a function result should be cached doesn't depend on the Name,
// so it's not part of the cache key.
func (f *Function) SetCacheKey() string {
	out := strings.Builder{}
	if !f.Lambda {
		out.WriteString("func ")
	}
	f.CacheKey = f.finishFuncOutput(&out)
	return f.CacheKey
}

func (f *Function) lambdaPrint(ps *ast.PrintState, out *strings.Builder) string {
	if len(f.Parameters) != 1 {
		out.WriteString(")=>")
	} else {
		out.WriteString("=>")
	}
	needBraces := len(f.Body.Statements) != 1 || f.Body.Statements[0].Value().Type() == token.LBRACE
	if needBraces {
		out.WriteString("{")
	}
	f.Body.PrettyPrint(ps)
	if needBraces {
		out.WriteString("}")
	}
	return out.String()
}

// Common part of Inspect and SetCacheKey. Outputs the rest of the function.
func (f *Function) finishFuncOutput(out *strings.Builder) string {
	needParen := !f.Lambda || len(f.Parameters) != 1
	if needParen {
		out.WriteString("(")
	}
	ps := &ast.PrintState{Out: out, Compact: true}
	ps.ComaList(f.Parameters)
	if f.Lambda {
		return f.lambdaPrint(ps, out)
	}
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

const MaxSmallArray = 8

type SmallArray struct {
	smallArr [MaxSmallArray]Object
	len      int
}

func (sa SmallArray) Type() Type { return ARRAY }
func (sa SmallArray) Unwrap(forceStringKeys bool) any {
	return Unwrap(sa.smallArr[:sa.len], forceStringKeys)
}
func (sa SmallArray) JSON(w io.Writer) error { return BigArray{elements: sa.smallArr[:sa.len]}.JSON(w) }
func (sa SmallArray) Inspect() string {
	if sa.len == 0 {
		return "[]"
	}
	out := strings.Builder{}
	WriteStrings(&out, sa.smallArr[:sa.len], "[", ",", "]")
	return out.String()
}

var EmptyArray = SmallArray{}

func NewArray(elements []Object) Object {
	if len(elements) == 0 {
		return EmptyArray
	}
	if len(elements) <= MaxSmallArray {
		sa := SmallArray{len: len(elements)}
		copy(sa.smallArr[:], elements)
		return sa
	}
	return BigArray{elements: elements}
}

func Len(a Object) int {
	switch a := a.(type) {
	case SmallArray:
		return a.len
	case BigArray:
		return len(a.elements)
	case Map:
		return a.Len()
	case String:
		return len(a.Value)
	case Null:
		return 0
	}
	return -1
}

func First(a Object) Object {
	switch a := a.(type) {
	case Null:
		return NULL
	case SmallArray:
		if a.len == 0 {
			return NULL
		}
		return a.smallArr[0]
	case BigArray:
		if len(a.elements) == 0 {
			return NULL
		}
		return a.elements[0]
	case Map:
		return a.First()
	case String:
		if a.Value == "" {
			return NULL
		}
		// first rune of str
		return String{Value: string([]rune(a.Value)[:1])}
	}
	return Error{Value: "first() not supported on " + a.Type().String()}
}

func Rest(val Object) Object {
	switch v := val.(type) {
	case Null:
		return NULL
	case String:
		if len(v.Value) <= 1 {
			return NULL
		}
		// rest of the string
		return String{Value: string([]rune(v.Value)[1:])}
	case SmallArray:
		if v.len <= 1 {
			return NULL
		}
		return NewArray(v.smallArr[1:v.len])
	case BigArray:
		if len(v.elements) <= 1 {
			return NULL
		}
		return NewArray(v.elements[1:])
	case *BigMap:
		return v.Rest()
	case SmallMap:
		return v.Rest()
	}
	return Error{Value: "rest() not supported on " + val.Type().String()}
}

func Range(val Object, l, r int64) Object {
	switch v := val.(type) {
	case SmallArray:
		if l < 0 || r > int64(v.len) {
			return Error{Value: "range() out of bounds"}
		}
		return NewArray(v.smallArr[l:r])
	case BigArray:
		if l < 0 || r > int64(len(v.elements)) {
			return Error{Value: "range() out of bounds"}
		}
		return NewArray(v.elements[l:r])
	case String:
		rs := []rune(v.Value)
		if l < 0 || r > int64(len(rs)) {
			return Error{Value: "range() out of bounds"}
		}
		return String{Value: string(rs[l:r])}
	case *BigMap:
		return v.Range(l, r)
	case SmallMap:
		return v.Range(l, r)
	}
	return Error{Value: "range() not supported on " + val.Type().String()}
}

func (ao BigArray) First() Object        { return First(ao) }
func (ao BigArray) Rest() Object         { return Rest(ao) }
func (ao BigArray) Elements() []Object   { return ao.elements }
func (ao BigArray) Len() int             { return len(ao.elements) }
func (sa SmallArray) First() Object      { return First(sa) }
func (sa SmallArray) Rest() Object       { return Rest(sa) }
func (sa SmallArray) Elements() []Object { return sa.smallArr[:sa.len] }
func (sa SmallArray) Len() int           { return sa.len }

// Elements returns the keys of a map or the elements of an array.
func Elements(val Object) []Object {
	switch v := val.(type) {
	case Array:
		return v.Elements()
	case SmallMap:
		res := make([]Object, v.len)
		for i, kv := range v.smallKV[:v.len] {
			res[i] = kv.Key
		}
		return res
	case *BigMap:
		res := make([]Object, len(v.kv))
		for i, kv := range v.kv {
			res[i] = kv.Key
		}
		return res
	}
	return nil
}

type Array interface {
	Object
	Len() int
	First() Object
	Rest() Object
	Elements() []Object
}

type BigArray struct {
	elements []Object
}

func (ao BigArray) Unwrap(forceStringKeys bool) any { return Unwrap(ao.elements, forceStringKeys) }
func (ao BigArray) Type() Type                      { return ARRAY }
func (ao BigArray) Inspect() string {
	out := strings.Builder{}
	WriteStrings(&out, ao.elements, "[", ",", "]")
	return out.String()
}

func (ao BigArray) JSON(w io.Writer) error {
	_, _ = w.Write([]byte("["))
	for i, p := range ao.elements {
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
type keyValuePair struct {
	Key   Object
	Value Object
}

const MaxSmallMap = 4 // so 2x, so same as MaxSmallArray really.

type SmallMap struct {
	smallKV [MaxSmallMap]keyValuePair
	len     int
}

// Sorted KV pairs, O(n) insert O(log n) access/same key mutations.
type BigMap struct {
	kv []keyValuePair
}

type Map interface {
	Object
	Get(key Object) (Object, bool)
	Set(key, value Object) Map
	Len() int
	First() Object
	Rest() Object
	mapElements() []keyValuePair
	Append(right Map) Map
}

func NewMap() Map {
	return SmallMap{}
}

func areIntFloat(a, b Type) bool {
	l := min(a, b)
	h := max(a, b)
	return l == INTEGER && h == FLOAT
}

func (ao BigArray) Less(i, j int) bool {
	return Cmp(ao.elements[i], ao.elements[j]) < 0
}

func (ao BigArray) Swap(i, j int) {
	ao.elements[i], ao.elements[j] = ao.elements[j], ao.elements[i]
}

func (m *BigMap) Unwrap(forceStringKeys bool) any {
	if res, ok := UnwrapStringKeys(m, forceStringKeys); ok {
		return res
	}
	res := make(map[any]any, len(m.kv))
	for _, kv := range m.kv {
		res[UnwrapHashable(kv.Key)] = kv.Value.Unwrap(forceStringKeys)
	}
	return res
}

func (m *BigMap) mapElements() []keyValuePair {
	return m.kv
}

func (m SmallMap) mapElements() []keyValuePair {
	return m.smallKV[:m.len]
}

func UnwrapStringKeys(m Map, forceStringKeys bool) (map[string]any, bool) {
	res := make(map[string]any, m.Len())
	for _, kv := range m.mapElements() {
		s, ok := kv.Key.(String)
		if !ok {
			if forceStringKeys {
				res[kv.Key.Inspect()] = kv.Value.Unwrap(true)
				continue
			}
			return nil, false
		}
		res[s.Value] = kv.Value.Unwrap(forceStringKeys)
	}
	return res, true
}

func (m SmallMap) Unwrap(forceStringKeys bool) any {
	if res, ok := UnwrapStringKeys(m, forceStringKeys); ok {
		return res
	}
	res := make(map[any]any, m.len)
	for _, kv := range m.smallKV[:m.len] {
		res[UnwrapHashable(kv.Key)] = kv.Value.Unwrap(forceStringKeys)
	}
	return res
}

func (m SmallMap) Type() Type { return MAP }
func (m *BigMap) Type() Type  { return MAP }

func (m SmallMap) Inspect() string {
	if m.len == 0 {
		return "{}"
	}
	out := strings.Builder{}
	out.WriteString("{")
	for i := range m.len {
		if i > 0 {
			out.WriteString(",")
		}
		out.WriteString(m.smallKV[i].Key.Inspect())
		out.WriteString(":")
		out.WriteString(m.smallKV[i].Value.Inspect())
	}
	out.WriteString("}")
	return out.String()
}

func (m *BigMap) Inspect() string {
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

func (m SmallMap) JSON(w io.Writer) error {
	if m.len == 0 {
		_, err := w.Write([]byte("{}"))
		return err
	}
	return (&BigMap{kv: m.smallKV[:m.len]}).JSON(w)
}

func (m *BigMap) JSON(w io.Writer) error {
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

func (q Quote) Unwrap(_ bool) any { return q.Node }
func (q Quote) Type() Type        { return QUOTE }
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

func (m Macro) Unwrap(_ bool) any { return m }
func (m Macro) Type() Type        { return MACRO }
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
	Variadic bool        // MaxArgs > MinArgs (or MaxArg == -1)
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
	prefix := ", "
	if e.MinArgs == 0 {
		prefix = ""
	}
	switch {
	case e.MaxArgs < 0:
		out.WriteString(", ..")
	case e.MaxArgs == e.MinArgs+1: // only 1 extra optional argument.
		arg := "arg"
		if len(e.ArgTypes) > e.MinArgs {
			arg = strings.ToLower(e.ArgTypes[e.MinArgs].String())
		}
		out.WriteString(prefix)
		out.WriteString("[") // to indicate optional
		out.WriteString(arg)
		out.WriteString("]")
	case e.MaxArgs > e.MinArgs:
		out.WriteString(fmt.Sprintf("%sarg%d..arg%d", prefix, e.MinArgs+1, e.MaxArgs))
	}
}

func (e Extension) Unwrap(_ bool) any { return e }
func (e Extension) Type() Type        { return EXTENSION }
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
