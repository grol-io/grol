package parser

import (
	"fmt"
	"strconv"
	"strings"

	"fortio.org/log"
	"grol.io/grol/ast"
	"grol.io/grol/lexer"
	"grol.io/grol/token"
)

type (
	prefixParseFn  func() ast.Node
	infixParseFn   func(ast.Node) ast.Node
	postfixParseFn prefixParseFn
)

type Parser struct {
	l *lexer.Lexer

	prevToken *token.Token
	curToken  *token.Token
	peekToken *token.Token

	prevNewline        bool
	nextNewline        bool
	continuationNeeded bool
	prevPos            int

	errors []string

	prefixParseFns  map[token.Type]prefixParseFn
	infixParseFns   map[token.Type]infixParseFn
	postfixParseFns map[token.Type]postfixParseFn
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

func (p *Parser) registerPostfix(t token.Type, fn postfixParseFn) {
	p.postfixParseFns[t] = fn
}

func New(l *lexer.Lexer) *Parser { //nolint:funlen // yes we have a lot to register.
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[token.Type]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier)  // arguable that ident/ints are prefixes - they are absence of operator?
	p.registerPrefix(token.DOTDOT, p.parseIdentifier) // hack for now to treat .. as an identifier
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.PLUS, p.parsePrefixExpression)
	p.registerPrefix(token.INCR, p.parsePrefixExpression)
	p.registerPrefix(token.DECR, p.parsePrefixExpression)
	p.registerPrefix(token.BITNOT, p.parsePrefixExpression)
	p.registerPrefix(token.BITXOR, p.parsePrefixExpression) // go devs are used to using ^ for bit not
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.FOR, p.parseForExpression)
	p.registerPrefix(token.BREAK, p.parseControlExpression)
	p.registerPrefix(token.CONTINUE, p.parseControlExpression)
	p.registerPrefix(token.RETURN, p.parseReturnStatement)
	p.registerPrefix(token.FUNC, p.parseFunctionLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.LEN, p.parseBuiltin)
	p.registerPrefix(token.FIRST, p.parseBuiltin)
	p.registerPrefix(token.REST, p.parseBuiltin)
	p.registerPrefix(token.LBRACKET, p.parseArrayLiteral)
	p.registerPrefix(token.LBRACE, p.parseMapLiteral)
	p.registerPrefix(token.LINECOMMENT, p.parseComment)
	p.registerPrefix(token.BLOCKCOMMENT, p.parseComment)
	p.registerPrefix(token.PRINT, p.parseBuiltin)
	p.registerPrefix(token.PRINTLN, p.parseBuiltin)
	p.registerPrefix(token.LOG, p.parseBuiltin)
	p.registerPrefix(token.MACRO, p.parseMacroLiteral)
	p.registerPrefix(token.ERROR, p.parseBuiltin)
	p.registerPrefix(token.CATCH, p.parseBuiltin)
	p.registerPrefix(token.QUOTE, p.parseBuiltin)
	p.registerPrefix(token.UNQUOTE, p.parseBuiltin)
	p.registerPrefix(token.DEL, p.parseBuiltin)

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
	p.registerInfix(token.DOT, p.parseIndexExpression)
	p.registerInfix(token.LEFTSHIFT, p.parseInfixExpression)
	p.registerInfix(token.RIGHTSHIFT, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.BITAND, p.parseInfixExpression)
	p.registerInfix(token.BITOR, p.parseInfixExpression)
	p.registerInfix(token.BITXOR, p.parseInfixExpression)
	p.registerInfix(token.COLON, p.parseInfixExpression)
	p.registerInfix(token.LAMBDA, p.parseLambdaExpression)

	// no let:
	p.registerInfix(token.ASSIGN, p.parseInfixExpression)
	p.registerInfix(token.DEFINE, p.parseInfixExpression)

	p.postfixParseFns = make(map[token.Type]postfixParseFn)
	p.registerPostfix(token.INCR, p.parsePostfixExpression)
	p.registerPostfix(token.DECR, p.parsePostfixExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) nextToken() {
	p.prevToken = p.curToken
	p.curToken = p.peekToken
	p.prevPos = p.l.Pos()
	p.peekToken = p.l.NextToken()
	p.prevNewline = p.nextNewline
	p.nextNewline = p.l.HadNewline()
}

