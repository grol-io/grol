package eval

import (
	"fmt"

	"fortio.org/log"
	"github.com/ldemailly/gorepl/ast"
	"github.com/ldemailly/gorepl/object"
	"github.com/ldemailly/gorepl/token"
)

type State struct {
	env *object.Environment
}

func NewState() *State {
	return &State{env: object.NewEnvironment()}
}

// TODO: don't call the .String() if log level isn't verbose.

func (s *State) Eval(node any) object.Object {
	result := s.evalInternal(node)
	// unwrap return values only at the top.
	if returnValue, ok := result.(*object.ReturnValue); ok {
		return returnValue.Value
	}
	return result
}

func (s *State) evalAssignment(right object.Object, node *ast.InfixExpression) object.Object {
	// let free assignments.
	id, ok := node.Left.(*ast.Identifier)
	if !ok {
		return &object.Error{Value: "<assignment to non identifier: " + node.Left.String() + ">"}
	}
	if rt := right.Type(); rt == object.ERROR {
		log.Warnf("can't assign %q: %v", right.Inspect(), right)
		return right
	}
	log.LogVf("eval assign %#v to %#v", right, id.Val)
	s.env.Set(id.Val, right)
	return right // maybe only if it's a literal?
}

func ArgCheck[T any](msg string, n int, args []T) *object.Error {
	if len(args) != n {
		return &object.Error{Value: fmt.Sprintf("%s: wrong number of arguments. got=%d, want=%d", msg, len(args), n)}
	}
	return nil
}

func (s *State) evalInternal(node any) object.Object {
	switch node := node.(type) {
	// Statements
	case *ast.Program:
		log.LogVf("eval program")
		return s.evalStatements(node.Statements)

	case *ast.ExpressionStatement:
		log.LogVf("eval expr statement")
		return s.evalInternal(node.Val)

	case *ast.BlockStatement:
		if node == nil { // TODO: only here? this comes from empty else branches.
			return object.NULL
		}
		log.LogVf("eval block statement")
		return s.evalStatements(node.Statements)

	case *ast.IfExpression:
		return s.evalIfExpression(node)
		// assignement
	case *ast.LetStatement:
		val := s.evalInternal(node.Value)
		if rt := val.Type(); rt == object.ERROR {
			log.Warnf("can't eval %q: %v", node.String(), val)
			return val
		}
		log.LogVf("eval let %s to %#v", node.Name.Val, val)
		s.env.Set(node.Name.Val, val)
		return val // maybe only if it's a literal?
		// Expressions
	case *ast.Identifier:
		return s.evalIdentifier(node)
	case *ast.PrefixExpression:
		log.LogVf("eval prefix %s", node.String())
		right := s.evalInternal(node.Right)
		return s.evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		log.LogVf("eval infix %s", node.String())
		right := s.evalInternal(node.Right)
		if node.Operator == "=" {
			return s.evalAssignment(right, node)
		}
		left := s.evalInternal(node.Left)
		return s.evalInfixExpression(node.Operator, left, right)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Val}

	case *ast.Boolean:
		return object.NativeBoolToBooleanObject(node.Val)

	case *ast.StringLiteral:
		return &object.String{Value: node.Val}

	case *ast.ReturnStatement:
		val := s.evalInternal(node.ReturnValue)
		return &object.ReturnValue{Value: val}
	case *ast.Builtin:
		return s.evalBuiltin(node)
	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: s.env, Body: body}
	case *ast.CallExpression:
		f := s.evalInternal(node.Function)
		args, oerr := s.evalExpressions(node.Arguments)
		if oerr != nil {
			return oerr
		}
		return s.applyFunction(f, args)
	case *ast.ArrayLiteral:
		elements, objerr := s.evalExpressions(node.Elements)
		if objerr != nil {
			return objerr
		}
		return &object.Array{Elements: elements}
	case *ast.IndexExpression:
		left := s.evalInternal(node.Left)
		index := s.evalInternal(node.Index)
		return evalIndexExpression(left, index)
	}
	return &object.Error{Value: fmt.Sprintf("unknown node type: %T", node)}
}

