package ast

// Note, this is somewhat similar to eval.go's eval... both are "apply"ing.
func Modify(node Node, f func(Node) Node) Node {
	// TODO: add err checks for _s.
	switch node := node.(type) {
	case *Program:
		for i, statement := range node.Statements {
			node.Statements[i] = Modify(statement, f)
		}
	case *ExpressionStatement:
		node.Val, _ = Modify(node.Val, f).(Expression)
	case *InfixExpression:
		node.Left, _ = Modify(node.Left, f).(Expression)
		node.Right, _ = Modify(node.Right, f).(Expression)
	case *PrefixExpression:
		node.Right, _ = Modify(node.Right, f).(Expression)
	case *IndexExpression:
		node.Left, _ = Modify(node.Left, f).(Expression)
		node.Index, _ = Modify(node.Index, f).(Expression)
	case *IfExpression:
		node.Condition, _ = Modify(node.Condition, f).(Expression)
		node.Consequence, _ = Modify(node.Consequence, f).(*BlockStatement)
		if node.Alternative != nil {
			node.Alternative, _ = Modify(node.Alternative, f).(*BlockStatement)
		}
	case *BlockStatement:
		for i := range node.Statements {
			node.Statements[i] = Modify(node.Statements[i], f)
		}
	case *ReturnStatement:
		node.ReturnValue, _ = Modify(node.ReturnValue, f).(Expression)
	case *FunctionLiteral:
		for i := range node.Parameters {
			node.Parameters[i], _ = Modify(node.Parameters[i], f).(*Identifier)
		}
		node.Body, _ = Modify(node.Body, f).(*BlockStatement)
	case *ArrayLiteral:
		for i := range node.Elements {
			node.Elements[i], _ = Modify(node.Elements[i], f).(Expression)
		}
	case *MapLiteral:
		newPairs := make(map[Expression]Expression)
		for key, val := range node.Pairs {
			newKey, _ := Modify(key, f).(Expression)
			newVal, _ := Modify(val, f).(Expression)
			newPairs[newKey] = newVal
		}
		node.Pairs = newPairs
	}
	return f(node)
}
