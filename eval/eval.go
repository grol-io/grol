package eval

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"fortio.org/log"
	"grol.io/grol/ast"
	"grol.io/grol/object"
	"grol.io/grol/token"
)

type State struct {
	env        *object.Environment
	Out        io.Writer
	LogOut     io.Writer
	NoLog      bool // turn log() into println() (for EvalString)
	cache      Cache
	extensions map[string]object.Extension
}

func NewState() *State {
	return &State{
		env:        object.NewEnvironment(),
		Out:        os.Stdout,
		LogOut:     os.Stdout,
		cache:      NewCache(),
		extensions: object.Commands(),
	}
}

func (s *State) ResetCache() {
	s.cache = NewCache()
}

// Forward to env to count the number of bindings. Used mostly to know if there are any macros.
func (s *State) Len() int {
	return s.env.Len()
}

// Does unwrap (so stop bubbling up) return values.
func (s *State) Eval(node any) object.Object {
	result := s.evalInternal(node)
	// unwrap return values only at the top.
	if returnValue, ok := result.(object.ReturnValue); ok {
		return returnValue.Value
	}
	return result
}

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
	s.env.Set(id.Literal(), right)
	return right // maybe only if it's a literal?
}

func ArgCheck[T any](msg string, n int, vararg bool, args []T) *object.Error {
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
	switch val := val.(type) {
	case object.Integer:
		s.env.Set(id, object.Integer{Value: val.Value + toAdd})
	case object.Float:
		s.env.Set(id, object.Float{Value: val.Value + float64(toAdd)})
	default:
		return object.Error{Value: "can't increment/decrement " + val.Type().String()}
	}
	return val
}

// Doesn't unwrap return - return bubbles up.
func (s *State) evalInternal(node any) object.Object { //nolint:funlen // we have a lot of cases.
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
		right := s.evalInternal(node.Right)
		return s.evalPrefixExpression(node.Type(), right)
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
		params := node.Parameters
		body := node.Body
		name := node.Name
		fn := object.Function{Parameters: params, Name: name, Env: s.env, Body: body}
		fn.SetCacheKey() // sets cache key
		if name != nil {
			s.env.Set(name.Literal(), fn)
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
		return object.Array{Elements: elements}
	case *ast.MapLiteral:
		return s.evalMapLiteral(node)
	case *ast.IndexExpression:
		left := s.evalInternal(node.Left)
		index := s.evalInternal(node.Index)
		return evalIndexExpression(left, index)
	case *ast.Comment:
		return object.NULL
	}
	return object.Error{Value: fmt.Sprintf("unknown node type: %T", node)}
}

func hashable(o object.Object) *object.Error {
	t := o.Type()
	// because it contains env which is a map.
	if t == object.FUNC || t == object.ARRAY || t == object.MAP {
		return &object.Error{Value: o.Type().String() + " not usable as map key"}
	}
	return nil
}

func (s *State) evalMapLiteral(node *ast.MapLiteral) object.Object {
	result := object.NewMap()

	for _, keyNode := range node.Order {
		valueNode := node.Pairs[keyNode]
		key := s.evalInternal(keyNode)
		value := s.evalInternal(valueNode)
		if oerr := hashable(key); oerr != nil {
			return *oerr
		}
		result[key] = value
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
	min := 1
	varArg := (t == token.PRINT || t == token.LOG || t == token.ERROR)
	if t == token.PRINTLN {
		min = 0
		varArg = true
	}
	if oerr := ArgCheck(node.Literal(), min, varArg, node.Parameters); oerr != nil {
		return *oerr
	}
	if t == token.QUOTE {
		return s.quote(node.Parameters[0])
	}
	var val object.Object
	var rt object.Type
	if min > 0 {
		val = s.evalInternal(node.Parameters[0])
		rt = val.Type()
		if rt == object.ERROR {
			return val
		}
	}
	arr, _ := val.(object.Array)
	switch t { //nolint:exhaustive // we have defaults and covering all the builtins.
	case token.ERROR:
		fallthrough
	case token.PRINT:
		fallthrough
	case token.PRINTLN:
		fallthrough
	case token.LOG:
		return s.evalPrintLogError(node)
	case token.FIRST:
		if rt != object.ARRAY {
			break
		}
		if len(arr.Elements) == 0 {
			return object.NULL
		}
		return arr.Elements[0]
	case token.REST:
		if rt != object.ARRAY {
			break
		}
		return object.Array{Elements: arr.Elements[1:]}
	case token.LEN:
		switch rt { //nolint:exhaustive // we have default, len doesn't work on many types.
		case object.STRING:
			return object.Integer{Value: int64(len(val.(object.String).Value))}
		case object.ARRAY:
			return object.Integer{Value: int64(len(arr.Elements))}
		case object.NIL:
			return object.Integer{Value: 0}
		}
	default:
		return object.Error{Value: fmt.Sprintf("builtin %s yet implemented", node.Type())}
	}
	return object.Error{Value: node.Literal() + ": not supported on " + rt.String()}
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY && index.Type() == object.INTEGER:
		return evalArrayIndexExpression(left, index)
	case left.Type() == object.MAP:
		return evalMapIndexExpression(left, index)
	default:
		return object.Error{Value: "index operator not supported: " + left.Type().String() + "[" + index.Type().String() + "]"}
	}
}

