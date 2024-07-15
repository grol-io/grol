package eval

import (
	"strconv"

	"fortio.org/log"
	"grol.io/grol/ast"
	"grol.io/grol/object"
	"grol.io/grol/token"
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
	// TODD: more types
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
	case object.Quote:
		return obj.Node
	default:
		log.Warnf("convertObjectToASTNode: unsupported object type %T", obj)
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
