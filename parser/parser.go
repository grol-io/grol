package parser

import (
	"fmt"
	"strconv"

	"fortio.org/log"
	"grol.io/grol/ast"
	"grol.io/grol/lexer"
	"grol.io/grol/token"
)

type Priority int8

const (
	_ Priority = iota
	LOWEST
	ASSIGN      // =
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
	INDEX       // array[index]
)

//go:generate stringer -type=Priority
var _ = CALL.String() // force compile error if go generate is missing.

type (
	prefixParseFn func() ast.Node
	infixParseFn  func(ast.Node) ast.Node
)

type Parser struct {
	l *lexer.Lexer

	curToken  *token.Token
	peekToken *token.Token

	errors             []string
	continuationNeeded bool

	prefixParseFns map[token.Type]prefixParseFn
	infixParseFns  map[token.Type]infixParseFn
}

func (p *Parser) ContinuationNeeded() bool {
	return p.continuationNeeded
}

func (p *Parser) registerPrefix(t token.Type, fn prefixParseFn) {
	p.prefixParseFns[t] = fn
}

func (p *Parser) registerInfix(t token.Type, fn infixParseFn) {
	p.infixParseFns[t] = fn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[token.Type]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier) // arguable that ident/ints are prefixes - they are absence of operator?
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.FUNC, p.parseFunctionLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.LEN, p.parseBuiltin)
	p.registerPrefix(token.FIRST, p.parseBuiltin)
	p.registerPrefix(token.REST, p.parseBuiltin)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(token.LBRACE, p.parseMapLiteral)
	p.registerPrefix(token.LINECOMMENT, p.parseComment)
	p.registerPrefix(token.PRINT, p.parseBuiltin)
	p.registerPrefix(token.LOG, p.parseBuiltin)
	p.registerPrefix(token.MACRO, p.parseMacroLiteral)
	p.registerPrefix(token.ERROR, p.parseBuiltin)

	p.infixParseFns = make(map[token.Type]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOTEQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.LTEQ, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.GTEQ, p.parseInfixExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexExpression)

	// no let:
	p.registerInfix(token.ASSIGN, p.parseInfixExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Node{}

	for p.curToken.Type() != token.EOF && p.curToken.Type() != token.EOL {
		stmt := p.parseStatement()
		program.Statements = append(program.Statements, stmt)
		p.nextToken()
	}

	return program
}

func (p *Parser) parseArrayLiteral() ast.Node {
	array := &ast.ArrayLiteral{}
	array.Token = p.curToken

	array.Elements = p.parseExpressionList(token.RBRACKET)

	return array
}

func (p *Parser) parseStringLiteral() ast.Node {
	r := &ast.StringLiteral{}
	r.Token = p.curToken
	return r
}

func (p *Parser) parseComment() ast.Node {
	r := &ast.Comment{}
	r.Token = p.curToken
	return r
}

func (p *Parser) parseStatement() ast.Node {
	switch p.curToken.Type() { //nolint:exhaustive // we're not done yet TODO: remove.
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		stmt := p.parseExpression(LOWEST)
		if p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		return stmt
	}
}

func (p *Parser) parseReturnStatement() ast.Node {
	stmt := &ast.ReturnStatement{}
	stmt.Token = p.curToken

	// hacky for empty expressions like plain `return`.
	if p.peekTokenIs(token.SEMICOLON) || p.peekTokenIs(token.RBRACE) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.EOL) {
		log.Debugf("parseExpression: %s returning nil", p.curToken)
		// nil return value
		return stmt
	}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func sameToken(msg string, actual *token.Token, expected token.Type) bool {
	res := actual.Type() == expected
	if res {
		log.Debugf("%sTokenIs indeed: %s", msg, actual)
	} else {
		log.LogVf("%sTokenIs not: %s - found %s/%s instead", msg, expected, actual.Type(), actual.Literal())
	}
	return res
}

func (p *Parser) curTokenIs(t token.Type) bool {
	return sameToken("cur", p.curToken, t)
}

func (p *Parser) peekTokenIs(t token.Type) bool {
	return sameToken("peek", p.peekToken, t)
}

func (p *Parser) expectPeek(t token.Type) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	if p.peekTokenIs(token.EOL) {
		p.continuationNeeded = true
		return false
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t token.Type) {
	msg := fmt.Sprintf("expected next token to be %s, got %s (%q) instead",
		t, p.peekToken.Type(), p.peekToken.Literal())
	p.errors = append(p.errors, msg)
}

func (p *Parser) noPrefixParseFnError(t token.Type) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseExpression(precedence Priority) ast.Node {
	log.Debugf("parseExpression: %s precedence %s", p.curToken, precedence)
	prefix := p.prefixParseFns[p.curToken.Type()]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type())
		return nil
	}
	leftExp := prefix()
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type()]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}
	return leftExp
}

func (p *Parser) parseIdentifier() ast.Node {
	i := &ast.Identifier{}
	i.Token = p.curToken
	return i
}

