package parser

import (
	"fortio.org/log"
	"github.com/ldemailly/gorpl/ast"
	"github.com/ldemailly/gorpl/lexer"
	"github.com/ldemailly/gorpl/token"
)

type Parser struct {
	l *lexer.Lexer

	curToken  token.Token
	peekToken token.Token
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l}

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil { // classic interface nil gotcha, must make sure explicit nil interface is returned (right type)
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		return p.parseLetStatement()
	default:
		return nil
	}
}

func (p *Parser) parseLetStatement() ast.Statement {
	stmt := &ast.LetStatement{Token: p.curToken}

	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

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

func sameToken(msg string, actual token.Token, expected token.Type) bool {
	res := actual.Type == expected
	if res {
		log.Debugf("%sTokenIs indeed: %s", msg, actual)
	} else {
		log.Warnf("%sTokenIs not: %s - found %s/%s instead", msg, expected, actual.Type, actual.Literal)
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
	} else {
		return false
	}
}
