package eval

import (
	"bytes"
	"io"
	"math"
	"strings"

	"fortio.org/log"
	"grol.io/grol/ast"
	"grol.io/grol/object"
	"grol.io/grol/token"
)

// Eval implementation.

// See todo in token about publishing all of them.
var unquoteToken = token.ByType(token.UNQUOTE)

func (s *State) compoundAssignNested(node ast.Node, operator token.Type, value object.Object) (object.Object, *object.Error) {
	n, ok := node.(*ast.IndexExpression)
	if !ok {
		err := s.NewError("assignment to non identifier: " + node.Value().DebugString())
		return err, &err
	}

	// Recursively assign into the left, then update this level
	left := n.Left
	// Evaluate the left side (could be another IndexExpression or Identifier)
	var base object.Object
	var identifier string
	if id, ok := left.(*ast.Identifier); ok {
		identifier = id.Literal()
		baseObj, ok := s.env.Get(identifier)
		if !ok {
			err := s.NewError("identifier not found: " + identifier)
			return err, &err
		}
		base = object.Value(baseObj)
	} else {
		base = s.Eval(left)
		if base.Type() == object.ERROR {
			err := base.(object.Error)
			return base, &err
		}
	}
	// Compute the index
	var index object.Object
	if n.Token.Type() == token.DOT {
		index = object.String{Value: n.Index.Value().Literal()}
	} else {
		index = s.Eval(n.Index)
		if index.Type() == object.ERROR {
			err := index.(object.Error)
			return index, &err
		}
	}
	if value.Type() == object.BOOLEAN {
		if operator == token.BITAND {
			operator = token.AND
		}
		if operator == token.BITOR {
			operator = token.OR
		}
	}
	// Set the value at this level
	indexValue := s.evalIndexExpression(base, n)
	// evaluate result of operator
	compounded := s.evalInfixExpression(operator, value, indexValue)
	newBase := s.evalIndexAssignmentValue(base, index, compounded, identifier)
	log.LogVf("%s new base", newBase)
	if newBase.Type() == object.ERROR {
		err := newBase.(object.Error)
		return newBase, &err
	}
	// assign the updated base to the parent
	res, err := s.assignNested(left, newBase)
	if err != nil {
		return res, err
	}
	return compounded, err

}

// Helper to recursively assign into nested structures and propagate the change up to the top-level identifier.
func (s *State) assignNested(node ast.Node, value object.Object) (object.Object, *object.Error) {
	switch n := node.(type) {
	case *ast.Identifier:
		// Top-level variable, update env
		oerr := s.env.Set(n.Literal(), value)
		if oerr.Type() == object.ERROR {
			err := oerr.(object.Error)
			return oerr, &err
		}
		return value, nil
	case *ast.IndexExpression:
		// Recursively assign into the left, then update this level
		left := n.Left
		// Evaluate the left side (could be another IndexExpression or Identifier)
		var base object.Object
		var identifier string
		if id, ok := left.(*ast.Identifier); ok {
			identifier = id.Literal()
			baseObj, ok := s.env.Get(identifier)
			if !ok {
				err := s.NewError("identifier not found: " + identifier)
				return err, &err
			}
			base = object.Value(baseObj)
		} else {
			base = s.Eval(left)
			if base.Type() == object.ERROR {
				err := base.(object.Error)
				return base, &err
			}
		}
		// Compute the index
		var index object.Object
		if n.Token.Type() == token.DOT {
			index = object.String{Value: n.Index.Value().Literal()}
		} else {
			index = s.Eval(n.Index)
			if index.Type() == object.ERROR {
				err := index.(object.Error)
				return index, &err
			}
		}
		// Set the value at this level
		newBase := s.evalIndexAssignmentValue(base, index, value, identifier)
		if newBase.Type() == object.ERROR {
			err := newBase.(object.Error)
			return newBase, &err
		}
		// Recursively assign the updated base to the parent
		return s.assignNested(left, newBase)
	default:
		err := s.NewError("assignment to non identifier: " + node.Value().DebugString())
		return err, &err
	}
}

func (s *State) evalAssignment(right object.Object, node *ast.InfixExpression) object.Object {
	if rt := right.Type(); rt == object.ERROR {
		log.Warnf("Not assigning %q", right.Inspect())
		return right
	}
	switch node.Left.Value().Type() {
	case token.DOT, token.LBRACKET:
		nodeType := node.Type()
		if isCompound(nodeType) {
			opToEval := nodeType - (token.SUMASSIGN - token.PLUS)
			res, _ := s.compoundAssignNested(node.Left, opToEval, right) // un
			log.LogVf("%s - right", right)
			return res
		}
		// Use the recursive assignNested helper
		res, oerr := s.assignNested(node.Left, right)
		if oerr != nil {
			return res
		}
		return right
	case token.IDENT:
		id := node.Left.(*ast.Identifier)
		name := id.Literal()
		nodeType := node.Type()
		if isCompound(nodeType) {
			opToEval := nodeType - (token.SUMASSIGN - token.PLUS)
			value := s.evalIdentifier(id)

			if value.Type() == object.BOOLEAN {
				if opToEval == token.BITAND {
					opToEval = token.AND
				}
				if opToEval == token.BITOR {
					opToEval = token.OR
				}
			}
			compounded := s.evalInfixExpression(opToEval, value, right)
			return s.env.CreateOrSet(name, compounded, false)
		}
		log.LogVf("eval assign %#v to %s", right, name)
		// Propagate possible error (constant, extension names setting).
		// Distinguish between define and assign, define (:=) forces a new variable.
		return s.env.CreateOrSet(name, right, nodeType == token.DEFINE)
	case token.REGISTER:
		reg := node.Left.(*object.Register)
		log.LogVf("eval assign %#v to register %d", right, reg.Idx)
		intVal, ok := Int64Value(right)
		if !ok {
			return s.NewError("register assignment of non integer: " + right.Inspect())
		}
		*reg.Ptr() = intVal
		return right
	default:
		return s.NewError("assignment to non identifier: " + node.Left.Value().DebugString())
	}
}

