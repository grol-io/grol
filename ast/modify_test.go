package ast

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestModify(t *testing.T) {
	one := func() Node { return IntegerLiteral{Val: 1} }
	two := func() Node { return IntegerLiteral{Val: 2} }

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
					&ReturnStatement{ReturnValue: one()},
				},
			},
			&Program{
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
				Consequence: &BlockStatement{Statements: []Node{
					one(),
				}},
				Alternative: &BlockStatement{Statements: []Node{
					one(),
				}},
			},
			&IfExpression{
				Condition: two(),
				Consequence: &BlockStatement{Statements: []Node{
					two(),
				}},
				Alternative: &BlockStatement{Statements: []Node{
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
				Body: &BlockStatement{Statements: []Node{
					one(),
				}},
			},
			&FunctionLiteral{
				Parameters: []Node{&Identifier{}},
				Body: &BlockStatement{Statements: []Node{
					two(),
				}},
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
		if !reflect.DeepEqual(modified, tt.expected) {
			t.Errorf("not equal.\n%#v\n-vs-\n%#v", modified, tt.expected)
		}
		if !cmp.Equal(modified, tt.expected) {
			t.Errorf("not equal. %v", cmp.Diff(modified, tt.expected))
		}
	}
}
