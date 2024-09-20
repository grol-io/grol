package eval

import (
	"fmt"

	"fortio.org/log"
	"grol.io/grol/ast"
	"grol.io/grol/object"
	"grol.io/grol/token"
)

func (s *State) DefineMacros(programNode ast.Node) {
	program := programNode.(*ast.Statements) // panic if not a program is ok.
	for i := 0; i < len(program.Statements); /* not always incrementing */ {
		statement := program.Statements[i]
		if isMacroDefinition(statement) {
			addMacro(s.macroState, statement)
			program.Statements = append(program.Statements[:i], program.Statements[i+1:]...)
		} else {
			i++
		}
	}
}

func isAssign(node ast.Node) (*ast.InfixExpression, bool) {
	exp, ok := node.(*ast.InfixExpression)
	if ok && exp.Token == token.ByType(token.ASSIGN) {
		return exp, true
	}
	return nil, false
}

func isMacroDefinition(node ast.Node) bool {
	exp, ok := isAssign(node)
	if !ok {
		return false
	}
	_, ok = exp.Right.(*ast.MacroLiteral)
	return ok
}

func addMacro(s *object.Environment, stmt ast.Node) {
	// TODO ok checks
	assign, _ := stmt.(*ast.InfixExpression)
	macroLiteral, _ := assign.Right.(*ast.MacroLiteral)
	name := assign.Left.(*ast.Identifier).Literal()

	macro := &object.Macro{
		Parameters: macroLiteral.Parameters,
		Env:        s,
		Body:       macroLiteral.Body,
	}

	s.Set(name, macro)
}

func isMacroCall(s *object.Environment, exp *ast.CallExpression) (*object.Macro, bool) {
	identifier, ok := exp.Function.(*ast.Identifier)
	if !ok {
		return nil, false
	}

	obj, ok := s.Get(identifier.Literal())
	if !ok {
		return nil, false
	}

	macro, ok := obj.(*object.Macro)
	if !ok {
		return nil, false
	}

	return macro, true
}

func (s *State) MacroErrorf(fmtmsg string, args ...any) ast.Node {
	res := ast.Builtin{}
	res.Token = token.ByType(token.ERROR)
	msgNode := ast.StringLiteral{}
	msgNode.Token = token.Intern(token.STRING, fmt.Sprintf(fmtmsg, args...))
	res.Parameters = []ast.Node{&msgNode}
	return &res
}

func (s *State) ExpandMacros(program ast.Node) ast.Node {
	return ast.ModifyNoOk(program, func(node ast.Node) ast.Node {
		callExpression, ok := node.(*ast.CallExpression)
		if !ok {
			return node
		}

		macro, ok := isMacroCall(s.macroState, callExpression)
		if !ok {
			return node
		}

		args := quoteArgs(callExpression)
		if len(args) != len(macro.Parameters) {
			return s.MacroErrorf("wrong number of macro arguments, want=%d, got=%d", len(macro.Parameters), len(args))
		}

		evalEnv := extendMacroEnv(macro, args)

		evaluated := evalEnv.Eval(macro.Body)

		quote, ok := evaluated.(object.Quote)
		if !ok {
			estr := fmt.Sprintf("macro should return Quote. got=%T (%+v)", evaluated, evaluated)
			log.Critf("%s", estr)
			return s.MacroErrorf("%s", estr)
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
		extended.Set(param.Value().Literal(), args[paramIdx])
	}

	return &State{env: extended}
}