func (s *State) evalIndexAssignmentValue(base, index, value object.Object, identifier string) object.Object {
	switch base.Type() {
	case object.ARRAY:
		idx, ok := Int64Value(index)
		if !ok {
			return s.NewError("index assignment to array with non integer index: " + index.Inspect())
		}
		if idx < 0 {
			idx = int64(object.Len(base)) + idx
		}
		if idx < 0 || idx >= int64(object.Len(base)) {
			return s.NewError("index assignment out of bounds: " + index.Inspect())
		}
		elements := object.Elements(base)
		elements[idx] = value
		return object.NewArray(elements)
	case object.MAP:
		m := base.(object.Map)
		return m.Set(index, value)
	default:
		if identifier != "" {
			return s.Errorf("index assignment to %s of unexpected type %s (%s)", identifier, base.Type().String(), base.Inspect())
		}
		return s.Errorf("index assignment to object of unexpected type %s (%s)", base.Type().String(), base.Inspect())
	}
}

func argCheck[T any](s *State, msg string, n int, vararg bool, args []T) *object.Error {
	if vararg {
		if len(args) < n {
			e := s.Errorf("%s: wrong number of arguments. got=%d, want at least %d", msg, len(args), n)
			return &e
		}
		return nil
	}
	if len(args) != n {
		e := s.Errorf("%s: wrong number of arguments. got=%d, want=%d", msg, len(args), n)
		return &e
	}
	return nil
}

func (s *State) evalPrefixIncrDecr(operator token.Type, node ast.Node) object.Object {
	if log.LogVerbose() {
		log.LogVf("eval prefix %s", ast.DebugString(node))
	}
	nv := node.Value()
	if nv.Type() != token.IDENT {
		return s.NewError("can't prefix increment/decrement " + nv.DebugString())
	}
	id := nv.Literal()
	val, ok := s.env.Get(id)
	if !ok {
		return s.NewError("identifier not found: " + id)
	}
	val = object.Value(val) // deref.
	toAdd := int64(1)
	if operator == token.DECR {
		toAdd = -1
	}
	switch val := val.(type) {
	case object.Integer:
		return s.env.Set(id, object.Integer{Value: val.Value + toAdd})
	case object.Float:
		return s.env.Set(id, object.Float{Value: val.Value + float64(toAdd)}) // So PI++ fails not silently.
	default:
		return s.NewError("can't prefix increment/decrement " + val.Type().String())
	}
}

func (s *State) evalPostfixExpression(node *ast.PostfixExpression) object.Object {
	if log.LogVerbose() {
		log.LogVf("eval postfix %s", node.DebugString())
	}
	id := node.Prev.Literal()
	val, ok := s.env.Get(id)
	if !ok {
		return s.NewError("identifier not found: " + id)
	}
	val = object.Value(val) // deref.
	var toAdd int64
	switch node.Type() {
	case token.INCR:
		toAdd = 1
	case token.DECR:
		toAdd = -1
	default:
		return s.NewError("unknown postfix operator: " + node.Type().String())
	}
	var oerr object.Object
	switch val := val.(type) {
	case object.Integer:
		oerr = s.env.Set(id, object.Integer{Value: val.Value + toAdd})
	case object.Float:
		oerr = s.env.Set(id, object.Float{Value: val.Value + float64(toAdd)}) // So PI++ fails not silently.
	default:
		return s.NewError("can't postfix increment/decrement " + val.Type().String())
	}
	if oerr.Type() == object.ERROR {
		return oerr
	}
	return val
}

// Doesn't unwrap return - return bubbles up.
// Initially this was the one to use internally recursively, except for when evaluating a function
// but now it's less clear because of the need to unwrap references too. TODO: fix/clarify.
func (s *State) evalInternal(node any) object.Object { //nolint:funlen,gocognit,gocyclo // quite a lot of cases.
	if s.Context != nil && s.Context.Err() != nil {
		return s.Error(s.Context.Err())
	}
	switch node := node.(type) {
	case *object.Register:
		// somehow returning unwrapped node as is for Eval to unwrap is more expensive (escape analysis issue?)
		return node
	// Statements
	case *ast.Statements:
		if node == nil { // TODO: only here? this comes from empty else branches.
			return object.NULL
		}
		log.LogVf("eval program")
		return s.evalStatements(node.Statements)
	case *ast.IfExpression:
		return s.evalIfExpression(node)
	case *ast.ForExpression:
		return s.evalForExpression(node)
		// Expressions
	case *ast.Identifier:
		return s.evalIdentifier(node)
	case *ast.PrefixExpression:
		if log.LogVerbose() {
			log.LogVf("eval prefix %s", node.DebugString())
		}
		switch node.Type() {
		case token.INCR, token.DECR:
			return s.evalPrefixIncrDecr(node.Type(), node.Right)
		default:
			right := s.Eval(node.Right)
			if right.Type() == object.ERROR {
				return right
			}
			return s.evalPrefixExpression(node.Type(), right)
		}
	case *ast.PostfixExpression:
		return s.evalPostfixExpression(node)
	case *ast.InfixExpression:
		if log.LogVerbose() { // DebugString() is expensive/shows up in profiles significantly otherwise (before the ifs).
			log.LogVf("eval infix %s", node.DebugString())
		}
		// Eval and not evalInternal because we need to unwrap "return".
		if isAssignment(node.Type()) {
			return s.evalAssignment(s.Eval(node.Right), node)
		}
		// Humans expect left to right evaluations.
		left := s.Eval(node.Left)
		if left.Type() == object.ERROR {
			return left
		}
		// Short circuiting for AND and OR:
		if node.Token.Type() == token.AND && left == object.FALSE {
			return object.FALSE
		}
		if node.Token.Type() == token.OR && left == object.TRUE {
			return object.TRUE
		}
		// Pipe operator, for now only for string | call expressions:
		if node.Token.Type() == token.BITOR && left.Type() == object.STRING && node.Right.Value().Type() == token.LPAREN {
			return s.evalPipe(left, node.Right)
		}
		right := s.Eval(node.Right)
		if right.Type() == object.ERROR {
			return right
		}
		return s.evalInfixExpression(node.Type(), left, right)

	case *ast.IntegerLiteral:
		return object.Integer{Value: node.Val}
	case *ast.FloatLiteral:
		return object.Float{Value: node.Val}
	case *ast.Boolean:
		return object.NativeBoolToBooleanObject(node.Val)
	case *ast.StringLiteral:
		return object.String{Value: node.Literal()}

	case *ast.ControlExpression:
		return object.ReturnValue{Value: object.NULL, ControlType: node.Type()}
	case *ast.ReturnStatement:
		if node.ReturnValue == nil {
			return object.ReturnValue{Value: object.NULL, ControlType: token.RETURN}
		}
		val := s.evalInternal(node.ReturnValue)
		return object.ReturnValue{Value: val, ControlType: token.RETURN}
	case *ast.Builtin:
		return s.evalBuiltin(node)
	case *ast.FunctionLiteral:
		name := node.Name
		fn := object.Function{
			Parameters: node.Parameters,
			Name:       name,
			Env:        s.env,
			Body:       node.Body,
			Variadic:   node.Variadic,
			Lambda:     node.IsLambda,
		}
		if !fn.Lambda && fn.Name == nil {
			log.LogVf("Normalizing non-short lambda form to => lambda")
			fn.Lambda = true
		}
		object.SetCacheKey(&fn) // sets cache key
		if name != nil {
			oerr := s.env.Set(name.Literal(), fn)
			if oerr.Type() == object.ERROR {
				return oerr // propagate that func FOO() { ... } can only be defined once.
			}
		}
		return fn
	case *ast.CallExpression:
		f := s.Eval(node.Function)
		if f.Type() == object.ERROR {
			return f
		}
		args, oerr := s.evalExpressions(node.Arguments)
		if oerr != nil {
			return *oerr
		}
		if f.Type() == object.EXTENSION {
			return s.applyExtension(f.(object.Extension), args)
		}
		name := node.Function.Value().Literal()
		return s.applyFunction(name, f, args)
	case *ast.ArrayLiteral:
		elements, oerr := s.evalExpressions(node.Elements)
		if oerr != nil {
			return *oerr
		}
		return object.NewArray(elements)
	case *ast.MapLiteral:
		return s.evalMapLiteral(node)
	case *ast.IndexExpression:
		if node.Value().Type() == token.DOT {
			// See commits in PR#217 for a version using double map lookup, trading off the string concat (alloc)
			// for a map lookup. code is a lot simpler without actual ns map though so we stick to this version
			// for now.
			extName := node.Left.Value().Literal() + "." + node.Index.Value().Literal()
			if ext, ok := s.Extensions[extName]; ok {
				return ext
			}
		}
		return s.evalIndexExpression(s.Eval(node.Left), node)
	case *ast.Comment:
		return object.NULL
	}
	return s.Errorf("unknown node type: %T", node)
}

