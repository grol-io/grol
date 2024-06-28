package eval

import (
	"fmt"

	"github.com/ldemailly/gorepl/ast"
	"github.com/ldemailly/gorepl/object"
)

func Eval(node any) object.Object {
	switch node := node.(type) {
	// Statements
	case *ast.Program:
		return evalStatements(node.Statements)

	case *ast.ExpressionStatement:
		return Eval(node.Val)

	// Expressions
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Val}
	}

	return &object.Error{Value: fmt.Sprintf("unknown node type: %T", node)}
}

func evalStatements(stmts []ast.Node) object.Object {
	var result object.Object
	result = &object.Null{} // no crash when empty program.

	for _, statement := range stmts {
		result = Eval(statement)
	}

	return result
}
