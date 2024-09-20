package ast

import (
	"reflect"
	"testing"
)

var (
	one  = func() Node { return &IntegerLiteral{Val: 1} }
	two  = func() Node { return &IntegerLiteral{Val: 2} }
	aone = one()
	atwo = two()

	turnOneIntoTwo = func(node Node) Node {
		integer, ok := node.(*IntegerLiteral)
		if !ok {
			return node
		}

		if integer.Val != 1 {
			return node
		}

		integer.Val = 2
		return integer
	}
)

func TestModifyNoOk(t *testing.T) {
	tests := []struct {
		input    Node
		expected Node
	}{
		{
			one(),
			two(),
		},
		{
			&Statements{
				Statements: []Node{
					&ReturnStatement{ReturnValue: one()},
				},
			},
			&Statements{
				Statements: []Node{
					&ReturnStatement{ReturnValue: two()},
				},
			},
		},
		{
			&InfixExpression{Left: one(), Right: two()},
			&InfixExpression{Left: two(), Right: two()},
		},
		{
			&InfixExpression{Left: two(), Right: one()},
			&InfixExpression{Left: two(), Right: two()},
		},
		{
			&PrefixExpression{Right: one()},
			&PrefixExpression{Right: two()},
		},
		{
			&IndexExpression{Left: one(), Index: one()},
			&IndexExpression{Left: two(), Index: two()},
		},
		{
			&IfExpression{
				Condition: one(),
				Consequence: &Statements{Statements: []Node{
					one(),
				}},
				Alternative: &Statements{Statements: []Node{
					one(),
				}},
			},
			&IfExpression{
				Condition: two(),
				Consequence: &Statements{Statements: []Node{
					two(),
				}},
				Alternative: &Statements{Statements: []Node{
					two(),
				}},
			},
		},
		{
			&ReturnStatement{ReturnValue: one()},
			&ReturnStatement{ReturnValue: two()},
		},
		{
			&FunctionLiteral{
				Parameters: []Node{&Identifier{}},
				Body: &Statements{Statements: []Node{
					one(),
				}},
			},
			&FunctionLiteral{
				Parameters: []Node{&Identifier{}},
				Body: &Statements{Statements: []Node{
					two(),
				}},
			},
		},
		{
			&ArrayLiteral{Elements: []Node{one(), one()}},
			&ArrayLiteral{Elements: []Node{two(), two()}},
		},
	}
	for _, tt := range tests {
		modified := ModifyNoOk(tt.input, turnOneIntoTwo)
		if !reflect.DeepEqual(modified, tt.expected) {
			t.Errorf("not equal.\n%#v\n-vs-\n%#v", modified, tt.expected)
		}
	}
}

func TestModifyMap(t *testing.T) {
	// need to test map separately because deepequal can't compare a map
	// with keys of the same underlying value (2:2, 2:2)
	modified := ModifyNoOk(
		&MapLiteral{
			Pairs: map[Node]Node{
				aone: one(),
				atwo: one(),
			},
			Order: []Node{atwo, aone},
		}, turnOneIntoTwo).(*MapLiteral)
	if len(modified.Order) != 2 {
		t.Errorf("Order length not 2. got=%d", len(modified.Order))
	}
	for i := range 2 {
		key := modified.Order[i]
		if key.(*IntegerLiteral).Val != 2 {
			t.Errorf("Order[%d] not 2. got=%v", i, modified.Order[i])
		}
		val := modified.Pairs[key]
		if val.(*IntegerLiteral).Val != 2 {
			t.Errorf("Pairs not modified for key %v -> %v", key, val)
		}
	}
}