func (s *State) evalPipe(left object.Object, right ast.Node) object.Object {
	s.PipeVal = []byte(left.(object.String).Value)
	res := s.evalInternal(right)
	s.PipeVal = nil
	return res
}

func (s *State) evalIndexExpression(left object.Object, node *ast.IndexExpression) object.Object {
	if left.Type() == object.ERROR {
		return left
	}
	var index object.Object
	if node.Token.Type() == token.DOT {
		// index is the string value and not an identifier to resolve.
		key := node.Index.Value()
		if key.Type() != token.STRING && key.Type() != token.IDENT {
			return s.Errorf("index expression with . not string: %s", key.Literal())
		}
		return s.evalIndexExpressionIdx(left, object.String{Value: key.Literal()})
	}
	if node.Index.Value().Type() == token.COLON {
		rangeExp := node.Index.(*ast.InfixExpression)
		return s.evalIndexRangeExpression(left, rangeExp.Left, rangeExp.Right)
	}
	index = s.Eval(node.Index)
	if index.Type() == object.ERROR {
		return index
	}
	return s.evalIndexExpressionIdx(left, index)
}

func (s *State) evalMapLiteral(node *ast.MapLiteral) object.Object {
	result := object.NewMapSize(len(node.Order))

	for _, keyNode := range node.Order {
		valueNode := node.Pairs[keyNode]
		key := s.Eval(keyNode)
		if !object.Equals(key, key) {
			log.Warnf("key %s is not hashable", key.Inspect())
			return s.NewError("key " + key.Inspect() + " is not hashable")
		}
		value := s.Eval(valueNode)
		result = result.Set(key, value)
	}
	return result
}

func (s *State) evalPrintLogError(node *ast.Builtin) object.Object {
	doLog := (node.Type() == token.LOG)
	if doLog && (log.GetLogLevel() >= log.Error) {
		return object.NULL
	}
	buf := strings.Builder{}
	for i, v := range node.Parameters {
		if i > 0 {
			buf.WriteString(" ")
		}
		r := s.evalInternal(v)
		// If what we print/println is an error, return it instead. log can log errors.
		if r.Type() == object.ERROR && !doLog {
			return r
		}
		if isString := r.Type() == object.STRING; isString {
			buf.WriteString(r.(object.String).Value)
		} else {
			buf.WriteString(r.Inspect())
		}
	}
	if node.Type() == token.ERROR {
		return s.NewError(buf.String())
	}
	if (s.NoLog && doLog) || node.Type() == token.PRINTLN {
		buf.WriteRune('\n') // log() has a implicit newline when using log.Xxx, print() doesn't, println() does.
	}
	if doLog && !s.NoLog {
		// Consider passing the arguments to log instead of making a string concatenation.
		log.Printf("%s", buf.String())
	} else {
		where := s.Out
		if doLog {
			where = s.LogOut
		}
		_, err := where.Write([]byte(buf.String()))
		if err != nil {
			log.Warnf("print: %v", err)
		}
	}
	return object.NULL
}

var ErrorKey = object.String{Value: "err"} // can't use error as that's a builtin.

func (s *State) evalDelete(node ast.Node) object.Object {
	s.env.TriggerNoCache()
	switch node.Value().Type() {
	case token.IDENT:
		name := node.Value().Literal()
		if object.Constant(name) {
			return s.NewError("delete constant")
		}
		return s.env.Delete(name)
	case token.DOT:
		idxE := node.(*ast.IndexExpression)
		// index is the string value and not an identifier to resolve.
		key := idxE.Index.Value()
		if key.Type() != token.STRING && key.Type() != token.IDENT {
			return s.Errorf("del expression with . not a string: %s", key.Literal())
		}
		index := object.String{Value: key.Literal()}
		return s.deleteMapEntry(idxE, index)
	case token.LBRACKET:
		// Map/array [] index
		idxE := node.(*ast.IndexExpression)
		index := s.Eval(idxE.Index)
		if index.Type() == object.ERROR {
			return index
		}
		return s.deleteMapEntry(idxE, index)
	default:
		return s.NewError("delete not supported on " + node.Value().Type().String())
	}
}