func (s *State) evalBuiltin(node *ast.Builtin) object.Object {
	// so far all 3 have exactly 1 argument.
	if oerr := ArgCheck(node.Literal, 1, node.Parameters); oerr != nil {
		return oerr
	}
	val := s.evalInternal(node.Parameters[0])
	rt := val.Type()
	if rt == object.ERROR {
		return val
	}
	arr, _ := val.(*object.Array)
	switch node.Type { //nolint:exhaustive // we have default, only 2 cases.
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
		return &object.Array{Elements: arr.Elements[1:]}
	case token.LEN:
		switch rt { //nolint:exhaustive // we have default, len doesn't work on many types.
		case object.STRING:
			return &object.Integer{Value: int64(len(val.(*object.String).Value))}
		case object.ARRAY:
			return &object.Integer{Value: int64(len(arr.Elements))}
		case object.NIL:
			return &object.Integer{Value: 0}
		}
	default:
		return &object.Error{Value: fmt.Sprintf("builtin %s yet implemented", node.Type)}
	}
	return &object.Error{Value: node.Literal + ": not supported on " + rt.String()}
}

func evalIndexExpression(left, index object.Object) object.Object {
	switch {
	case left.Type() == object.ARRAY && index.Type() == object.INTEGER:
		return evalArrayIndexExpression(left, index)
	default:
		return &object.Error{Value: "index operator not supported: " + left.Type().String() + "[" + index.Type().String() + "]"}
	}
}

func evalArrayIndexExpression(array, index object.Object) object.Object {
	arrayObject := array.(*object.Array)
	idx := index.(*object.Integer).Value
	max := int64(len(arrayObject.Elements) - 1)

	if idx < 0 || idx > max {
		return object.NULL
	}
	return arrayObject.Elements[idx]
}

func (s *State) applyFunction(fn object.Object, args []object.Object) object.Object {
	function, ok := fn.(*object.Function)
	if !ok {
		return &object.Error{Value: "<not a function: " + fn.Type().String() + ">"}
	}
	nenv, oerr := extendFunctionEnv(function, args)
	if oerr != nil {
		return oerr
	}
	curState := s.env
	s.env = nenv
	res := s.evalInternal(function.Body)
	// restore the previous env/state.
	s.env = curState
	return res
}

func extendFunctionEnv(fn *object.Function, args []object.Object) (*object.Environment, *object.Error) {
	env := object.NewEnclosedEnvironment(fn.Env)
	n := len(fn.Parameters)
	if len(args) != n {
		// TODO: would be nice with the function name.
		return nil, &object.Error{Value: fmt.Sprintf("<wrong number of arguments. got=%d, want=%d>",
			len(args), n)}
	}
	for paramIdx, param := range fn.Parameters {
		env.Set(param.Val, args[paramIdx])
	}

	return env, nil
}

// TODO: isn't this same as statements?
func (s *State) evalExpressions(exps []ast.Expression) ([]object.Object, *object.Error) {
	result := make([]object.Object, 0, len(exps))
	for _, e := range exps {
		evaluated := s.evalInternal(e)
		if rt := evaluated.Type(); rt == object.ERROR {
			return nil, evaluated.(*object.Error)
		}
		result = append(result, evaluated)
	}
	return result, nil
}

func (s *State) evalIdentifier(node *ast.Identifier) object.Object {
	val, ok := s.env.Get(node.Val)
	if !ok {
		return &object.Error{Value: "<identifier not found: " + node.Val + ">"}
	}
	return val
}

func (s *State) evalIfExpression(ie *ast.IfExpression) object.Object {
	condition := s.evalInternal(ie.Condition)
	switch condition {
	case object.TRUE:
		log.LogVf("if %s is object.TRUE, picking true branch", ie.Condition.String())
		return s.evalInternal(ie.Consequence)
	case object.FALSE:
		log.LogVf("if %s is object.FALSE, picking else branch", ie.Condition.String())
		return s.evalInternal(ie.Alternative)
	default:
		return &object.Error{Value: "<condition is not a boolean: " + condition.Inspect() + ">"}
	}
}

