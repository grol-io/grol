package eval

import (
	"bytes"
	"fmt"
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

func (s *State) evalAssignment(right object.Object, node *ast.InfixExpression) object.Object {
	if rt := right.Type(); rt == object.ERROR {
		log.Warnf("Not assigning %q", right.Inspect())
		return right
	}
	switch node.Left.Value().Type() {
	case token.DOT:
		idxE := node.Left.(*ast.IndexExpression)
		index := object.String{Value: idxE.Index.Value().Literal()}
		return s.evalIndexAssigment(idxE.Left, index, right)
	case token.LBRACKET:
		idxE := node.Left.(*ast.IndexExpression)
		index := s.evalInternal(idxE.Index)
		return s.evalIndexAssigment(idxE.Left, index, right)
	case token.IDENT:
		id, _ := node.Left.(*ast.Identifier)
		name := id.Literal()
		log.LogVf("eval assign %#v to %s", right, name)
		return s.env.Set(name, right) // Propagate possible error (constant, extension names setting).
	default:
		return s.NewError("assignment to non identifier: " + node.Left.Value().DebugString())
	}
}

func (s *State) evalIndexAssigment(which ast.Node, index, value object.Object) object.Object {
	if which.Value().Type() != token.IDENT {
		return s.NewError("index assignment to non identifier: " + which.Value().DebugString())
	}
	id, _ := which.(*ast.Identifier)
	val, ok := s.env.Get(id.Literal())
	if !ok {
		return s.NewError("identifier not found: " + id.Literal())
	}
	switch val.Type() {
	case object.ARRAY:
		if index.Type() != object.INTEGER {
			return s.NewError("index assignment to array with non integer index: " + index.Inspect())
		}
		idx := index.(object.Integer).Value
		if idx < 0 {
			idx = int64(object.Len(val)) + idx
		}
		if idx < 0 || idx >= int64(object.Len(val)) {
			return s.NewError("index assignment out of bounds: " + index.Inspect())
		}
		elements := object.Elements(val)
		elements[idx] = value
		oerr := s.env.Set(id.Literal(), object.NewArray(elements))
		if oerr.Type() == object.ERROR {
			return oerr
		}
		return value
	case object.MAP:
		m := val.(object.Map)
		m = m.Set(index, value)
		oerr := s.env.Set(id.Literal(), m)
		if oerr.Type() == object.ERROR {
			return oerr
		}
		return value
	default:
		return object.Error{Value: fmt.Sprintf("index assignment to %s of unexpected type %s",
			id.Literal(), val.Type().String())}
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
	log.LogVf("eval prefix %s", ast.DebugString(node))
	nv := node.Value()
	if nv.Type() != token.IDENT {
		return s.NewError("can't increment/decrement " + nv.DebugString())
	}
	id := nv.Literal()
	val, ok := s.env.Get(id)
	if !ok {
		return s.NewError("identifier not found: " + id)
	}
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
		return s.NewError("can't increment/decrement " + val.Type().String())
	}
}

func (s *State) evalPostfixExpression(node *ast.PostfixExpression) object.Object {
	log.LogVf("eval postfix %s", node.DebugString())
	id := node.Prev.Literal()
	val, ok := s.env.Get(id)
	if !ok {
		return s.NewError("identifier not found: " + id)
	}
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
		return s.NewError("can't increment/decrement " + val.Type().String())
	}
	if oerr.Type() == object.ERROR {
		return oerr
	}
	return val
}

