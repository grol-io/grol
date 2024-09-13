package ast

import "fmt"

// Note, this is somewhat similar to eval.go's eval... both are "apply"ing.
func Modify(node Node, f func(Node) Node) Node { //nolint:funlen // yeah lots of types.
	// TODO: add err checks for _s.
	switch node := node.(type) {
	case *Statements:
		newNode := &Statements{Base: node.Base, Statements: make([]Node, len(node.Statements))}
		for i, statement := range node.Statements {
			newNode.Statements[i] = Modify(statement, f)
		}
		return f(newNode)
	case *InfixExpression:
		newNode := &InfixExpression{Base: node.Base}
		newNode.Left = Modify(node.Left, f)
		newNode.Right = Modify(node.Right, f)
		return f(newNode)
	case *PrefixExpression:
		newNode := &PrefixExpression{Base: node.Base}
		newNode.Right = Modify(node.Right, f)
		return f(newNode)
	case *IndexExpression:
		newNode := &IndexExpression{Base: node.Base}
		newNode.Left = Modify(node.Left, f)
		newNode.Index = Modify(node.Index, f)
		return f(newNode)
	case *IfExpression:
		newNode := &IfExpression{Base: node.Base}
		newNode.Condition = Modify(node.Condition, f)
		newNode.Consequence = Modify(node.Consequence, f).(*Statements)
		if node.Alternative != nil {
			newNode.Alternative = Modify(node.Alternative, f).(*Statements)
		}
		return f(newNode)
	case *ForExpression:
		newNode := &ForExpression{Base: node.Base}
		newNode.Condition = Modify(node.Condition, f)
		newNode.Body = Modify(node.Body, f).(*Statements)
		return f(newNode)
	case *ReturnStatement:
		newNode := &ReturnStatement{Base: node.Base}
		newNode.ReturnValue = Modify(node.ReturnValue, f)
		return f(newNode)
	case *FunctionLiteral:
		newNode := *node
		newNode.Parameters = make([]Node, len(node.Parameters))
		for i := range node.Parameters {
			newNode.Parameters[i] = Modify(node.Parameters[i], f).(*Identifier)
		}
		newNode.Body = Modify(node.Body, f).(*Statements)
		return f(&newNode)
	case *ArrayLiteral:
		newNode := &ArrayLiteral{Base: node.Base, Elements: make([]Node, len(node.Elements))}
		for i := range node.Elements {
			newNode.Elements[i] = Modify(node.Elements[i], f)
		}
		return f(newNode)
	case *MapLiteral:
		newNode := &MapLiteral{Base: node.Base, Pairs: make(map[Node]Node)}
		for _, key := range node.Order {
			val, ok := node.Pairs[key]
			if !ok {
				panic(fmt.Sprintf("key %v not in pairs for map %v", key, node))
			}
			newKey := Modify(key, f)
			newNode.Order = append(newNode.Order, newKey)
			newNode.Pairs[newKey] = Modify(val, f)
		}
		return f(newNode)
	case *Identifier:
		n := *node
		return f(&n) // silly go optimizes &(*node) to node (ptr) so need 2 steps
	case *IntegerLiteral:
		n := node
		return f(n)
	case *FloatLiteral:
		n := *node
		return f(&n)
	case *StringLiteral:
		n := *node
		return f(&n)
	case *Boolean:
		n := *node
		return f(&n)
	case *Comment:
		n := *node
		return f(&n)
	case *ControlExpression:
		n := *node
		return f(&n)
	case *PostfixExpression:
		n := *node
		return f(&n)
	case *Builtin:
		newNode := &Builtin{Base: node.Base, Parameters: make([]Node, len(node.Parameters))}
		for i := range node.Parameters {
			newNode.Parameters[i] = Modify(node.Parameters[i], f)
		}
		return f(newNode)
	case *CallExpression:
		newNode := *node
		newNode.Arguments = make([]Node, len(node.Arguments))
		for i := range node.Arguments {
			newNode.Arguments[i] = Modify(node.Arguments[i], f)
		}
		return f(&newNode)
	case *MacroLiteral:
		// modifying macro itself may be pointless but for completeness.
		newNode := *node
		newNode.Parameters = make([]Node, len(node.Parameters))
		for i := range node.Parameters {
			newNode.Parameters[i] = Modify(node.Parameters[i], f).(*Identifier)
		}
		newNode.Body = Modify(node.Body, f).(*Statements)
		return f(&newNode)
	default:
		panic(fmt.Sprintf("Modify not implemented for node type %T", node))
	}
}