func (s *State) evalStatements(stmts []ast.Node) object.Object {
	var result object.Object
	result = object.NULL // no crash when empty program.
	for _, statement := range stmts {
		result = s.evalInternal(statement)
		if rt := result.Type(); rt == object.RETURN || rt == object.ERROR {
			return result
		}
	}
	return result
}

func (s *State) evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return s.evalBangOperatorExpression(right)
	case "-":
		return s.evalMinusPrefixOperatorExpression(right)
	default:
		return &object.Error{Value: "unknown operator: " + operator}
	}
}

func (s *State) evalBangOperatorExpression(right object.Object) object.Object {
	switch right {
	case object.TRUE:
		return object.FALSE
	case object.FALSE:
		return object.TRUE
	case object.NULL:
		return &object.Error{Value: "<not of object.NULL>"}
	default:
		return &object.Error{Value: "<not of " + right.Inspect() + ">"}
	}
}

func (s *State) evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() != object.INTEGER {
		return &object.Error{Value: "<minus of " + right.Inspect() + ">"}
	}

	value := right.(*object.Integer).Value
	return &object.Integer{Value: -value}
}

func (s *State) evalInfixExpression(operator string, left, right object.Object) object.Object {
	switch {
	case left.Type() == object.INTEGER && right.Type() == object.INTEGER:
		return s.evalIntegerInfixExpression(operator, left, right)
	case left.Type() == object.STRING && right.Type() == object.STRING:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == object.ARRAY:
		return evalArrayInfixExpression(operator, left, right)
	case operator == "==":
		// should be left.Value() and right.Value() as currently this relies
		// on bool interning and ptr equality.
		return object.NativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return object.NativeBoolToBooleanObject(left != right)
	default:
		return &object.Error{Value: "<operation on non integers left=" + left.Inspect() + " right=" + right.Inspect() + ">"}
	}
}

func evalStringInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.String).Value
	rightVal := right.(*object.String).Value
	switch operator {
	case "==":
		return object.NativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return object.NativeBoolToBooleanObject(leftVal != rightVal)
	case "+":
		return &object.String{Value: leftVal + rightVal}
	default:
		return &object.Error{Value: fmt.Sprintf("<unknown operator: %s %s %s>",
			left.Type(), operator, right.Type())}
	}
}

func evalArrayInfixExpression(operator string, left, right object.Object) object.Object {
	leftVal := left.(*object.Array).Elements
	switch operator {
	case "==":
		if right.Type() != object.ARRAY {
			return object.FALSE
		}
		rightVal := right.(*object.Array).Elements
		return object.ArrayEquals(leftVal, rightVal)
	case "!=":
		if right.Type() != object.ARRAY {
			return object.TRUE
		}
		rightVal := right.(*object.Array).Elements
		if object.ArrayEquals(leftVal, rightVal) == object.FALSE {
			return object.TRUE
		}
		return object.FALSE
	case "+": // concat / append
		if right.Type() != object.ARRAY {
			return &object.Array{Elements: append(leftVal, right)}
		}
		return &object.Array{Elements: append(leftVal, right.(*object.Array).Elements...)}
	default:
		return &object.Error{Value: fmt.Sprintf("<unknown operator: %s %s %s>",
			left.Type(), operator, right.Type())}
	}
}

func (s *State) evalIntegerInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	leftVal := left.(*object.Integer).Value
	rightVal := right.(*object.Integer).Value

	switch operator {
	case "+":
		return &object.Integer{Value: leftVal + rightVal}
	case "-":
		return &object.Integer{Value: leftVal - rightVal}
	case "*":
		return &object.Integer{Value: leftVal * rightVal}
	case "/":
		return &object.Integer{Value: leftVal / rightVal}
	case "%":
		return &object.Integer{Value: leftVal % rightVal}
	case "<":
		return object.NativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return object.NativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return object.NativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return object.NativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return &object.Error{Value: "unknown operator: " + operator}
	}
}
