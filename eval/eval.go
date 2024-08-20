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
	// let free assignments.
	id, ok := node.Left.(*ast.Identifier)
	if !ok {
		return object.Error{Value: "assignment to non identifier: " + node.Left.Value().DebugString()}
	}
	if rt := right.Type(); rt == object.ERROR {
		log.Warnf("can't assign %q: %v", right.Inspect(), right)
		return right
	}
	log.LogVf("eval assign %#v to %#v", right, id.Value())
	return s.env.Set(id.Literal(), right) // Propagate possible error (constant setting).
}

func argCheck[T any](msg string, n int, vararg bool, args []T) *object.Error {
	if vararg {
		if len(args) < n {
			return &object.Error{Value: fmt.Sprintf("%s: wrong number of arguments. got=%d, want at least %d", msg, len(args), n)}
		}
		return nil
	}
	if len(args) != n {
		return &object.Error{Value: fmt.Sprintf("%s: wrong number of arguments. got=%d, want=%d", msg, len(args), n)}
	}
	return nil
}

func (s *State) evalPrefixIncrDecr(operator token.Type, node ast.Node) object.Object {
	log.LogVf("eval prefix %s", ast.DebugString(node))
	nv := node.Value()
	if nv.Type() != token.IDENT {
		return object.Error{Value: "can't increment/decrement " + nv.DebugString()}
	}
	id := nv.Literal()
	val, ok := s.env.Get(id)
	if !ok {
		return object.Error{Value: "identifier not found: " + id}
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
		return object.Error{Value: "can't increment/decrement " + val.Type().String()}
	}
}

func (s *State) evalPostfixExpression(node *ast.PostfixExpression) object.Object {
	log.LogVf("eval postfix %s", node.DebugString())
	id := node.Prev.Literal()
	val, ok := s.env.Get(id)
	if !ok {
		return object.Error{Value: "identifier not found: " + id}
	}
	var toAdd int64
	switch node.Type() { //nolint:exhaustive // we have default.
	case token.INCR:
		toAdd = 1
	case token.DECR:
		toAdd = -1
	default:
		return object.Error{Value: "unknown postfix operator: " + node.Type().String()}
	}
	var oerr object.Object
	switch val := val.(type) {
	case object.Integer:
		oerr = s.env.Set(id, object.Integer{Value: val.Value + toAdd})
	case object.Float:
		oerr = s.env.Set(id, object.Float{Value: val.Value + float64(toAdd)}) // So PI++ fails not silently.
	default:
		return object.Error{Value: "can't increment/decrement " + val.Type().String()}
	}
	if oerr.Type() == object.ERROR {
		return oerr
	}
	return val
}

// Doesn't unwrap return - return bubbles up.
func (s *State) evalInternal(node any) object.Object { //nolint:funlen,gocyclo // quite a lot of cases.
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
		// Expressions
	case *ast.Identifier:
		return s.evalIdentifier(node)
	case *ast.PrefixExpression:
		log.LogVf("eval prefix %s", node.DebugString())
		switch node.Type() { //nolint:exhaustive // we have default.
		case token.INCR, token.DECR:
			return s.evalPrefixIncrDecr(node.Type(), node.Right)
		default:
			right := s.evalInternal(node.Right)
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
		// Short circuiting for AND and OR:
		if node.Token.Type() == token.AND && left == object.FALSE {
			return object.FALSE
		}
		if node.Token.Type() == token.OR && left == object.TRUE {
			return object.TRUE
		}
		right := s.Eval(node.Right)
		return s.evalInfixExpression(node.Type(), left, right)

	case *ast.IntegerLiteral:
		return object.Integer{Value: node.Val}
	case *ast.FloatLiteral:
		return object.Float{Value: node.Val}

	case *ast.Boolean:
		return object.NativeBoolToBooleanObject(node.Val)

	case *ast.StringLiteral:
		return object.String{Value: node.Literal()}

	case *ast.ReturnStatement:
		if node.ReturnValue == nil {
			return object.ReturnValue{Value: object.NULL}
		}
		val := s.evalInternal(node.ReturnValue)
		return object.ReturnValue{Value: val}
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
			index = s.evalInternal(node.Index)
		}
		return evalIndexExpression(left, index)
	case *ast.Comment:
		return object.NULL
	}
	return object.Error{Value: fmt.Sprintf("unknown node type: %T", node)}
}

