package eval

import (
	"strconv"

	"github.com/ldemailly/gorepl/ast"
	"github.com/ldemailly/gorepl/object"
	"github.com/ldemailly/gorepl/token"
)

func (s *State) quote(node ast.Node) object.Quote {
	node = s.evalUnquoteCalls(node)
	return object.Quote{Node: node}
}

func (s *State) evalUnquoteCalls(quoted ast.Node) ast.Node {
	return ast.Modify(quoted, func(node ast.Node) ast.Node {
		if !isUnquoteCall(node) {
			return node
		}

		call, ok := node.(*ast.CallExpression)
		if !ok {
			return node
		}

		if len(call.Arguments) != 1 {
			return node
		}
		unquoted := s.evalInternal(call.Arguments[0])
		return convertObjectToASTNode(unquoted)
	})
}

// feels like we should merge ast and object and avoid these?
func convertObjectToASTNode(obj object.Object) ast.Node {
	switch obj := obj.(type) {
	case object.Integer:
		t := token.Token{
			Type:    token.INT,
			Literal: strconv.FormatInt(obj.Value, 10),
		}
		r := ast.IntegerLiteral{Val: obj.Value}
		r.Token = t
		return r
	case object.Boolean:
		var t token.Token
		if obj.Value {
			t = token.Token{Type: token.TRUE, Literal: "true"}
		} else {
			t = token.Token{Type: token.FALSE, Literal: "false"}
		}
		return ast.Boolean{Base: ast.Base{Token: t}, Val: obj.Value}
	default:
		return nil
	}
}

func isUnquoteCall(node ast.Node) bool {
	callExpression, ok := node.(*ast.CallExpression)
	if !ok {
		return false
	}

	return callExpression.Function.TokenLiteral() == "unquote"
}
