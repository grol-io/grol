package ast

import "fortio.org/log"

// Note, this is somewhat similar to eval.go's eval... both are "apply"ing.
func Modify(node Node, f func(Node) Node) Node {
	// TODO: add err checks for _s.
	switch node := node.(type) {
	case *Statements:
		for i, statement := range node.Statements {
			node.Statements[i] = Modify(statement, f)
		}
	case *InfixExpression:
		le := Modify(node.Left, f)
		node.Left = le
		re := Modify(node.Right, f)
		node.Right = re
	case *PrefixExpression:
		pe := Modify(node.Right, f)
		node.Right = pe
	case *IndexExpression:
		node.Left = Modify(node.Left, f)
		node.Index = Modify(node.Index, f)
	case *IfExpression:
		ce := Modify(node.Condition, f)
		node.Condition = ce
		node.Consequence = Modify(node.Consequence, f).(*Statements)
		if node.Alternative != nil {
			node.Alternative = Modify(node.Alternative, f).(*Statements)
		}
	case *ReturnStatement:
		re := Modify(node.ReturnValue, f)
		node.ReturnValue = re
	case *FunctionLiteral:
		for i := range node.Parameters {
			node.Parameters[i] = Modify(node.Parameters[i], f).(*Identifier)
		}
		node.Body = Modify(node.Body, f).(*Statements)
	case *ArrayLiteral:
		for i := range node.Elements {
			node.Elements[i] = Modify(node.Elements[i], f)
		}
	case *MapLiteral:
		newPairs := make(map[Node]Node)
		for key, val := range node.Pairs {
			newKey := Modify(key, f)
			newVal := Modify(val, f)
			newPairs[newKey] = newVal // TODO: bug, change Order too.
		}
		node.Pairs = newPairs
	default:
		log.LogVf("default for node type %T", node)
	}
	return f(node)
}