func (p *Parser) ParseProgram() *ast.Statements {
	program := &ast.Statements{}
	program.Statements = []ast.Node{}

	for p.curToken.Type() != token.EOF && p.curToken.Type() != token.EOL {
		stmt := p.parseStatement()
		if stmt == nil {
			return program
		}
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
	r.SameLineAsPrevious = !p.prevNewline
	r.SameLineAsNext = !p.nextNewline
	isBlockComment := (p.curToken.Type() == token.BLOCKCOMMENT)
	log.Debugf("parseComment: %#v", r)
	if isBlockComment {
		if !strings.HasSuffix(p.curToken.Literal(), "*/") {
			log.LogVf("parseComment: block comment not closed: %s", p.curToken.DebugString())
			p.continuationNeeded = true
			return nil
		}
	} else {
		if r.SameLineAsNext && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.EOL) {
			panic("parseComment for line comment: same line as next and not EOL/EOF")
		}
	}
	return r
}

func (p *Parser) parseStatement() ast.Node {
	if p.curToken.Type() == token.RETURN {
		return p.parseReturnStatement()
	}
	stmt := p.parseExpression(ast.LOWEST)
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
	return stmt
}

func (p *Parser) parseReturnStatement() ast.Node {
	stmt := &ast.ReturnStatement{}
	stmt.Token = p.curToken

	// hacky for empty expressions like plain `return`.
	if p.peekTokenIs(token.SEMICOLON) || p.peekTokenIs(token.RBRACE) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.EOL) {
		log.Debugf("parseExpression: %s returning nil", p.curToken.DebugString())
		// nil return value
		return stmt
	}

	p.nextToken()

	stmt.ReturnValue = p.parseExpression(ast.LOWEST)

	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func sameToken(msg string, actual *token.Token, expected token.Type) bool {
	res := actual.Type() == expected
	if res {
		log.Debugf("%s: indeed: %s", msg, actual.DebugString())
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

// ErrorLine returns the current line followed by a newline with a marker to the error position and the line number.
// If forPreviousToken is true, the error position is relative to the previous token instead of current one.
func (p *Parser) ErrorLine(forPreviousToken bool) (string, int) {
	line, errPos, lineNum := p.l.CurrentLine()
	if forPreviousToken {
		// When the error is about the previous token, adjust the position accordingly.
		// (note this doesn't work when the previous token in on a different line -- TODO: improve)
		errPos -= (p.l.Pos() - p.prevPos)
	}
	repeat := max(0, errPos-1)
	return line + "\n" + strings.Repeat(" ", repeat) + "^", lineNum
}

func (p *Parser) peekError(t token.Type) {
	log.Debugf("peekError: %s", t)
	errLine, lineNum := p.ErrorLine(false)
	msg := fmt.Sprintf("%d: expected next token to be `%s`, got `%s` instead:\n%s",
		lineNum, token.ByType(t).Literal(), p.peekToken.Literal(), errLine)
	p.errors = append(p.errors, msg)
}

func (p *Parser) noPrefixParseFnError(t *token.Token) {
	log.Debugf("Adding noPrefixParseFnError: %s", t.DebugString())
	errLine, lineNum := p.ErrorLine(true)
	msg := fmt.Sprintf("%d: no prefix parse function for `%s` found:\n%s", lineNum, t.Literal(), errLine)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseExpression(precedence ast.Priority) ast.Node {
	log.Debugf("parseExpression: %s precedence %s", p.curToken.DebugString(), precedence)
	if p.curToken.Type() == token.EOL {
		log.Debugf("parseExpression: EOL")
		p.continuationNeeded = true
		return nil
	}
	prefix := p.prefixParseFns[p.curToken.Type()]
	if prefix == nil {
		if !p.peekTokenIs(token.LAMBDA) { // To make () => { ... } without errors.
			p.noPrefixParseFnError(p.curToken)
		}
		return nil
	}
	leftExp := prefix()
	if p.peekTokenIs(token.LAMBDA) && precedence == ast.LAMBDA { // allow lambda chaining without parentheses in input.
		p.nextToken()
		return p.parseLambdaMulti(leftExp)
	}
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		t := p.peekToken.Type()
		infix := p.infixParseFns[t]
		if infix == nil {
			return leftExp
		}
		// Avoid that 3\n(4) tries to call 3 as a function with 4 param.
		// force calls() to not have whitespace between the function and the (.
		if t == token.LPAREN && p.l.HadWhitespace() {
			log.LogVf("parseExpression: call expression with whitespace")
			return leftExp
		}
		// same for a [n] that's the [n] literal array.
		if t == token.LBRACKET && p.l.HadWhitespace() {
			log.LogVf("parseExpression: index expression with whitespace")
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}
	return leftExp
}

func (p *Parser) parseIdentifier() ast.Node {
	postfix := p.postfixParseFns[p.peekToken.Type()]
	if postfix != nil {
		log.LogVf("parseIdentifier: next is a postfix for %s: %s", p.curToken.DebugString(), p.peekToken.DebugString())
		p.nextToken()
		return postfix()
	}
	i := &ast.Identifier{}
	i.Token = p.curToken
	return i
}

func (p *Parser) parseIntegerLiteral() ast.Node {
	value, err := strconv.ParseInt(p.curToken.Literal(), 0, 64)
	if err != nil { // switch to float
		return p.parseFloatLiteral()
	}
	lit := &ast.IntegerLiteral{}
	lit.Token = p.curToken
	lit.Val = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Node {
	value, err := strconv.ParseFloat(p.curToken.Literal(), 64)
	if err != nil {
		errLine, lineNum := p.ErrorLine(false)
		msg := fmt.Sprintf("%d: could not parse %q as float:\n%s", lineNum, p.curToken.Literal(), errLine)
		p.errors = append(p.errors, msg)
		return nil
	}
	lit := &ast.FloatLiteral{}
	lit.Token = p.curToken
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
	exp := p.parseExpression(ast.LOWEST)
	log.Debugf("parseGroupedExpression: %#v", exp)
	if p.peekTokenIs(token.LAMBDA) { // () => { ... } case
		p.nextToken()
		return p.parseLambdaMulti(exp)
	}
	if p.peekTokenIs(token.COMMA) { // (a,b,,.) => { ... } case
		p.nextToken()
		el := p.parseExpressionList(token.RPAREN)
		if el == nil {
			return nil
		}
		if !p.expectPeek(token.LAMBDA) {
			return nil
		}
		return p.parseLambdaMulti(exp, el...)
	}
	if !p.expectPeek(token.RPAREN) {
		return nil
	}
	return exp
}

func (p *Parser) parsePrefixExpression() ast.Node {
	expression := &ast.PrefixExpression{}
	expression.Token = p.curToken

	p.nextToken()

	expression.Right = p.parseExpression(ast.PREFIX)

	return expression
}

func (p *Parser) parsePostfixExpression() ast.Node {
	expression := &ast.PostfixExpression{}
	expression.Token = p.curToken
	expression.Prev = p.prevToken
	return expression
}

func (p *Parser) peekPrecedence() ast.Priority {
	if p, ok := ast.Precedences[p.peekToken.Type()]; ok {
		return p
	}
	return ast.LOWEST
}

func (p *Parser) curPrecedence() ast.Priority {
	if p, ok := ast.Precedences[p.curToken.Type()]; ok {
		return p
	}
	return ast.LOWEST
}

func (p *Parser) parseLambdaExpression(left ast.Node) ast.Node {
	return p.parseLambdaMulti(left)
}

func okParamList(nodes []ast.Node) (*token.Token, bool) {
	l := len(nodes)
	log.Debugf("okParamList: %d: %#v", l, nodes)
	for i, n := range nodes {
		last := i == l-1
		t := n.Value()
		if last && t.Type() == token.DOTDOT {
			return t, true
		}
		if t.Type() != token.IDENT {
			return t, false
		}
	}
	return nil, true
}

func (p *Parser) parseLambdaMulti(left ast.Node, more ...ast.Node) ast.Node {
	lambda := &ast.FunctionLiteral{IsLambda: true}
	lambda.Token = p.curToken
	if left == nil {
		lambda.Parameters = more
	} else {
		lambda.Parameters = append([]ast.Node{left}, more...)
	}
	t, ok := okParamList(lambda.Parameters)
	if !ok {
		errLine, lineNum := p.ErrorLine(false)
		p.errors = append(p.errors, fmt.Sprintf("%d: lambda parameters must be identifiers, not %s\n%s",
			lineNum, t.Literal(), errLine))
		return nil
	}
	if t != nil {
		lambda.Variadic = true
	}
	log.Debugf("parseLambdaMulti: %#v", lambda)
	if p.peekTokenIs(token.LBRACE) {
		p.nextToken()
		lambda.Body = p.parseBlockStatement()
		if p.continuationNeeded {
			return nil
		}
		log.Debugf("parseLambdaMulti: body: %#v", lambda.Body)
		return lambda
	}
	precedence := p.curPrecedence()
	p.nextToken()
	body := p.parseExpression(precedence)
	lambda.Body = &ast.Statements{Statements: []ast.Node{body}}
	return lambda
}

func (p *Parser) parseInfixExpression(left ast.Node) ast.Node {
	expression := &ast.InfixExpression{
		Left: left,
	}
	expression.Token = p.curToken

	precedence := p.curPrecedence()
	// handle [n:] case
	if (expression.Token.Type() == token.COLON) && (p.peekToken.Type() == token.RBRACKET) {
		return expression
	}
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) parseControlExpression() ast.Node {
	expression := &ast.ControlExpression{}
	expression.Token = p.curToken
	return expression
}

func (p *Parser) parseForExpression() ast.Node {
	expression := &ast.ForExpression{}
	expression.Token = p.curToken
	p.nextToken()
	expression.Condition = p.parseExpression(ast.LOWEST)

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Body = p.parseBlockStatement()
	if p.continuationNeeded {
		return nil
	}

	return expression
}

func (p *Parser) parseIfExpression() ast.Node {
	expression := &ast.IfExpression{}
	expression.Token = p.curToken

	p.nextToken()
	expression.Condition = p.parseExpression(ast.LOWEST)

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()
	if p.continuationNeeded {
		return nil
	}

	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if p.peekTokenIs(token.IF) {
			p.nextToken()
			expression.Alternative = &ast.Statements{Statements: []ast.Node{p.parseIfExpression()}}
			return expression
		}

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

func (p *Parser) parseBlockStatement() *ast.Statements {
	block := &ast.Statements{}
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
	// Optional name/identifier
	if p.peekTokenIs(token.IDENT) {
		p.nextToken()
		name := &ast.Identifier{}
		name.Token = p.curToken
		lit.Name = name
	}
	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters, lit.Variadic = p.parseFunctionParameters()

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

func (p *Parser) parseFunctionParameters() ([]ast.Node, bool) {
	identifiers := []ast.Node{}
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers, false
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
		p.skipPeekComments()
	}
	p.skipPeekComments()
	if !p.expectPeek(token.RPAREN) {
		log.Debugf("parseFunctionParameters: nil return 0: %s", p.peekToken.Literal())
		return nil, false
	}
	return identifiers, (p.prevToken.Type() == token.DOTDOT)
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
	p.skipCommentsIfAny()
	args = append(args, p.parseExpression(ast.LOWEST))
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		p.skipCommentsIfAny()
		args = append(args, p.parseExpression(ast.LOWEST))
		p.skipPeekComments()
	}
	p.skipPeekComments()
	if !p.expectPeek(end) {
		log.Debugf("parseCallExpression: nil return 0: %s", p.peekToken.Literal())
		return nil
	}
	return args
}

func (p *Parser) parseIndexExpression(left ast.Node) ast.Node {
	exp := &ast.IndexExpression{Left: left}
	exp.Token = p.curToken
	isDot := p.curToken.Type() == token.DOT

	p.nextToken()
	prec := ast.LOWEST
	if isDot {
		prec = ast.DOTINDEX
	}
	exp.Index = p.parseExpression(prec)
	if isDot {
		return exp
	}

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
		if p.continuationNeeded {
			return nil
		}
		p.skipCommentsIfAny()
		// comment at the end of the map
		if p.curToken.Type() == token.RBRACE {
			return mapRes
		}
		kv := p.parseExpression(ast.LOWEST)
		ex, ok := kv.(*ast.InfixExpression)
		if !ok || ex.Token.Type() != token.COLON {
			if p.peekTokenIs(token.EOL) {
				p.continuationNeeded = true
			} else {
				p.peekError(token.COLON)
			}
			return nil
		}
		key := ex.Left
		value := ex.Right

		mapRes.Pairs[key] = value
		mapRes.Order = append(mapRes.Order, key)

		if !p.peekComment() && !p.peekTokenIs(token.RBRACE) && !p.expectPeek(token.COMMA) {
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
	lit.Parameters, _ = p.parseFunctionParameters() // TODO variadic macros?
	if !p.expectPeek(token.LBRACE) {
		return nil
	}
	lit.Body = p.parseBlockStatement()
	if p.continuationNeeded {
		return nil
	}
	return lit
}

func (p *Parser) isComment() bool {
	return p.curToken.Type() == token.LINECOMMENT || p.curToken.Type() == token.BLOCKCOMMENT
}

// skipCommentsIfAny checks if the current token is a comment, logs it if it is, and advances to the next token
// until it finds a non-comment token.
func (p *Parser) skipCommentsIfAny() {
	for p.isComment() {
		log.LogVf("Ignoring current token comment: %s", p.curToken.Literal())
		p.nextToken()
	}
}

// peekComment checks if the peek token is a comment.
func (p *Parser) peekComment() bool {
	return p.peekTokenIs(token.LINECOMMENT) || p.peekTokenIs(token.BLOCKCOMMENT)
}

func (p *Parser) skipPeekComments() {
	for p.peekComment() {
		p.nextToken()
		log.LogVf("Ignoring peeked token comment: %s", p.curToken.Literal())
	}
}