func (s *State) deleteMapEntry(idxE *ast.IndexExpression, index object.Object) object.Object {
	if idxE.Left.Value().Type() != token.IDENT {
		return s.NewError("delete index on non identifier: " + idxE.Left.Value().DebugString())
	}
	id := idxE.Left.Value().Literal()
	obj, ok := s.env.Get(id)
	if !ok {
		// Nothing to delete, we're done
		return object.FALSE
	}
	// Handle references to maps
	obj = object.Value(obj)
	// TODO: handle arrays too? though delete arr[idx] == arr[0:idx]+arr[idx+1:] so... no point
	if obj.Type() != object.MAP {
		return s.NewError("delete index on non map: " + id + " " + obj.Type().String())
	}
	log.LogVf("remove map: %s from %s", index.Inspect(), id)
	m := obj.(object.Map)
	m, changed := m.Delete(index)
	if !changed {
		return object.FALSE
	}
	oerr := s.env.Set(id, m)
	if oerr.Type() == object.ERROR {
		return oerr
	}
	return object.TRUE
}

func (s *State) evalBuiltin(node *ast.Builtin) object.Object {
	// all take 1 arg exactly except print and log which take 1+.
	t := node.Type()
	minV := 1
	varArg := (t == token.PRINT || t == token.LOG || t == token.ERROR)
	if t == token.PRINTLN {
		minV = 0
		varArg = true
	}
	if oerr := argCheck(s, node.Literal(), minV, varArg, node.Parameters); oerr != nil {
		return *oerr
	}
	// builtins that don't eval arguments (quote, del)
	switch t {
	case token.QUOTE:
		return s.quote(node.Parameters[0])
	case token.DEL:
		return s.evalDelete(node.Parameters[0])
	default:
	}
	var val object.Object
	var rt object.Type
	if minV > 0 {
		val = s.evalInternal(node.Parameters[0])
		rt = val.Type()
		if rt == object.ERROR && t != token.LOG && t != token.CATCH { // log can log (and thus catch) errors.
			return val
		}
	}
	switch t {
	case token.CATCH:
		isError := rt == object.ERROR
		if isError {
			val = object.String{Value: val.(object.Error).Value}
		}
		return object.MakeQuad(ErrorKey, object.NativeBoolToBooleanObject(isError), object.ValueKey, val)
	case token.ERROR, token.PRINT, token.PRINTLN, token.LOG:
		return s.evalPrintLogError(node)
	case token.FIRST:
		return object.First(val)
	case token.REST:
		return object.Rest(val)
	case token.LEN:
		l := object.Len(val)
		if l == -1 {
			return s.NewError("len: not supported on " + val.Type().String())
		}
		return object.Integer{Value: int64(l)}
	default:
		return s.Errorf("builtin %s yet implemented", node.Type())
	}
}

func (s *State) evalIndexRangeExpression(left object.Object, leftIdx, rightIdx ast.Node) object.Object {
	leftIndex := s.Eval(leftIdx)
	nilRight := (rightIdx == nil)
	var rightIndex object.Object
	if nilRight {
		if log.LogDebug() {
			log.Debugf("eval index %s[%s:]", left.Inspect(), leftIndex.Inspect())
		}
	} else {
		rightIndex = s.Eval(rightIdx)
		if log.LogDebug() {
			log.Debugf("eval index %s[%s:%s]", left.Inspect(), leftIndex.Inspect(), rightIndex.Inspect())
		}
	}
	if !object.IsIntType(leftIndex.Type()) || (!nilRight && !object.IsIntType(rightIndex.Type())) {
		return s.NewError("range index not integer")
	}
	num := object.Len(left)
	l, _ := Int64Value(leftIndex)
	if l < 0 { // negative is relative to the end.
		l = int64(num) + l
	}
	var r int64
	if nilRight {
		r = int64(num)
	} else {
		r, _ = Int64Value(rightIndex)
		if r < 0 {
			r = int64(num) + r
		}
	}
	if l > r {
		return s.NewError("range index invalid: left greater then right")
	}
	l = min(l, int64(num))
	r = min(r, int64(num))
	switch left.Type() {
	case object.STRING:
		str := left.(object.String).Value
		return object.String{Value: str[l:r]}
	case object.ARRAY:
		return object.NewArray(object.Elements(left)[l:r])
	case object.MAP:
		return object.Range(left, l, r) // could call that one for all of them...
	case object.NIL:
		return object.NULL
	default:
		return s.NewError("range index operator not supported: " + left.Type().String())
	}
}

func (s *State) evalIndexExpressionIdx(left, index object.Object) object.Object {
	var idx int64
	var isInt bool
	if index.Type() == object.NIL {
		idx = 0
		isInt = true
	} else {
		idx, isInt = Int64Value(index)
	}
	switch {
	case left.Type() == object.STRING && isInt:
		str := left.(object.String).Value
		num := len(str)
		if idx < 0 { // negative is relative to the end.
			idx = int64(num) + idx
		}
		if idx < 0 || idx >= int64(len(str)) {
			return object.NULL
		}
		return object.Integer{Value: int64(str[idx])}
	case left.Type() == object.ARRAY && isInt:
		return evalArrayIndexExpression(left, idx)
	case left.Type() == object.MAP:
		return evalMapIndexExpression(left, index)
	case left.Type() == object.NIL:
		return object.NULL
	default:
		return s.NewError("index operator not supported: " + left.Type().String() + "[" + index.Type().String() + "]")
	}
}

func evalMapIndexExpression(assoc, key object.Object) object.Object {
	m := assoc.(object.Map)
	v, ok := m.Get(key)
	if !ok {
		return object.NULL
	}
	return v // already unwrapped (index has been Eval'ed)
}

func evalArrayIndexExpression(array object.Object, idx int64) object.Object {
	maxV := int64(object.Len(array) - 1)
	if idx < 0 { // negative is relative to the end.
		idx = maxV + 1 + idx // elsewhere we use len() but here maxV is len-1
	}
	if idx < 0 || idx > maxV {
		return object.NULL
	}
	return object.Elements(array)[idx]
}

