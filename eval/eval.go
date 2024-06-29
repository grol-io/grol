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

// TODO: don't call the .String() if log level isn't verbose.

func Eval(node any) object.Object {
	switch node := node.(type) {
	// Statements
	case *ast.Program:
		log.LogVf("eval program")
		return evalStatements(node.Statements)

	case *ast.ExpressionStatement:
		log.LogVf("eval expr statement")
		return Eval(node.Val)

	case *ast.BlockStatement:
		if node == nil { // TODO: only here? this comes from empty else branches.
			return NULL
		}
		log.LogVf("eval block statement")
		return evalStatements(node.Statements)

	case *ast.IfExpression:
		return evalIfExpression(node)

		// Expressions
	case *ast.PrefixExpression:
		log.LogVf("eval prefix %s", node.String())
		right := Eval(node.Right)
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		log.LogVf("eval infix %s", node.String())
		left := Eval(node.Left)
		right := Eval(node.Right)
		return evalInfixExpression(node.Operator, left, right)

	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Val}

	case *ast.Boolean:
		return nativeBoolToBooleanObject(node.Val)
	}

	return &object.Error{Value: fmt.Sprintf("unknown node type: %T", node)}
}

func evalIfExpression(ie *ast.IfExpression) object.Object {
	condition := Eval(ie.Condition)
	switch condition {
	case TRUE:
		log.LogVf("if %s is TRUE, picking true branch", ie.Condition.String())
		return Eval(ie.Consequence)
	case FALSE:
		log.LogVf("if %s is FALSE, picking else branch", ie.Condition.String())
		return Eval(ie.Alternative)
	default:
		return &object.Error{Value: "<condition is not a boolean: " + condition.Inspect() + ">"}
	}
}

func evalStatements(stmts []ast.Node) object.Object {
	var result object.Object
	result = NULL // no crash when empty program.

	for _, statement := range stmts {
		result = Eval(statement)
	}

	return result
}

func nativeBoolToBooleanObject(input bool) *object.Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return &object.Error{Value: "unknown operator: " + operator}
	}
}

func evalBangOperatorExpression(right object.Object) object.Object {
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

func evalMinusPrefixOperatorExpression(right object.Object) object.Object {
	if right.Type() != object.INTEGER {
		return &object.Error{Value: "<minus of " + right.Inspect() + ">"}
	}

	value := right.(*object.Integer).Value
	return &object.Integer{Value: -value}
}

func evalInfixExpression(
	operator string,
	left, right object.Object,
) object.Object {
	switch {
	case left.Type() == object.INTEGER && right.Type() == object.INTEGER:
		return evalIntegerInfixExpression(operator, left, right)
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

func evalIntegerInfixExpression(
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
