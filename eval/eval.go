package eval

import (
	"fmt"

	"fortio.org/log"
	"github.com/ldemailly/gorepl/ast"
	"github.com/ldemailly/gorepl/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
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
			return NULL
		}
		log.LogVf("eval block statement")
		return s.evalStatements(node.Statements)

	case *ast.IfExpression:
		return s.evalIfExpression(node)
		// assignement
	case *ast.LetStatement:
		val := s.Eval(node.Value)
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
		left := s.evalInternal(node.Left)
		right := s.evalInternal(node.Right)
		return s.evalInfixExpression(node.Operator, left, right)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Val}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Val)

	case *ast.ReturnStatement:
		val := s.evalInternal(node.ReturnValue)
		return &object.ReturnValue{Value: val}

	case *ast.FunctionLiteral:
		params := node.Parameters
		body := node.Body
		return &object.Function{Parameters: params, Env: s.env, Body: body}
	case *ast.CallExpression:
		f := s.Eval(node.Function)
		args, oerr := s.evalExpressions(node.Arguments)
		if oerr != nil {
			return oerr
		}
		return s.applyFunction(f, args)
	}
	return &object.Error{Value: fmt.Sprintf("unknown node type: %T", node)}
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
	res := s.Eval(function.Body)
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
		evaluated := s.Eval(e)
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
	case TRUE:
		log.LogVf("if %s is TRUE, picking true branch", ie.Condition.String())
		return s.evalInternal(ie.Consequence)
	case FALSE:
		log.LogVf("if %s is FALSE, picking else branch", ie.Condition.String())
		return s.evalInternal(ie.Alternative)
	default:
		return &object.Error{Value: "<condition is not a boolean: " + condition.Inspect() + ">"}
	}
}

func (s *State) evalStatements(stmts []ast.Node) object.Object {
	var result object.Object
	result = NULL // no crash when empty program.
	for _, statement := range stmts {
		result = s.evalInternal(statement)
		if rt := result.Type(); rt == object.RETURN || rt == object.ERROR {
			return result
		}
	}
	return result
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
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
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return &object.Error{Value: "<not of NULL>"}
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

func (s *State) evalInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	switch {
	case left.Type() == object.INTEGER && right.Type() == object.INTEGER:
		return s.evalIntegerInfixExpression(operator, left, right)
	case operator == "==":
		// should be left.Value() and right.Value() as currently this relies
		// on bool interning and ptr equality.
		return nativeBoolToBooleanObject(left == right)
	case operator == "!=":
		return nativeBoolToBooleanObject(left != right)
	default:
		return &object.Error{Value: "<operation on non integers left=" + left.Inspect() + " right=" + right.Inspect() + ">"}
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
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return &object.Error{Value: "unknown operator: " + operator}
	}
}