func (s *State) applyExtension(fn object.Extension, args []object.Object) object.Object {
	// TODO: consider memoizing the extension functions too? or maybe based on flags on the extension.
	l := len(args)
	if log.LogDebug() {
		log.Debugf("apply extension %s variadic %t : %d args %v", fn.Inspect(), fn.Variadic, l, args)
	}
	if fn.MaxArgs == -1 {
		// Only do this for true variadic functions (maxargs == -1)
		if l > 0 && args[l-1].Type() == object.ARRAY {
			args = append(args[:l-1], object.Elements(args[l-1])...)
			l = len(args)
			log.Debugf("expending last arg now %d args %v", l, args)
		}
	}
	if l < fn.MinArgs {
		return s.Errorf("wrong number of arguments got=%d, want %s",
			l, fn.Inspect()) // shows usage
	}
	if fn.MaxArgs != -1 && l > fn.MaxArgs {
		return s.Errorf("wrong number of arguments got=%d, want %s",
			l, fn.Inspect()) // shows usage
	}
	for i, arg := range args {
		if i >= len(fn.ArgTypes) {
			break
		}
		if fn.ArgTypes[i] == object.ANY {
			continue
		}
		// deref but only if type isn't ANY - so type() gets the REFERENCES but math functions don't/get values.
		arg = object.Value(arg)
		args[i] = arg
		// Auto promote integer to float if needed.
		if fn.ArgTypes[i] == object.FLOAT && arg.Type() == object.INTEGER {
			args[i] = object.Float{Value: float64(arg.(object.Integer).Value)}
			continue
		}
		if fn.ArgTypes[i] != arg.Type() {
			return s.Errorf("wrong type of argument got=%s, want %s",
				arg.Type(), fn.Inspect())
		}
	}
	if fn.DontCache {
		s.env.TriggerNoCache()
	}
	if fn.ClientData != nil {
		res := fn.Callback(fn.ClientData, fn.Name, args)
		if res.Type() == object.ERROR {
			// Add the stack trace to the error.
			return s.ErrorAddStack(res.(object.Error))
		}
		return res
	}
	return fn.Callback(s, fn.Name, args)
}

func (s *State) applyFunction(name string, fn object.Object, args []object.Object) object.Object {
	function, ok := fn.(object.Function)
	if !ok {
		return s.NewError("not a function: " + fn.Type().String() + ":" + fn.Inspect())
	}
	if v, output, ok := s.cache.Get(function.CacheKey, args); ok {
		log.Debugf("Cache hit for %s %v -> %#v", function.CacheKey, args, v)
		if len(output) > 0 {
			_, err := s.Out.Write(output)
			if err != nil {
				log.Warnf("output: %v", err)
			}
		}
		return v
	}
	nenv, newBody, oerr := s.extendFunctionEnv(s.env, name, function, args)
	if oerr != nil {
		return *oerr
	}
	curState := s.env
	s.env = nenv
	oldOut := s.startOutputBuffering()
	// This is 0 as the env is new, but... we just want to make sure there is
	// no get() up stack to confirm the function might be cacheable.
	before := s.env.GetMisses()
	res := s.Eval(newBody) // Need to have the return value unwrapped. Fixes bug #46, also need to count recursion.
	after := s.env.GetMisses()
	cantCache := s.env.CantCache()
	// gather output
	output := s.stopOutputBuffering()
	// restore the previous env/state.
	s.env = curState
	if len(output) > 0 {
		_, err := oldOut.Write(output)
		if err != nil {
			log.Warnf("output: %v", err)
		}
	}
	if after != before {
		log.Debugf("Cache miss for %s %v, %d get misses", function.CacheKey, args, after-before)
		// Propagate the can't cache
		if cantCache {
			s.env.TriggerNoCache()
		}
		return res
	}
	// Don't cache errors, as it could be due to binding for instance.
	if res.Type() == object.ERROR {
		log.Debugf("Cache miss for %s %v, not caching error result", function.CacheKey, args)
		return res
	}
	// TODO: reduce scope of not caching to a function that captures state (#358)
	if res.Type() == object.FUNC {
		log.Debugf("Cache miss for %s %v, not caching function returned", function.CacheKey, args)
		s.env.TriggerNoCache()
		return res
	}
	s.cache.Set(function.CacheKey, args, res, output)
	log.Debugf("Cache miss for %s %v", function.CacheKey, args)
	return res
}

func (s *State) extendFunctionEnv(
	currrentEnv *object.Environment,
	name string, fn object.Function,
	args []object.Object,
) (*object.Environment, ast.Node, *object.Error) {
	// https://github.com/grol-io/grol/issues/47
	// fn.Env is "captured state", but for recursion we now parent from current state; eg
	//     func test(n) {if (n==2) {x=1}; if (n==1) {return x}; test(n-1)}; test(3)
	// return 1 (state set by recursion with n==2)
	env, _ := object.NewFunctionEnvironment(fn, currrentEnv)
	params := fn.Parameters
	atLeast := ""
	var extra []object.Object
	if fn.Variadic {
		n := len(params) - 1
		params = params[:n]
		// Expending the last argument expecting it to be "..", but any other array will do too.
		if len(args) > 0 && args[len(args)-1].Type() == object.ARRAY {
			args = append(args[:len(args)-1], object.Elements(args[len(args)-1])...)
		}
		if len(args) >= n {
			extra = args[n:]
			args = args[:n]
		}
		atLeast = " at least"
	}
	n := len(params)
	if len(args) != n {
		oerr := s.Errorf("wrong number of arguments for %s. got=%d, want%s=%d",
			name, len(args), atLeast, n)
		return nil, nil, &oerr
	}
	var newBody ast.Node
	newBody = fn.Body
	for paramIdx, param := range params {
		// By definition function parameters are local copies, deref argument values:
		pval := object.Value(args[paramIdx])
		needVariable := true
		if !s.NoReg && pval.Type() == object.INTEGER {
			// We will release all these registers just by returning/dropping the env.
			_, nbody, ok := setupRegister(env, param.Value().Literal(), pval.(object.Integer).Value, newBody)
			if ok {
				newBody = nbody
				needVariable = false
			}
		}
		if needVariable {
			oerr := env.CreateOrSet(param.Value().Literal(), pval, true)
			if log.LogVerbose() {
				log.LogVf("set %s to %s - %s", param.Value().Literal(), args[paramIdx].Inspect(), oerr.Inspect())
			}
			if oerr.Type() == object.ERROR {
				oe, _ := oerr.(object.Error)
				return nil, nil, &oe
			}
		}
	}
	if fn.Variadic {
		env.SetNoChecks("..", object.NewArray(extra), true)
	}
	// Recursion is handle specially in Get (defining "self" and the function name in the env)
	// For recursion in named functions, set it here so we don't need to go up a stack of 50k envs to find it
	/*
		if sameFunction && name != "" {
			env.SetNoChecks(name, fn, true)
		}
	*/
	return env, newBody, nil
}

