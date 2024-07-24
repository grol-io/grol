package ast

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestModify(t *testing.T) {
	one := func() Node { return &IntegerLiteral{Val: 1} }
	two := func() Node { return &IntegerLiteral{Val: 2} }

	turnOneIntoTwo := func(node Node) Node {
		integer, ok := node.(IntegerLiteral)
		if !ok {
			return node
		}

		if integer.Val != 1 {
			return node
		}

		integer.Val = 2
		return integer
	}

	tests := []struct {
		input    Node
		expected Node
	}{
		{
			one(),
			two(),
		},
		{
			&Program{
				Statements: []Node{
					&ExpressionStatement{Val: one()},
				},
			},
			&Program{
				Statements: []Node{
					&ExpressionStatement{Val: two()},
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
				Consequence: &BlockStatement{
					Program: Program{Statements: []Node{
						&ExpressionStatement{Val: one()},
					}},
				},
				Alternative: &BlockStatement{
					Program: Program{Statements: []Node{
						&ExpressionStatement{Val: one()},
					}},
				},
			},
			&IfExpression{
				Condition: two(),
				Consequence: &BlockStatement{
					Program: Program{Statements: []Node{
						&ExpressionStatement{Val: two()},
					}},
				},
				Alternative: &BlockStatement{
					Program: Program{Statements: []Node{
						&ExpressionStatement{Val: two()},
					}},
				},
			},
		},
		{
			&ReturnStatement{ReturnValue: one()},
			&ReturnStatement{ReturnValue: two()},
		},
		{
			&FunctionLiteral{
				Parameters: []Node{},
				Body: &BlockStatement{
					Program: Program{Statements: []Node{
						&ExpressionStatement{Val: one()},
					}},
				},
			},
			&FunctionLiteral{
				Parameters: []Node{},
				Body: &BlockStatement{
					Program: Program{Statements: []Node{
						&ExpressionStatement{Val: two()},
					}},
				},
			},
		},
		{
			&ArrayLiteral{Elements: []Node{one(), one()}},
			&ArrayLiteral{Elements: []Node{two(), two()}},
		},
		{
			&MapLiteral{Pairs: map[Node]Node{
				one(): one(),
				one(): one(),
			}},
			&MapLiteral{Pairs: map[Node]Node{
				two(): two(),
				two(): two(),
			}},
		},
	}
	for _, tt := range tests {
		modified := Modify(tt.input, turnOneIntoTwo)
		if !cmp.Equal(modified, tt.expected) {
			t.Errorf("not equal. %v", cmp.Diff(modified, tt.expected))
		}
	}
}