// Doesn't unwrap return - return bubbles up.
func (s *State) evalInternal(node any) object.Object { //nolint:funlen,gocyclo,gocognit // quite a lot of cases.
	if s.Context != nil && s.Context.Err() != nil {
		return s.Error(s.Context.Err())
	}
	switch node := node.(type) {
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
		log.LogVf("eval prefix %s", node.DebugString())
		switch node.Type() {
		case token.INCR, token.DECR:
			return s.evalPrefixIncrDecr(node.Type(), node.Right)
		default:
			right := s.evalInternal(node.Right)
			if right.Type() == object.ERROR {
				return right
			}
			return s.evalPrefixExpression(node.Type(), right)
		}
	case *ast.PostfixExpression:
		return s.evalPostfixExpression(node)
	case *ast.InfixExpression:
		log.LogVf("eval infix %s", node.DebugString())
		// Eval and not evalInternal because we need to unwrap "return".
		if node.Token.Type() == token.ASSIGN {
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
		fn.SetCacheKey() // sets cache key
		if name != nil {
			oerr := s.env.Set(name.Literal(), fn)
			if oerr.Type() == object.ERROR {
				return oerr // propagate that func FOO() { ... } can only be defined once.
			}
		}
		return fn
	case *ast.CallExpression:
		f := s.evalInternal(node.Function)
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
		left := s.evalInternal(node.Left)
		var index object.Object
		if node.Token.Type() == token.DOT {
			// index is the string value and not an identifier.
			index = object.String{Value: node.Index.Value().Literal()}
		} else {
			if node.Index.Value().Type() == token.COLON {
				rangeExp := node.Index.(*ast.InfixExpression)
				return s.evalIndexRangeExpression(left, rangeExp.Left, rangeExp.Right)
			}
			index = s.evalInternal(node.Index)
		}
		return s.evalIndexExpression(left, index)
	case *ast.Comment:
		return object.NULL
	}
	return s.Errorf("unknown node type: %T", node)
}

func (s *State) evalMapLiteral(node *ast.MapLiteral) object.Object {
	result := object.NewMapSize(len(node.Order))

	for _, keyNode := range node.Order {
		valueNode := node.Pairs[keyNode]
		key := s.evalInternal(keyNode)
		if !object.Equals(key, key) {
			log.Warnf("key %s is not hashable", key.Inspect())
			return s.NewError("key " + key.Inspect() + " is not hashable")
		}
		value := s.evalInternal(valueNode)
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
	if t == token.QUOTE {
		return s.quote(node.Parameters[0])
	}
	var val object.Object
	var rt object.Type
	if minV > 0 {
		val = s.evalInternal(node.Parameters[0])
		rt = val.Type()
		if rt == object.ERROR && t != token.LOG { // log can log (and thus catch) errors.
			return val
		}
	}
	switch t {
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
	leftIndex := s.evalInternal(leftIdx)
	nilRight := (rightIdx == nil)
	var rightIndex object.Object
	if nilRight {
		log.Debugf("eval index %s[%s:]", left.Inspect(), leftIndex.Inspect())
	} else {
		rightIndex = s.evalInternal(rightIdx)
		log.Debugf("eval index %s[%s:%s]", left.Inspect(), leftIndex.Inspect(), rightIndex.Inspect())
	}
	if leftIndex.Type() != object.INTEGER || (!nilRight && rightIndex.Type() != object.INTEGER) {
		return s.NewError("range index not integer")
	}
	num := object.Len(left)
	l := leftIndex.(object.Integer).Value
	if l < 0 { // negative is relative to the end.
		l = int64(num) + l
	}
	var r int64
	if nilRight {
		r = int64(num)
	} else {
		r = rightIndex.(object.Integer).Value
		if r < 0 {
			r = int64(num) + r
		}
	}
	if l > r {
		return s.NewError("range index invalid: left greater then right")
	}
	l = min(l, int64(num))
	r = min(r, int64(num))
	switch {
	case left.Type() == object.STRING:
		str := left.(object.String).Value
		return object.String{Value: str[l:r]}
	case left.Type() == object.ARRAY:
		return object.NewArray(object.Elements(left)[l:r])
	case left.Type() == object.MAP:
		return object.Range(left, l, r) // could call that one for all of them...
	case left.Type() == object.NIL:
		return object.NULL
	default:
		return s.NewError("range index operator not supported: " + left.Type().String())
	}
}

func (s *State) evalIndexExpression(left, index object.Object) object.Object {
	idxOrZero := index
	if idxOrZero.Type() == object.NIL {
		idxOrZero = object.Integer{Value: 0}
	}
	switch {
	case left.Type() == object.STRING && idxOrZero.Type() == object.INTEGER:
		idx := idxOrZero.(object.Integer).Value
		str := left.(object.String).Value
		num := len(str)
		if idx < 0 { // negative is relative to the end.
			idx = int64(num) + idx
		}
		if idx < 0 || idx >= int64(len(str)) {
			return object.NULL
		}
		return object.Integer{Value: int64(str[idx])}
	case left.Type() == object.ARRAY && idxOrZero.Type() == object.INTEGER:
		return evalArrayIndexExpression(left, idxOrZero)
	case left.Type() == object.MAP:
		return evalMapIndexExpression(left, index)
	case left.Type() == object.NIL:
		return object.NULL
	default:
		return s.NewError("index operator not supported: " + left.Type().String() + "[" + index.Type().String() + "]")
	}
}

func evalMapIndexExpression(hash, key object.Object) object.Object {
	m := hash.(object.Map)
	v, ok := m.Get(key)
	if !ok {
		return object.NULL
	}
	return v
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	idx := index.(object.Integer).Value
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
	l := len(args)
	log.Debugf("apply extension %s variadic %t : %d args %v", fn.Inspect(), fn.Variadic, l, args)
	if fn.MaxArgs == -1 {
		// Only do this for true variadic functions (maxargs == -1)
		if l > 0 && args[l-1].Type() == object.ARRAY {
			args = append(args[:l-1], object.Elements(args[l-1])...)
			l = len(args)
			log.Debugf("expending last arg now %d args %v", l, args)
		}
	}
	if l < fn.MinArgs {
		return object.Error{Value: fmt.Sprintf("wrong number of arguments got=%d, want %s",
			l, fn.Inspect())} // shows usage
	}
	if fn.MaxArgs != -1 && l > fn.MaxArgs {
		return object.Error{Value: fmt.Sprintf("wrong number of arguments got=%d, want %s",
			l, fn.Inspect())} // shows usage
	}
	for i, arg := range args {
		if i >= len(fn.ArgTypes) {
			break
		}
		if fn.ArgTypes[i] == object.ANY {
			continue
		}
		// Auto promote integer to float if needed.
		if fn.ArgTypes[i] == object.FLOAT && arg.Type() == object.INTEGER {
			args[i] = object.Float{Value: float64(arg.(object.Integer).Value)}
			continue
		}
		if fn.ArgTypes[i] != arg.Type() {
			return object.Error{Value: fmt.Sprintf("wrong type of argument got=%s, want %s",
				arg.Type(), fn.Inspect())}
		}
	}
	if fn.ClientData != nil {
		return fn.Callback(fn.ClientData, fn.Name, args)
	}
	return fn.Callback(s, fn.Name, args)
}

func (s *State) applyFunction(name string, fn object.Object, args []object.Object) object.Object {
	function, ok := fn.(object.Function)
	if !ok {
		return s.NewError("not a function: " + fn.Type().String() + ":" + fn.Inspect())
	}
	if v, output, ok := s.cache.Get(function.CacheKey, args); ok {
		log.Debugf("Cache hit for %s %v", function.CacheKey, args)
		_, err := s.Out.Write(output)
		if err != nil {
			log.Warnf("output: %v", err)
		}
		return v
	}
	nenv, oerr := extendFunctionEnv(s.env, name, function, args)
	if oerr != nil {
		return *oerr
	}
	curState := s.env
	s.env = nenv
	oldOut := s.Out
	buf := bytes.Buffer{}
	s.Out = &buf
	// This is 0 as the env is new, but... we just want to make sure there is
	// no get() up stack to confirm the function might be cacheable.
	before := s.env.GetMisses()
	res := s.Eval(function.Body) // Need to have the return value unwrapped. Fixes bug #46, also need to count recursion.
	after := s.env.GetMisses()
	// restore the previous env/state.
	s.env = curState
	s.Out = oldOut
	output := buf.Bytes()
	_, err := s.Out.Write(output)
	if err != nil {
		log.Warnf("output: %v", err)
	}
	if after != before {
		log.Debugf("Cache miss for %s %v, %d get misses", function.CacheKey, args, after-before)
		return res
	}
	// Don't cache errors, as it could be due to binding for instance.
	if res.Type() == object.ERROR {
		log.Debugf("Cache miss for %s %v, not caching error result", function.CacheKey, args)
		return res
	}
	s.cache.Set(function.CacheKey, args, res, output)
	log.Debugf("Cache miss for %s %v", function.CacheKey, args)
	return res
}

func extendFunctionEnv(
	currrentEnv *object.Environment,
	name string, fn object.Function,
	args []object.Object,
) (*object.Environment, *object.Error) {
	// https://github.com/grol-io/grol/issues/47
	// fn.Env is "captured state", but for recursion we now parent from current state; eg
	//     func test(n) {if (n==2) {x=1}; if (n==1) {return x}; test(n-1)}; test(3)
	// return 1 (state set by recursion with n==2)
	// Make sure `self` is used to recurse, or named function, otherwise the function will
	// need to be found way up the now much deeper stack.
	env, sameFunction := object.NewFunctionEnvironment(fn, currrentEnv)
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
		return nil, &object.Error{Value: fmt.Sprintf("wrong number of arguments for %s. got=%d, want%s=%d",
			name, len(args), atLeast, n)}
	}
	for paramIdx, param := range params {
		oerr := env.Set(param.Value().Literal(), args[paramIdx])
		log.LogVf("set %s to %s - %s", param.Value().Literal(), args[paramIdx].Inspect(), oerr.Inspect())
		if oerr.Type() == object.ERROR {
			oe, _ := oerr.(object.Error)
			return nil, &oe
		}
	}
	if fn.Variadic {
		env.SetNoChecks("..", object.NewArray(extra))
	}
	// for recursion in anonymous functions.
	// TODO: consider not having to keep setting this in the function's env and treating as a keyword.
	env.SetNoChecks("self", fn)
	// For recursion in named functions, set it here so we don't need to go up a stack of 50k envs to find it
	if sameFunction && name != "" {
		env.SetNoChecks(name, fn)
	}
	return env, nil
}

func (s *State) evalExpressions(exps []ast.Node) ([]object.Object, *object.Error) {
	result := object.MakeObjectSlice(len(exps)) // not that this one can ever be huge but, for consistency.
	for _, e := range exps {
		evaluated := s.evalInternal(e)
		if rt := evaluated.Type(); rt == object.ERROR {
			oerr := evaluated.(object.Error)
			return nil, &oerr
		}
		result = append(result, evaluated)
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
	condition := s.evalInternal(ie.Condition)
	switch condition {
	case object.TRUE:
		log.LogVf("if %s is object.TRUE, picking true branch", ie.Condition.Value().DebugString())
		return s.evalInternal(ie.Consequence)
	case object.FALSE:
		log.LogVf("if %s is object.FALSE, picking else branch", ie.Condition.Value().DebugString())
		return s.evalInternal(ie.Alternative)
	default:
		return s.NewError("condition is not a boolean: " + condition.Inspect())
	}
}

func (s *State) evalForInteger(fe *ast.ForExpression, start *object.Integer, end object.Integer, name string) object.Object {
	var lastEval object.Object
	lastEval = object.NULL
	startValue := 0
	if start != nil {
		startValue = int(start.Value)
	}
	endValue := int(end.Value)
	num := endValue - startValue
	if num < 0 {
		return s.Errorf("for loop with negative count [%d,%d[", startValue, endValue)
	}
	for i := startValue; i < endValue; i++ {
		if name != "" {
			s.env.Set(name, object.Integer{Value: int64(i)})
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
	}
	return lastEval
}

func (s *State) evalForSpecialForms(fe *ast.ForExpression) (object.Object, bool) {
	ie, ok := fe.Condition.(*ast.InfixExpression)
	if !ok {
		return object.NULL, false
	}
	if ie.Token.Type() != token.ASSIGN {
		return object.NULL, false
	}
	if ie.Left.Value().Type() != token.IDENT {
		return s.Errorf("for var = ... not a var %s", ie.Left.Value().DebugString()), true
	}
	name := ie.Left.Value().Literal()
	if ie.Right.Value().Type() == token.COLON {
		start := s.evalInternal(ie.Right.(*ast.InfixExpression).Left)
		if start.Type() != object.INTEGER {
			return s.NewError("for var = n:m n not an integer: " + start.Inspect()), true
		}
		end := s.evalInternal(ie.Right.(*ast.InfixExpression).Right)
		if end.Type() != object.INTEGER {
			return s.NewError("for var = n:m m not an integer: " + end.Inspect()), true
		}
		startInt := start.(object.Integer)
		return s.evalForInteger(fe, &startInt, end.(object.Integer), name), true
	}
	// Evaluate:
	v := s.Eval(ie.Right)
	switch v.Type() {
	case object.INTEGER:
		return s.evalForInteger(fe, nil, v.(object.Integer), name), true
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
		s.env.Set(name, v)
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
		condition := s.evalInternal(fe.Condition)
		switch condition {
		case object.TRUE:
			log.LogVf("for %s is object.TRUE, running body", fe.Condition.Value().DebugString())
			lastEval = s.evalInternal(fe.Body)
			if rt := lastEval.Type(); rt == object.RETURN || rt == object.ERROR {
				return lastEval
			}
		case object.FALSE, object.NULL:
			log.LogVf("for %s is object.FALSE, done", fe.Condition.Value().DebugString())
			return lastEval
		default:
			switch condition.Type() {
			case object.ERROR:
				return condition
			case object.INTEGER:
				return s.evalForInteger(fe, nil, condition.(object.Integer), "")
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
		log.LogVf("eval statement %T %s", statement, statement.Value().DebugString())
		if isComment(statement) {
			log.Debugf("skipping comment")
			continue
		}
		result = s.evalInternal(statement)
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
		if right.Type() == object.INTEGER {
			return object.Integer{Value: ^right.(object.Integer).Value}
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
	case object.FLOAT:
		value := right.(object.Float).Value
		return object.Float{Value: -value}
	default:
		return s.NewError("minus of " + right.Inspect())
	}
}

func (s *State) evalInfixExpression(operator token.Type, left, right object.Object) object.Object {
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
	case left.Type() == object.INTEGER && right.Type() == object.INTEGER:
		return s.evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT || right.Type() == object.FLOAT:
		return s.evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING:
		return s.evalStringInfixExpression(operator, left, right)
	case left.Type() == object.ARRAY:
		return s.evalArrayInfixExpression(operator, left, right)
	case left.Type() == object.MAP && right.Type() == object.MAP:
		return evalMapInfixExpression(operator, left, right)
	default:
		return s.NewError("no " + operator.String() + " on left=" + left.Inspect() + " right=" + right.Inspect())
	}
}

func (s *State) evalStringInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := left.(object.String).Value
	switch {
	case operator == token.PLUS && right.Type() == object.STRING:
		rightVal := right.(object.String).Value
		return object.String{Value: leftVal + rightVal}
	case operator == token.ASTERISK && right.Type() == object.INTEGER:
		rightVal := right.(object.Integer).Value
		n := len(leftVal) * int(rightVal)
		if rightVal < 0 {
			return s.NewError("right operand of * on strings must be a positive integer")
		}
		object.MustBeOk(n / object.ObjectSize)
		return object.String{Value: strings.Repeat(leftVal, int(rightVal))}
	default:
		return object.Error{Value: fmt.Sprintf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())}
	}
}

func (s *State) evalArrayInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := object.Elements(left)
	switch operator {
	case token.ASTERISK: // repeat
		if right.Type() != object.INTEGER {
			return s.NewError("right operand of * on arrays must be an integer")
		}
		// TODO: go1.23 use	slices.Repeat
		rightVal := right.(object.Integer).Value
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
			return object.NewArray(append(leftVal, right))
		}
		rightArr := object.Elements(right)
		object.MustBeOk(len(leftVal) + len(rightArr))
		return object.NewArray(append(leftVal, rightArr...))
	default:
		return object.Error{Value: fmt.Sprintf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())}
	}
}

func evalMapInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftMap := left.(object.Map)
	rightMap := right.(object.Map)
	switch operator {
	case token.PLUS: // concat / append
		return leftMap.Append(rightMap)
	default:
		return object.Error{Value: fmt.Sprintf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())}
	}
}

// You would think this is an ideal case for generics... yet...
// can't use fields directly in generic code,
// https://github.com/golang/go/issues/48522
// would need getters/setters which is not very go idiomatic.
func (s *State) evalIntegerInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := left.(object.Integer).Value
	rightVal := right.(object.Integer).Value

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

func (s *State) getFloatValue(o object.Object) (float64, *object.Error) {
	switch o.Type() {
	case object.INTEGER:
		return float64(o.(object.Integer).Value), nil
	case object.FLOAT:
		return o.(object.Float).Value, nil
	default:
		e := s.NewError("not converting to float: " + o.Type().String())
		return math.NaN(), &e
	}
}

// So we copy-pasta instead :-(.
func (s *State) evalFloatInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal, oerr := s.getFloatValue(left)
	if oerr != nil {
		return *oerr
	}
	rightVal, oerr := s.getFloatValue(right)
	if oerr != nil {
		return *oerr
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