func (s *State) evalExpressions(exps []ast.Node) ([]object.Object, *object.Error) {
	result := object.MakeObjectSlice(len(exps)) // not that this one can ever be huge but, for consistency.
	for _, e := range exps {
		evaluated := s.evalInternal(e)
		if rt := evaluated.Type(); rt == object.ERROR {
			oerr := evaluated.(object.Error)
			return nil, &oerr
		}
		result = append(result, object.CopyRegister(evaluated))
	}
	return result, nil
}

func (s *State) evalIdentifier(node *ast.Identifier) object.Object {
	name := node.Literal()
	// initially we had that local var can shadow extensions - but no that makes everything a cache miss.
	// also much faster to look here first than failing to find say max() all the way looking up stack
	// and only then looking in extensions like we did at first. (possible compromise: look in local level
	// only, then extensions, then up the stack).
	ext, ok := s.Extensions[name]
	if ok {
		return ext
	}
	val, ok := s.env.Get(name)
	if !ok {
		return s.NewError("identifier not found: " + node.Literal())
	}
	return val
}

func (s *State) evalIfExpression(ie *ast.IfExpression) object.Object {
	condition := object.Value(s.evalInternal(ie.Condition))
	switch condition {
	case object.TRUE:
		if log.LogVerbose() {
			log.LogVf("if %s is object.TRUE, picking true branch", ie.Condition.Value().DebugString())
		}
		return s.evalInternal(ie.Consequence)
	case object.FALSE, object.NULL:
		if log.LogVerbose() {
			log.LogVf("if %s is object.FALSE, picking else branch", ie.Condition.Value().DebugString())
		}
		return s.evalInternal(ie.Alternative)
	default:
		return s.NewError("condition is not a boolean: " + condition.Inspect())
	}
}

func ModifyRegister(register *object.Register, in ast.Node) (ast.Node, bool) {
	switch in := in.(type) {
	case *ast.Identifier:
		if in.Literal() == register.Literal() {
			register.Count++
			return register, true
		}
	case *ast.PostfixExpression:
		if in.Prev.Literal() == register.Literal() {
			// not handled currently (x--)
			return nil, false
		}
	case *ast.FunctionLiteral:
		// skip lambda/functions in functions.
		return nil, false
	}
	return in, true
}

func setupRegister(env *object.Environment, name string, value int64, body ast.Node) (object.Register, ast.Node, bool) {
	register := env.MakeRegister(name, value)
	newBody, ok := ast.Modify(body, func(in ast.Node) (ast.Node, bool) {
		return ModifyRegister(&register, in)
	})
	if log.LogVerbose() {
		out := strings.Builder{}
		ps := &ast.PrintState{Out: &out, Compact: true}
		if newBody != nil {
			newBody.PrettyPrint(ps)
			log.LogVf("replaced %d registers - ok = %t: %s", register.Count, ok, out.String())
		} else {
			log.LogVf("replaced %d registers - ok = %t: AST modification stopped", register.Count, ok)
		}
	}
	if !ok || register.Count == 0 {
		return register, body, ok // original body unchanged.
	}
	return register, newBody, ok
}

func (s *State) evalForInteger(fe *ast.ForExpression, start *int64, end int64, name string) object.Object {
	var lastEval object.Object
	lastEval = object.NULL
	startValue := 0
	if start != nil {
		startValue = int(*start)
	}
	endValue := int(end)
	num := endValue - startValue
	if num < 0 {
		return s.Errorf("for loop with negative count [%d,%d[", startValue, endValue)
	}
	var ptr *int64
	var newBody ast.Node
	var register object.Register
	newBody = fe.Body
	if name != "" && !s.NoReg {
		var ok bool
		register, newBody, ok = setupRegister(s.env, name, int64(startValue), fe.Body)
		if !ok {
			return s.Errorf("for loop register %s shouldn't be modified inside the loop", name)
		}
		ptr = register.Ptr()
		defer s.env.ReleaseRegister(register)
	}
	for i := startValue; i < endValue; i++ {
		if s.NoReg && name != "" {
			s.env.Set(name, object.Integer{Value: int64(i)})
		}
		if ptr != nil {
			*ptr = int64(i)
		}
		nextEval := s.evalInternal(newBody)
		switch nextEval.Type() {
		case object.ERROR:
			return nextEval
		case object.RETURN:
			r := nextEval.(object.ReturnValue)
			switch r.ControlType {
			case token.BREAK:
				// log.Infof("break in for integer loop, returning %s", lastEval.Inspect())
				return lastEval
			case token.CONTINUE:
				continue
			case token.RETURN:
				return r
			default:
				return s.Errorf("for loop unexpected control type %s", r.ControlType.String())
			}
		default:
			lastEval = nextEval
		}
	}
	return lastEval
}

func (s *State) evalForSpecialForms(fe *ast.ForExpression) (object.Object, bool) {
	ie, ok := fe.Condition.(*ast.InfixExpression)
	if !ok {
		return object.NULL, false
	}
	if ie.Token.Type() != token.ASSIGN && ie.Token.Type() != token.DEFINE {
		return object.NULL, false
	}
	if ie.Left.Value().Type() != token.IDENT {
		return s.Errorf("for var = ... not a var %s", ie.Left.Value().DebugString()), true
	}
	name := ie.Left.Value().Literal()
	if ie.Right.Value().Type() == token.COLON {
		start := object.Value(s.evalInternal(ie.Right.(*ast.InfixExpression).Left))
		startInt, ok := Int64Value(start)
		if !ok {
			return s.NewError("for var = n:m n not an integer: " + start.Inspect()), true
		}
		end := object.Value(s.evalInternal(ie.Right.(*ast.InfixExpression).Right))
		endInt, ok := Int64Value(end)
		if !ok {
			return s.NewError("for var = n:m m not an integer: " + end.Inspect()), true
		}
		return s.evalForInteger(fe, &startInt, endInt, name), true
	}
	// Evaluate:
	v := object.Value(s.evalInternal(ie.Right))
	switch v.Type() {
	case object.INTEGER:
		return s.evalForInteger(fe, nil, v.(object.Integer).Value, name), true
	case object.ERROR:
		return v, true
	case object.ARRAY, object.MAP, object.STRING:
		return s.evalForList(fe, v, name), true
	default:
		return object.NULL, false
	}
}