func evalMapIndexExpression(hash, key object.Object) object.Object {
	if oerr := hashable(key); oerr != nil {
		return *oerr
	}
	m := hash.(object.Map)
	v, ok := m[key]
	if !ok {
		return object.NULL
	}
	return v
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(object.Array)
	idx := index.(object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return object.NULL
	}
	return arrayObject.Elements[idx]
}

func (s *State) applyExtension(fn object.Extension, args []object.Object) object.Object {
	l := len(args)
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
	return fn.Callback(args)
}

func (s *State) applyFunction(name string, fn object.Object, args []object.Object) object.Object {
	function, ok := fn.(object.Function)
	if !ok {
		return object.Error{Value: "not a function: " + fn.Type().String() + ":" + fn.Inspect()}
	}
	if v, output, ok := s.cache.Get(function.CacheKey, args); ok {
		log.Debugf("Cache hit for %s %v", function.CacheKey, args)
		_, _ = s.Out.Write(output)
		return v
	}
	nenv, oerr := extendFunctionEnv(name, function, args)
	if oerr != nil {
		return *oerr
	}
	curState := s.env
	s.env = nenv
	oldOut := s.Out
	buf := bytes.Buffer{}
	s.Out = &buf
	res := s.Eval(function.Body) // Need to have the return value unwrapped. Fixes bug #46
	// restore the previous env/state.
	s.env = curState
	s.Out = oldOut
	output := buf.Bytes()
	_, _ = s.Out.Write(output)
	s.cache.Set(function.CacheKey, args, res, output)
	log.Debugf("Cache miss for %s %v", function.CacheKey, args)
	return res
}

func extendFunctionEnv(name string, fn object.Function, args []object.Object) (*object.Environment, *object.Error) {
	env := object.NewEnclosedEnvironment(fn.Env)
	n := len(fn.Parameters)
	if len(args) != n {
		return nil, &object.Error{Value: fmt.Sprintf("wrong number of arguments for %s. got=%d, want=%d",
			name, len(args), n)}
	}
	for paramIdx, param := range fn.Parameters {
		env.Set(param.Value().Literal(), args[paramIdx])
	}
	// for recursion in anonymous functions.
	// TODO: consider not having to keep setting this in the function's env and treating as a keyword.
	env.Set("self", fn)
	return env, nil
}

func (s *State) evalExpressions(exps []ast.Node) ([]object.Object, *object.Error) {
	result := make([]object.Object, 0, len(exps))
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
		return object.Error{Value: "not of object.NULL"}
	default:
		return object.Error{Value: "not of " + right.Inspect()}
	}
}

func (s *State) evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() != object.INTEGER {
		return object.Error{Value: "minus of " + right.Inspect()}
	}

	value := right.(object.Integer).Value
	return object.Integer{Value: -value}
}

