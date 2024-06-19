package parser

import (
	"fmt"
	"strconv"

	"fortio.org/log"
	"github.com/ldemailly/gorpl/ast"
	"github.com/ldemailly/gorpl/lexer"
	"github.com/ldemailly/gorpl/token"
)

type Priority int8

const (
	_ Priority = iota
	LOWEST
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
	CALL        // myFunction(X)
)

//go:generate stringer -type=Priority
var _ = CALL.String() // force compile error if go generate is missing.

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l *lexer.Lexer

	curToken  token.Token
	peekToken token.Token

	errors []string

	prefixParseFns map[token.Type]prefixParseFn
	infixParseFns  map[token.Type]infixParseFn
}

func (p *Parser) RegisterPrefix(t token.Type, fn prefixParseFn) {
	p.prefixParseFns[t] = fn
}

func (p *Parser) RegisterInfix(t token.Type, fn infixParseFn) {
	p.infixParseFns[t] = fn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[token.Type]prefixParseFn)
	p.RegisterPrefix(token.IDENT, p.parseIdentifier)
	p.RegisterPrefix(token.INT, p.parseIntegerLiteral)
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

	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil { // classic interface nil gotcha, must make sure explicit nil interface is returned (right type)
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Node {
	switch p.curToken.Type { //nolint:exhaustive // we're not done yet TODO: remove.
	case token.LET:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		return p.parseExpressionStatement()
	}
}

func (p *Parser) parseLetStatement() ast.Node {
	stmt := &ast.LetStatement{}
	stmt.Token = p.curToken

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Base: ast.Base{Token: p.curToken}, Val: p.curToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	// TODO: We're skipping the expressions until we
	// encounter a semicolon
	for !p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseReturnStatement() ast.Node {
	stmt := &ast.ReturnStatement{}
	stmt.Token = p.curToken

	p.nextToken()

	// TODO: We're skipping the expressions until we
	// encounter a semicolon
	for !p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func sameToken(msg string, actual token.Token, expected token.Type) bool {
	res := actual.Type == expected
	if res {
		log.Debugf("%sTokenIs indeed: %s", msg, actual)
	} else {
		log.LogVf("%sTokenIs not: %s - found %s/%s instead", msg, expected, actual.Type, actual.Literal)
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
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t token.Type) {
	msg := fmt.Sprintf("expected next token to be %s, got %s (%q) instead",
		t, p.peekToken.Type, p.peekToken.Literal)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseExpressionStatement() ast.Expression {
	stmt := &ast.ExpressionStatement{}
	stmt.Token = p.curToken

	stmt.Val = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseExpression(precedence Priority) ast.Expression {
	log.Debugf("parseExpression: %s precedence %s", p.curToken, precedence)
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		return nil
	}
	leftExp := prefix()

	return leftExp
}

func (p *Parser) parseIdentifier() ast.Expression {
	i := &ast.Identifier{}
	i.Token = p.curToken
	i.Val = p.curToken.Literal
	return i
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{}
	lit.Token = p.curToken

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Val = value

	return lit
}