func (s *State) evalForList(fe *ast.ForExpression, list object.Object, name string) object.Object {
	var lastEval object.Object
	lastEval = object.NULL
	for object.Len(list) > 0 {
		v := object.First(list)
		list = object.Rest(list)
		if v == nil {
			return s.NewError("for list element is nil")
		}
		oerr := s.env.CreateOrSet(name, v, true) // Create new local scope for loop variable
		if oerr.Type() == object.ERROR {
			return oerr
		}
		// Copy pasta from evalForInteger. hard to share control flow.
		nextEval := s.evalInternal(fe.Body)
		switch nextEval.Type() {
		case object.ERROR:
			return nextEval
		case object.RETURN:
			r := nextEval.(object.ReturnValue)
			switch r.ControlType {
			case token.BREAK:
				return lastEval
			case token.CONTINUE:
				continue
			case token.RETURN:
				return r
			default:
				return s.Errorf("for loop unexpected control type %s", r.ControlType.String())
			}
		default:
			lastEval = nextEval
		}
	}
	return lastEval
}

func (s *State) evalForExpression(fe *ast.ForExpression) object.Object {
	// Check if it's form of var = ...
	if v, ok := s.evalForSpecialForms(fe); ok {
		return v
	}
	// Other: condition or number of iterations for loop.
	var lastEval object.Object
	lastEval = object.NULL
	for {
		condition := object.Value(s.evalInternal(fe.Condition))
		switch condition {
		case object.TRUE:
			if log.LogVerbose() {
				log.LogVf("for %s is object.TRUE, running body", fe.Condition.Value().DebugString())
			}
			nextEval := s.evalInternal(fe.Body)
			switch nextEval.Type() {
			case object.ERROR:
				return nextEval
			case object.RETURN:
				r := nextEval.(object.ReturnValue)
				switch r.ControlType {
				case token.BREAK:
					return lastEval
				case token.CONTINUE:
					continue
				case token.RETURN:
					return r
				default:
					return s.Errorf("for loop unexpected control type %s", r.ControlType.String())
				}
			default:
				lastEval = nextEval
			}
		case object.FALSE, object.NULL:
			if log.LogVerbose() {
				log.LogVf("for %s is object.FALSE, done", fe.Condition.Value().DebugString())
			}
			return lastEval
		default:
			switch condition.Type() {
			case object.ERROR:
				return condition
			case object.REGISTER:
				return s.evalForInteger(fe, nil, condition.(*object.Register).Int64(), "")
			case object.INTEGER:
				return s.evalForInteger(fe, nil, condition.(object.Integer).Value, "")
			default:
				return s.NewError("for condition is not a boolean nor integer nor assignment: " + condition.Inspect())
			}
		}
	}
}

func isComment(node ast.Node) bool {
	_, ok := node.(*ast.Comment)
	return ok
}

func (s *State) evalStatements(stmts []ast.Node) object.Object {
	var result object.Object
	result = object.NULL // no crash when empty program.
	for _, statement := range stmts {
		if log.LogVerbose() {
			log.LogVf("eval statement %T %s", statement, statement.Value().DebugString())
		}
		if isComment(statement) {
			log.Debugf("skipping comment")
			continue
		}
		result = s.evalInternal(statement)
		log.LogVf("result statement %s", result)
		if rt := result.Type(); rt == object.RETURN || rt == object.ERROR {
			return result
		}
	}
	return result
}

func (s *State) evalPrefixExpression(operator token.Type, right object.Object) object.Object {
	switch operator {
	case token.BLOCKCOMMENT:
		// /* comment */ treated as identity operator. TODO: implement in parser.
		return right
	case token.BANG:
		return s.evalBangOperatorExpression(right)
	case token.MINUS:
		return s.evalMinusPrefixOperatorExpression(right)
	case token.BITNOT, token.BITXOR:
		rightVal, ok := Int64Value(right)
		if ok {
			return object.Integer{Value: ^rightVal}
		}
		return s.NewError("bitwise not of " + right.Inspect())
	case token.PLUS:
		// nothing do with unary plus, just return the value.
		return right
	default:
		return s.NewError("unknown operator: " + operator.String())
	}
}

func (s *State) evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case object.TRUE:
		return object.FALSE
	case object.FALSE:
		return object.TRUE
	case object.NULL:
		return object.TRUE // allow !nil == true
	default:
		return s.NewError("not of " + right.Inspect())
	}
}

func (s *State) evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	switch right.Type() {
	case object.INTEGER:
		value := right.(object.Integer).Value
		return object.Integer{Value: -value}
	case object.REGISTER:
		value := right.(*object.Register).Int64()
		return object.Integer{Value: -value}
	case object.FLOAT:
		value := right.(object.Float).Value
		return object.Float{Value: -value}
	default:
		return s.NewError("minus of " + right.Inspect())
	}
}

func (s *State) evalInfixExpression(operator token.Type, left, right object.Object) object.Object {
	rightVal, rightIsInt := Int64Value(right)
	leftVal, leftIsInt := Int64Value(left)
	switch {
	case operator == token.EQ:
		return object.NativeBoolToBooleanObject(object.Equals(left, right))
	case operator == token.NOTEQ:
		return object.NativeBoolToBooleanObject(!object.Equals(left, right))
	case operator == token.GT:
		return object.NativeBoolToBooleanObject(object.Cmp(left, right) == 1)
	case operator == token.LT:
		return object.NativeBoolToBooleanObject(object.Cmp(left, right) == -1)
	case operator == token.GTEQ:
		return object.NativeBoolToBooleanObject(object.Cmp(left, right) >= 0)
	case operator == token.LTEQ:
		return object.NativeBoolToBooleanObject(object.Cmp(left, right) <= 0)
	case operator == token.AND:
		return object.NativeBoolToBooleanObject(left == object.TRUE && right == object.TRUE)
	case operator == token.OR:
		return object.NativeBoolToBooleanObject(left == object.TRUE || right == object.TRUE)
		// can't use generics :/ see other comment.
	case rightIsInt && leftIsInt:
		return s.evalIntegerInfixExpression(operator, leftVal, rightVal)
	case left.Type() == object.FLOAT || right.Type() == object.FLOAT:
		return s.evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING:
		return s.evalStringInfixExpression(operator, left, right)
	case left.Type() == object.ARRAY:
		return s.evalArrayInfixExpression(operator, left, right)
	case left.Type() == object.MAP && right.Type() == object.MAP:
		return s.evalMapInfixExpression(operator, left, right)
	default:
		return s.NewError("no " + operator.String() + " on left=" + left.Inspect() + " right=" + right.Inspect())
	}
}