func (p *Parser) parseIntegerLiteral() ast.Node {
	lit := &ast.IntegerLiteral{}
	lit.Token = p.curToken

	value, err := strconv.ParseInt(p.curToken.Literal(), 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Val = value

	return lit
}

func (p *Parser) parseFloatLiteral() ast.Node {
	lit := &ast.FloatLiteral{}
	lit.Token = p.curToken

	value, err := strconv.ParseFloat(p.curToken.Literal(), 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as float", p.curToken.Literal())
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Val = value

	return lit
}

func (p *Parser) parseBoolean() ast.Node {
	b := &ast.Boolean{Val: p.curTokenIs(token.TRUE)}
	b.Token = p.curToken
	return b
}

func (p *Parser) parseGroupedExpression() ast.Node {
	p.nextToken()
	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parsePrefixExpression() ast.Node {
	expression := &ast.PrefixExpression{}
	expression.Token = p.curToken

	p.nextToken()

	expression.Right = p.parseExpression(PREFIX)

	return expression
}

var precedences = map[token.Type]Priority{
	token.ASSIGN:   ASSIGN,
	token.EQ:       EQUALS,
	token.NOTEQ:    EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.LTEQ:     LESSGREATER,
	token.GTEQ:     LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.PERCENT:  PRODUCT,
	token.LPAREN:   CALL,
	token.LBRACKET: INDEX,
}

func (p *Parser) peekPrecedence() Priority {
	if p, ok := precedences[p.peekToken.Type()]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() Priority {
	if p, ok := precedences[p.curToken.Type()]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) parseInfixExpression(left ast.Node) ast.Node {
	expression := &ast.InfixExpression{
		Left: left,
	}
	expression.Token = p.curToken

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseIfExpression() ast.Node {
	expression := &ast.IfExpression{}
	expression.Token = p.curToken

	needCloseParen := false
	if p.peekTokenIs(token.LPAREN) {
		needCloseParen = true
		p.nextToken()
	}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	if needCloseParen && !p.expectPeek(token.RPAREN) {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()
	if p.continuationNeeded {
		return nil
	}

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if !p.expectPeek(token.LBRACE) {
			return nil
		}

		expression.Alternative = p.parseBlockStatement()
		if p.continuationNeeded {
			return nil
		}
	}

	return expression
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{}
	// block.Token = p.curToken
	block.Statements = []ast.Node{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		if p.curTokenIs(token.EOL) {
			log.Debugf("parseBlockStatement: EOL")
			p.continuationNeeded = true
			return nil
		}
		block.Statements = append(block.Statements, p.parseStatement())
		p.nextToken()
	}
	return block
}

func (p *Parser) parseFunctionLiteral() ast.Node {
	lit := &ast.FunctionLiteral{}
	lit.Token = p.curToken

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()
	if p.continuationNeeded {
		return nil
	}

	return lit
}

func (p *Parser) parseBuiltin() ast.Node {
	bi := &ast.Builtin{}
	bi.Token = p.curToken

	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	bi.Parameters = p.parseExpressionList(token.RPAREN)
	return bi
}

func (p *Parser) parseFunctionParameters() []ast.Node {
	identifiers := []ast.Node{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{}
	ident.Token = p.curToken
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		ident := &ast.Identifier{}
		ident.Token = p.curToken
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return identifiers
}

func (p *Parser) parseCallExpression(function ast.Node) ast.Node {
	exp := &ast.CallExpression{Function: function}
	exp.Token = p.curToken
	exp.Arguments = p.parseExpressionList(token.RPAREN)
	return exp
}

func (p *Parser) parseExpressionList(end token.Type) []ast.Node {
	args := []ast.Node{}

	if p.peekTokenIs(end) {
		p.nextToken()
		return args
	}
	p.nextToken()
	args = append(args, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return args
}

func (p *Parser) parseIndexExpression(left ast.Node) ast.Node {
	exp := &ast.IndexExpression{Left: left}
	exp.Token = p.curToken

	p.nextToken()
	exp.Index = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return exp
}

func (p *Parser) parseMapLiteral() ast.Node {
	mapRes := &ast.MapLiteral{}
	mapRes.Token = p.curToken
	mapRes.Pairs = make(map[ast.Node]ast.Node)

	for !p.peekTokenIs(token.RBRACE) {
		p.nextToken()
		key := p.parseExpression(LOWEST)

		if !p.expectPeek(token.COLON) {
			return nil
		}

		p.nextToken()
		value := p.parseExpression(LOWEST)

		mapRes.Pairs[key] = value

		if !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
			return nil
		}
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return mapRes
}

func (p *Parser) parseMacroLiteral() ast.Node {
	lit := &ast.MacroLiteral{}
	lit.Token = p.curToken
	if !p.expectPeek(token.LPAREN) {
		return nil
	}
	lit.Parameters = p.parseFunctionParameters()
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()
	if p.continuationNeeded {
		return nil
	}
	return lit
}