func (s *State) evalInfixExpression(operator token.Type, left, right object.Object) object.Object {
	switch {
	// can't use generics :/ see other comment.
	case left.Type() == object.INTEGER && right.Type() == object.INTEGER:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.FLOAT || right.Type() == object.FLOAT:
		return evalFloatInfixExpression(operator, left, right)
	case left.Type() == object.STRING && right.Type() == object.STRING:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.ARRAY:
		return evalArrayInfixExpression(operator, left, right)
	case left.Type() == object.MAP && right.Type() == object.MAP:
		return evalMapInfixExpression(operator, left, right)
	case operator == token.EQ:
		// should be left.Value() and right.Value() as currently this relies
		// on bool interning and ptr equality.
		return object.NativeBoolToBooleanObject(left == right)
	case operator == token.NOTEQ:
		return object.NativeBoolToBooleanObject(left != right)
	default:
		return object.Error{Value: "operation on non integers left=" + left.Inspect() + " right=" + right.Inspect()}
	}
}

func evalStringInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := left.(object.String).Value
	rightVal := right.(object.String).Value
	switch operator { //nolint:exhaustive // we have default.
	case token.EQ:
		return object.NativeBoolToBooleanObject(leftVal == rightVal)
	case token.NOTEQ:
		return object.NativeBoolToBooleanObject(leftVal != rightVal)
	case token.PLUS:
		return object.String{Value: leftVal + rightVal}
	default:
		return object.Error{Value: fmt.Sprintf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())}
	}
}

func evalArrayInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftVal := left.(object.Array).Elements
	switch operator { //nolint:exhaustive // we have default.
	case token.EQ:
		if right.Type() != object.ARRAY {
			return object.FALSE
		}
		rightVal := right.(object.Array).Elements
		return object.ArrayEquals(leftVal, rightVal)
	case token.NOTEQ:
		if right.Type() != object.ARRAY {
			return object.TRUE
		}
		rightVal := right.(object.Array).Elements
		if object.ArrayEquals(leftVal, rightVal) == object.FALSE {
			return object.TRUE
		}
		return object.FALSE
	case token.PLUS: // concat / append
		if right.Type() != object.ARRAY {
			return object.Array{Elements: append(leftVal, right)}
		}
		return object.Array{Elements: append(leftVal, right.(object.Array).Elements...)}
	default:
		return object.Error{Value: fmt.Sprintf("unknown operator: %s %s %s",
			left.Type(), operator, right.Type())}
	}
}

func evalMapInfixExpression(operator token.Type, left, right object.Object) object.Object {
	leftMap := left.(object.Map)
	rightMap := right.(object.Map)
	switch operator { //nolint:exhaustive // we have default.
	case token.EQ:
		return object.MapEquals(leftMap, rightMap)
	case token.NOTEQ:
		if object.MapEquals(leftMap, rightMap) == object.FALSE {
			return object.TRUE
		}
		return object.FALSE
	case token.PLUS: // concat / append
		res := object.NewMap()
		for k, v := range leftMap {
			res[k] = v
		}
		for k, v := range rightMap {
			res[k] = v
		}
		return res
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
	case token.LT:
		return object.NativeBoolToBooleanObject(leftVal < rightVal)
	case token.LTEQ:
		return object.NativeBoolToBooleanObject(leftVal <= rightVal)
	case token.GT:
		return object.NativeBoolToBooleanObject(leftVal > rightVal)
	case token.GTEQ:
		return object.NativeBoolToBooleanObject(leftVal >= rightVal)
	case token.EQ:
		return object.NativeBoolToBooleanObject(leftVal == rightVal)
	case token.NOTEQ:
		return object.NativeBoolToBooleanObject(leftVal != rightVal)
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
	case token.LT:
		return object.NativeBoolToBooleanObject(leftVal < rightVal)
	case token.LTEQ:
		return object.NativeBoolToBooleanObject(leftVal <= rightVal)
	case token.GT:
		return object.NativeBoolToBooleanObject(leftVal > rightVal)
	case token.GTEQ:
		return object.NativeBoolToBooleanObject(leftVal >= rightVal)
	case token.EQ:
		return object.NativeBoolToBooleanObject(leftVal == rightVal)
	case token.NOTEQ:
		return object.NativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return object.Error{Value: "unknown operator: " + operator.String()}
	}
}