func (s *State) evalStringInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := left.(object.String).Value
	rightVal, rightIsInt := Int64Value(right)
	switch {
	case operator == token.PLUS && right.Type() == object.STRING:
		rightVal := right.(object.String).Value
		return object.String{Value: leftVal + rightVal}
	case operator == token.ASTERISK && rightIsInt:
		n := len(leftVal) * int(rightVal)
		if rightVal < 0 {
			return s.Errorf("right operand of * on strings must be a positive integer, got %d", rightVal)
		}
		object.MustBeOk(n / object.ObjectSize)
		return object.String{Value: strings.Repeat(leftVal, int(rightVal))}
	default:
		return s.Errorf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func (s *State) evalArrayInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := object.Elements(left)
	switch operator {
	case token.ASTERISK: // repeat
		rightVal, ok := Int64Value(right)
		if !ok {
			return s.NewError("right operand of * on arrays must be an integer")
		}
		// TODO: go1.23 use	slices.Repeat
		if rightVal < 0 {
			return s.NewError("right operand of * on arrays must be a positive integer")
		}
		result := object.MakeObjectSlice(len(leftVal) * int(rightVal))
		for range rightVal {
			result = append(result, leftVal...)
		}
		return object.NewArray(result)
	case token.PLUS: // concat / append
		if right.Type() != object.ARRAY {
			return object.NewArray(append(leftVal, object.Value(right)))
		}
		rightArr := object.Elements(right)
		object.MustBeOk(len(leftVal) + len(rightArr))
		return object.NewArray(append(leftVal, rightArr...))
	default:
		return s.Errorf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func (s *State) evalMapInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftMap := left.(object.Map)
	rightMap := right.(object.Map)
	switch operator {
	case token.PLUS: // concat / append
		return leftMap.Append(rightMap)
	default:
		return s.Errorf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())
	}
}

func Int64Value(o object.Object) (int64, bool) {
	switch o.Type() {
	case object.INTEGER:
		return o.(object.Integer).Value, true
	case object.REGISTER:
		return o.(*object.Register).Int64(), true
	default:
		return -1, false // use -1 to get OOB when ok isn't checked/bug.
	}
}

// You would think this is an ideal case for generics... yet...
// can't use fields directly in generic code,
// https://github.com/golang/go/issues/48522
// would need getters/setters which is not very go idiomatic.
func (s *State) evalIntegerInfixExpression(operator token.Type, leftVal, rightVal int64) object.Object {
	switch operator {
	case token.PLUS:
		return object.Integer{Value: leftVal + rightVal}
	case token.MINUS:
		return object.Integer{Value: leftVal - rightVal}
	case token.ASTERISK:
		return object.Integer{Value: leftVal * rightVal}
	case token.SLASH:
		return object.Integer{Value: leftVal / rightVal}
	case token.PERCENT:
		return object.Integer{Value: leftVal % rightVal}
	case token.LEFTSHIFT:
		return object.Integer{Value: leftVal << rightVal}
	case token.RIGHTSHIFT:
		return object.Integer{Value: int64(uint64(leftVal) >> rightVal)} //nolint:gosec // we want to be able to shift the hight bit.
	case token.BITAND:
		return object.Integer{Value: leftVal & rightVal}
	case token.BITOR:
		return object.Integer{Value: leftVal | rightVal}
	case token.BITXOR:
		return object.Integer{Value: leftVal ^ rightVal}
	case token.COLON:
		lg := rightVal - leftVal
		if lg < 0 {
			return s.NewError("range index invalid: left greater then right")
		}
		arr := object.MakeObjectSlice(int(lg))
		for i := leftVal; i < rightVal; i++ {
			arr = append(arr, object.Integer{Value: i})
		}
		return object.NewArray(arr)
	default:
		return s.NewError("unknown operator: " + operator.String())
	}
}

func GetFloatValue(o object.Object) (float64, *object.Error) {
	switch o.Type() {
	case object.REGISTER:
		return float64(o.(*object.Register).Int64()), nil
	case object.INTEGER:
		return float64(o.(object.Integer).Value), nil
	case object.FLOAT:
		return o.(object.Float).Value, nil
	default:
		// Not using state.NewError here because we want this to be reusable by extensions that do not have a state.
		// they will get the stack trace added by the eval extension code. for here we
		// will add the stack with s.ErrorAddStack().
		e := object.Error{Value: "not converting to float: " + o.Type().String()}
		return math.NaN(), &e
	}
}

// So we copy-pasta instead :-(.
func (s *State) evalFloatInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal, oerr := GetFloatValue(left)
	if oerr != nil {
		return s.ErrorAddStack(*oerr)
	}
	rightVal, oerr := GetFloatValue(right)
	if oerr != nil {
		return s.ErrorAddStack(*oerr)
	}
	switch operator {
	case token.PLUS:
		return object.Float{Value: leftVal + rightVal}
	case token.MINUS:
		return object.Float{Value: leftVal - rightVal}
	case token.ASTERISK:
		return object.Float{Value: leftVal * rightVal}
	case token.SLASH:
		return object.Float{Value: leftVal / rightVal}
	case token.PERCENT:
		return object.Float{Value: math.Mod(leftVal, rightVal)}
	default:
		return s.NewError("unknown operator: " + operator.String())
	}
}

// startOutputBuffering starts capturing output in a buffer.
// Returns the previous output writer.
func (s *State) startOutputBuffering() io.Writer {
	s.env.OutputBuffer = &bytes.Buffer{}
	s.env.PrevOut = s.Out
	s.Out = s.env.OutputBuffer
	return s.env.PrevOut
}

// stopOutputBuffering stops capturing output and restores the previous output writer.
// Returns the buffered output.
func (s *State) stopOutputBuffering() []byte {
	output := s.env.OutputBuffer.Bytes()
	s.Out = s.env.PrevOut
	s.env.OutputBuffer = nil
	s.env.PrevOut = nil
	return output
}

func isAssignment(tok token.Type) bool {
	return tok == token.ASSIGN || tok == token.DEFINE || isCompound(tok)
}

func isCompound(tok token.Type) bool {
	return tok >= token.SUMASSIGN && tok <= token.NOTASSIGN
}
