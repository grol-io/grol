package ast

import (
	"fmt"

	"fortio.org/log"
)

func ModifyNoOk(node Node, f func(Node) Node) Node {
	newNode, _ := Modify(node, func(n Node) (Node, bool) {
		return f(n), true
	})
	return newNode
}

// Note, this is somewhat similar to eval.go's eval... both are "apply"ing.
func Modify(node Node, f func(Node) (Node, bool)) (Node, bool) { //nolint:funlen,gocyclo,gocognit,maintidx // yeah lots of types.
	// It's quite ugly all these continuation/ok checks.
	var cont bool
	switch node := node.(type) {
	case *Statements:
		newNode := &Statements{Base: node.Base, Statements: make([]Node, len(node.Statements))}
		for i, statement := range node.Statements {
			newNode.Statements[i], cont = Modify(statement, f)
			if !cont {
				return nil, false
			}
		}
		return f(newNode)
	case *InfixExpression:
		newNode := &InfixExpression{Base: node.Base}
		newNode.Left, cont = Modify(node.Left, f)
		if !cont {
			return nil, false
		}
		newNode.Right, cont = Modify(node.Right, f)
		if !cont {
			return nil, false
		}
		return f(newNode)
	case *PrefixExpression:
		newNode := &PrefixExpression{Base: node.Base}
		newNode.Right, cont = Modify(node.Right, f)
		if !cont {
			return nil, false
		}
		return f(newNode)
	case *IndexExpression:
		newNode := &IndexExpression{Base: node.Base}
		newNode.Left, cont = Modify(node.Left, f)
		if !cont {
			return nil, false
		}
		newNode.Index, cont = Modify(node.Index, f)
		if !cont {
			return nil, false
		}
		return f(newNode)
	case *IfExpression:
		newNode := &IfExpression{Base: node.Base}
		newNode.Condition, cont = Modify(node.Condition, f)
		if !cont {
			return nil, false
		}
		nc, ok := Modify(node.Consequence, f)
		if !ok {
			return nil, false
		}
		newNode.Consequence = nc.(*Statements)
		if node.Alternative != nil {
			nc, ok = Modify(node.Alternative, f)
			if !ok {
				return nil, false
			}
			newNode.Alternative = nc.(*Statements)
		}
		return f(newNode)
	case *ForExpression:
		newNode := &ForExpression{Base: node.Base}
		newNode.Condition, cont = Modify(node.Condition, f)
		if !cont {
			return nil, false
		}
		nb, ok := Modify(node.Body, f)
		if !ok {
			return nil, false
		}
		newNode.Body = nb.(*Statements)
		return f(newNode)
	case *ReturnStatement:
		newNode := &ReturnStatement{Base: node.Base}
		if node.ReturnValue != nil {
			newNode.ReturnValue, cont = Modify(node.ReturnValue, f)
			if !cont {
				return nil, false
			}
		}
		return f(newNode)
	case *FunctionLiteral:
		newNode := *node
		newNode.Parameters = make([]Node, len(node.Parameters))
		for i := range node.Parameters {
			id, ok := Modify(node.Parameters[i], f)
			if !ok {
				return nil, false
			}
			newNode.Parameters[i] = id.(*Identifier)
		}
		nb, ok := Modify(node.Body, f)
		if !ok {
			return nil, false
		}
		newNode.Body = nb.(*Statements)
		return f(&newNode)
	case *ArrayLiteral:
		newNode := &ArrayLiteral{Base: node.Base, Elements: make([]Node, len(node.Elements))}
		for i := range node.Elements {
			newNode.Elements[i], cont = Modify(node.Elements[i], f)
			if !cont {
				return nil, false
			}
		}
		return f(newNode)
	case *MapLiteral:
		newNode := &MapLiteral{Base: node.Base, Pairs: make(map[Node]Node)}
		for _, key := range node.Order {
			val, ok := node.Pairs[key]
			if !ok {
				panic(fmt.Sprintf("key %v not in pairs for map %v", key, node))
			}
			newKey, ok := Modify(key, f)
			if !ok {
				return nil, false
			}
			newNode.Order = append(newNode.Order, newKey)
			newNode.Pairs[newKey], cont = Modify(val, f)
			if !cont {
				return nil, false
			}
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
			newNode.Parameters[i], cont = Modify(node.Parameters[i], f)
			if !cont {
				return nil, false
			}
		}
		return f(newNode)
	case *CallExpression:
		newNode := *node
		newNode.Arguments = make([]Node, len(node.Arguments))
		for i := range node.Arguments {
			newNode.Arguments[i], cont = Modify(node.Arguments[i], f)
			if !cont {
				return nil, false
			}
		}
		return f(&newNode)
	case *MacroLiteral:
		// modifying macro itself may be pointless but for completeness.
		newNode := *node
		newNode.Parameters = make([]Node, len(node.Parameters))
		for i := range node.Parameters {
			id, ok := Modify(node.Parameters[i], f)
			if !ok {
				return nil, false
			}
			newNode.Parameters[i] = id.(*Identifier)
		}
		nb, ok := Modify(node.Body, f)
		if !ok {
			return nil, false
		}
		newNode.Body = nb.(*Statements)
		return f(&newNode)
	default:
		log.Debugf("Modify not implemented for node type %T", node)
		return f(node)
		// panic(fmt.Sprintf("Modify not implemented for node type %T", node))
	}
}