func (s *State) evalMapLiteral(node *ast.MapLiteral) object.Object {
	result := object.NewMapSize(len(node.Order))

	for _, keyNode := range node.Order {
		valueNode := node.Pairs[keyNode]
		key := s.evalInternal(keyNode)
		if !object.Equals(key, key) {
			log.Warnf("key %s is not hashable", key.Inspect())
			return object.Error{Value: "key " + key.Inspect() + " is not hashable"}
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
		if isString := r.Type() == object.STRING; isString {
			buf.WriteString(r.(object.String).Value)
		} else {
			buf.WriteString(r.Inspect())
		}
	}
	if node.Type() == token.ERROR {
		return object.Error{Value: buf.String()}
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
	if oerr := argCheck(node.Literal(), minV, varArg, node.Parameters); oerr != nil {
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
		if rt == object.ERROR {
			return val
		}
	}
	switch t { //nolint:exhaustive // we have defaults and covering all the builtins.
	case token.ERROR, token.PRINT, token.PRINTLN, token.LOG:
		return s.evalPrintLogError(node)
	case token.FIRST:
		return object.First(val)
	case token.REST:
		return object.Rest(val)
	case token.LEN:
		l := object.Len(val)
		if l == -1 {
			return object.Error{Value: "len: not supported on " + val.Type().String()}
		}
		return object.Integer{Value: int64(l)}
	default:
		return object.Error{Value: fmt.Sprintf("builtin %s yet implemented", node.Type())}
	}
}

func evalIndexExpression(left, index object.Object) object.Object {
	idxOrZero := index
	if idxOrZero.Type() == object.NIL {
		idxOrZero = object.Integer{Value: 0}
	}
	switch {
	case left.Type() == object.STRING && idxOrZero.Type() == object.INTEGER:
		idx := idxOrZero.(object.Integer).Value
		str := left.(object.String).Value
		if idx < 0 || idx >= int64(len(str)) {
			return object.NULL
		}
		return object.Integer{Value: int64(str[idx])} //nolint:gosec // https://github.com/securego/gosec/issues/1185
	case left.Type() == object.ARRAY && idxOrZero.Type() == object.INTEGER:
		return evalArrayIndexExpression(left, idxOrZero)
	case left.Type() == object.MAP:
		return evalMapIndexExpression(left, index)
	case left.Type() == object.NIL:
		return object.NULL
	default:
		return object.Error{Value: "index operator not supported: " + left.Type().String() + "[" + index.Type().String() + "]"}
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
	return fn.Callback(s, fn.Name, args)
}

func (s *State) applyFunction(name string, fn object.Object, args []object.Object) object.Object {
	function, ok := fn.(object.Function)
	if !ok {
		return object.Error{Value: "not a function: " + fn.Type().String() + ":" + fn.Inspect()}
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
	// local var can shadow extensions.
	val, ok := s.env.Get(node.Literal())
	if ok {
		return val
	}
	val, ok = s.extensions[node.Literal()]
	if !ok {
		return object.Error{Value: "identifier not found: " + node.Literal()}
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
		return object.Error{Value: "condition is not a boolean: " + condition.Inspect()}
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
	switch operator { //nolint:exhaustive // we have default.
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
		return object.Error{Value: "bitwise not of " + right.Inspect()}
	case token.PLUS:
		// nothing do with unary plus, just return the value.
		return right
	default:
		return object.Error{Value: "unknown operator: " + operator.String()}
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
		return object.Error{Value: "not of " + right.Inspect()}
	}
}

func (s *State) evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	switch right.Type() { //nolint:exhaustive // we have default which is errors.
	case object.INTEGER:
		value := right.(object.Integer).Value
		return object.Integer{Value: -value}
	case object.FLOAT:
		value := right.(object.Float).Value
		return object.Float{Value: -value}
	default:
		return object.Error{Value: "minus of " + right.Inspect()}
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
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT || right.Type() == object.FLOAT:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.ARRAY:
		return evalArrayInfixExpression(operator, left, right)
	case left.Type() == object.MAP && right.Type() == object.MAP:
		return evalMapInfixExpression(operator, left, right)
	default:
		return object.Error{Value: "operation on non integers left=" + left.Inspect() + " right=" + right.Inspect()}
	}
}

func evalStringInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := left.(object.String).Value
	switch {
	case operator == token.PLUS && right.Type() == object.STRING:
		rightVal := right.(object.String).Value
		return object.String{Value: leftVal + rightVal}
	case operator == token.ASTERISK && right.Type() == object.INTEGER:
		rightVal := right.(object.Integer).Value
		n := len(leftVal) * int(rightVal)
		object.MustBeOk(n / object.ObjectSize)
		return object.String{Value: strings.Repeat(leftVal, int(rightVal))}
	default:
		return object.Error{Value: fmt.Sprintf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())}
	}
}

func evalArrayInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := object.Elements(left)
	switch operator { //nolint:exhaustive // we have default.
	case token.ASTERISK: // repeat
		if right.Type() != object.INTEGER {
			return object.Error{Value: "right operand of * on arrays must be an integer"}
		}
		// TODO: go1.23 use	slices.Repeat
		rightVal := right.(object.Integer).Value
		if rightVal < 0 {
			return object.Error{Value: "right operand of * on arrays must be a positive integer"}
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
	switch operator { //nolint:exhaustive // we have default.
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
func evalIntegerInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := left.(object.Integer).Value
	rightVal := right.(object.Integer).Value

	switch operator { //nolint:exhaustive // we have default.
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
	default:
		return object.Error{Value: "unknown operator: " + operator.String()}
	}
}

func getFloatValue(o object.Object) (float64, *object.Error) {
	switch o.Type() { //nolint:exhaustive // we handle the others in default.
	case object.INTEGER:
		return float64(o.(object.Integer).Value), nil
	case object.FLOAT:
		return o.(object.Float).Value, nil
	default:
		return math.NaN(), &object.Error{Value: "not converting to float: " + o.Type().String()}
	}
}

// So we copy-pasta instead :-(.
func evalFloatInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal, oerr := getFloatValue(left)
	if oerr != nil {
		return *oerr
	}
	rightVal, oerr := getFloatValue(right)
	if oerr != nil {
		return *oerr
	}
	switch operator { //nolint:exhaustive // we have default.
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
		return object.Error{Value: "unknown operator: " + operator.String()}
	}
}
