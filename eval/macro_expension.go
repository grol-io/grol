package eval

import (
	"log"

	"github.com/ldemailly/gorepl/ast"
	"github.com/ldemailly/gorepl/object"
)

func (s *State) DefineMacros(program *ast.Program) {
	for i := 0; i < len(program.Statements); /* not always incrementing */ {
		statement := program.Statements[i]
		if isMacroDefinition(statement) {
			s.addMacro(statement)
			program.Statements = append(program.Statements[:i], program.Statements[i+1:]...)
		} else {
			i++
		}
	}
}

func isMacroDefinition(node ast.Node) bool {
	letStatement, ok := node.(*ast.LetStatement)
	if !ok {
		return false
	}

	_, ok = letStatement.Value.(*ast.MacroLiteral)
	return ok
}

func (s *State) addMacro(stmt ast.Node) {
	letStatement, _ := stmt.(*ast.LetStatement)
	macroLiteral, _ := letStatement.Value.(*ast.MacroLiteral)

	macro := &object.Macro{
		Parameters: macroLiteral.Parameters,
		Env:        s.env,
		Body:       macroLiteral.Body,
	}

	s.env.Set(letStatement.Name.Val, macro)
}

func (s *State) isMacroCall(exp *ast.CallExpression) (*object.Macro, bool) {
	identifier, ok := exp.Function.(*ast.Identifier)
	if !ok {
		return nil, false
	}

	obj, ok := s.env.Get(identifier.Val)
	if !ok {
		return nil, false
	}

	macro, ok := obj.(*object.Macro)
	if !ok {
		return nil, false
	}

	return macro, true
}

func (s *State) ExpandMacros(program ast.Node) ast.Node {
	return ast.Modify(program, func(node ast.Node) ast.Node {
		callExpression, ok := node.(*ast.CallExpression)
		if !ok {
			return node
		}

		macro, ok := s.isMacroCall(callExpression)
		if !ok {
			return node
		}

		args := quoteArgs(callExpression)
		evalEnv := extendMacroEnv(macro, args)

		evaluated := evalEnv.Eval(macro.Body)

		quote, ok := evaluated.(object.Quote)
		if !ok {
			log.Fatalf("We only support returning AST-nodes from macros, got %T", evaluated)
		}
		return quote.Node
	})
}

func quoteArgs(exp *ast.CallExpression) []object.Quote {
	args := []object.Quote{}

	for _, a := range exp.Arguments {
		args = append(args, object.Quote{Node: a})
	}

	return args
}

func extendMacroEnv(macro *object.Macro, args []object.Quote) *State {
	extended := object.NewEnclosedEnvironment(macro.Env)

	for paramIdx, param := range macro.Parameters {
		extended.Set(param.Val, args[paramIdx])
	}

	return &State{env: extended}
}
