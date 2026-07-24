// Package parser analiza el subconjunto SQL soportado por el motor y produce su AST.
package parser

import (
	"fmt"
	"strconv"

	"motor-consultas-sql/internal/lexer"
)

// Parse analiza una consulta SQL y devuelve su AST.
func Parse(input string) (*Query, error) {
	tokens, err := lexer.Lex(input)
	if err != nil {
		return nil, err
	}
	return ParseTokens(tokens)
}

// ParseTokens analiza una lista de tokens terminada en EOF.
func ParseTokens(tokens []lexer.Token) (*Query, error) {
	parser := parser{tokens: tokens}
	query, err := parser.parseQuery()
	if err != nil {
		return nil, err
	}
	if token := parser.current(); token.Kind != lexer.EOF {
		return nil, parser.unexpected(token, "fin de consulta")
	}
	return query, nil
}

type parser struct {
	tokens []lexer.Token
	index  int
}

func (p *parser) parseQuery() (*Query, error) {
	if _, err := p.expect(lexer.SelectToken); err != nil {
		return nil, err
	}

	query := &Query{}
	items, err := p.parseSelectItems()
	if err != nil {
		return nil, err
	}
	query.Select = items

	if _, err := p.expect(lexer.FromToken); err != nil {
		return nil, err
	}
	from, err := p.parseFrom()
	if err != nil {
		return nil, err
	}
	query.From = from

	if p.match(lexer.WhereToken) {
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		query.Where = where
	}
	if p.match(lexer.GroupToken) {
		if _, err := p.expect(lexer.ByToken); err != nil {
			return nil, err
		}
		groups, err := p.parseExpressionList()
		if err != nil {
			return nil, err
		}
		query.GroupBy = groups
	}
	if p.match(lexer.HavingToken) {
		having, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		query.Having = having
	}
	if p.match(lexer.OrderToken) {
		if _, err := p.expect(lexer.ByToken); err != nil {
			return nil, err
		}
		terms, err := p.parseSortTerms()
		if err != nil {
			return nil, err
		}
		query.OrderBy = terms
	}
	if p.match(lexer.LimitToken) {
		limit, err := p.parseCount("LIMIT")
		if err != nil {
			return nil, err
		}
		query.Limit = limit
	}
	if p.match(lexer.OffsetToken) {
		offset, err := p.parseCount("OFFSET")
		if err != nil {
			return nil, err
		}
		query.Offset = offset
	}

	return query, nil
}

// parseSelectItems analiza la lista de seleccion.
func (p *parser) parseSelectItems() ([]SelectItem, error) {
	items := make([]SelectItem, 0, 1)
	for {
		item, err := p.parseSelectItem()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		if !p.match(lexer.CommaToken) {
			return items, nil
		}
	}
}

// parseSelectItem analiza un elemento de la lista de seleccion. El asterisco es
// el comodin solo cuando le sigue una coma o FROM; en cualquier otra posicion
// es el operador de multiplicacion.
func (p *parser) parseSelectItem() (SelectItem, error) {
	if p.current().Kind == lexer.StarToken {
		next := p.peek(1).Kind
		if next == lexer.CommaToken || next == lexer.FromToken {
			p.index++
			return SelectItem{Star: true}, nil
		}
	}

	expression, err := p.parseExpression()
	if err != nil {
		return SelectItem{}, err
	}
	item := SelectItem{Expression: expression}
	if p.match(lexer.AsToken) {
		alias, err := p.expect(lexer.IdentifierToken)
		if err != nil {
			return SelectItem{}, err
		}
		item.Alias = alias.Lexeme
	}
	return item, nil
}

// parseFrom analiza la clausula FROM plegando los INNER JOIN hacia la izquierda.
func (p *parser) parseFrom() (FromItem, error) {
	left, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}

	for p.match(lexer.InnerToken) {
		if _, err := p.expect(lexer.JoinToken); err != nil {
			return nil, err
		}
		right, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.OnToken); err != nil {
			return nil, err
		}
		condition, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		left = &JoinRef{Left: left, Right: right, On: condition}
	}
	return left, nil
}

func (p *parser) parseTableRef() (FromItem, error) {
	table, err := p.expect(lexer.IdentifierToken)
	if err != nil {
		return nil, err
	}
	return &TableRef{Name: table.Lexeme}, nil
}

func (p *parser) parseExpressionList() ([]Expression, error) {
	expressions := make([]Expression, 0, 1)
	for {
		expression, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		expressions = append(expressions, expression)
		if !p.match(lexer.CommaToken) {
			return expressions, nil
		}
	}
}

func (p *parser) parseSortTerms() ([]SortTerm, error) {
	terms := make([]SortTerm, 0, 1)
	for {
		expression, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		term := SortTerm{Expression: expression}
		if p.match(lexer.DescToken) {
			term.Descending = true
		} else {
			p.match(lexer.AscToken)
		}
		terms = append(terms, term)
		if !p.match(lexer.CommaToken) {
			return terms, nil
		}
	}
}

// parseCount analiza el numero de LIMIT u OFFSET, que debe ser un entero no
// negativo escrito sin ceros a la izquierda.
func (p *parser) parseCount(clause string) (*int, error) {
	token, err := p.expect(lexer.NumberToken)
	if err != nil {
		return nil, err
	}
	value, err := strconv.Atoi(token.Lexeme)
	if err != nil || value < 0 || token.Lexeme != strconv.Itoa(value) {
		return nil, fmt.Errorf("%s invalido en la posicion %d", clause, token.Position)
	}
	return &value, nil
}

// La cadena de precedencia, de menor a mayor: OR < AND < comparacion <
// suma y resta < multiplicacion y division < signo < primaria.

func (p *parser) parseExpression() (Expression, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.match(lexer.OrToken) {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = BinaryExpression{Left: left, Operator: OpOr, Right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expression, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.match(lexer.AndToken) {
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = BinaryExpression{Left: left, Operator: OpAnd, Right: right}
	}
	return left, nil
}

func (p *parser) parseComparison() (Expression, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	operator := p.current()
	if !isComparison(operator.Kind) {
		return left, nil
	}
	p.index++
	right, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	return BinaryExpression{Left: left, Operator: binaryOp(operator.Kind), Right: right}, nil
}

func (p *parser) parseAdditive() (Expression, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for {
		operator := p.current().Kind
		if operator != lexer.PlusToken && operator != lexer.MinusToken {
			return left, nil
		}
		p.index++
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = BinaryExpression{Left: left, Operator: binaryOp(operator), Right: right}
	}
}

func (p *parser) parseMultiplicative() (Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		operator := p.current().Kind
		if operator != lexer.StarToken && operator != lexer.SlashToken {
			return left, nil
		}
		p.index++
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = BinaryExpression{Left: left, Operator: binaryOp(operator), Right: right}
	}
}

// parseUnary analiza el signo que precede a un operando. Liga mas fuerte que la
// multiplicacion, de modo que -a * b es (-a) * b, y admite signos encadenados.
func (p *parser) parseUnary() (Expression, error) {
	operator := UnaryInvalid
	switch p.current().Kind {
	case lexer.MinusToken:
		operator = OpNegate
	case lexer.PlusToken:
		operator = OpPositive
	default:
		return p.parsePrimary()
	}

	p.index++
	operand, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	return UnaryExpression{Operator: operator, Operand: operand}, nil
}

func (p *parser) parsePrimary() (Expression, error) {
	token := p.current()
	switch token.Kind {
	case lexer.LeftParenToken:
		p.index++
		expression, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.RightParenToken); err != nil {
			return nil, err
		}
		return expression, nil
	case lexer.IdentifierToken:
		p.index++
		if p.current().Kind == lexer.LeftParenToken {
			return p.parseFunctionCall(token.Lexeme)
		}
		return Identifier{Name: token.Lexeme}, nil
	case lexer.NumberToken, lexer.StringToken, lexer.BooleanToken, lexer.NullToken:
		p.index++
		return Literal{Value: token.Lexeme, Kind: literalKind(token.Kind)}, nil
	default:
		return nil, p.unexpected(token, "identificador o literal")
	}
}

// parseFunctionCall analiza los argumentos de una llamada. El parser no valida
// que la funcion exista: eso lo resuelve el registro de funciones.
func (p *parser) parseFunctionCall(name string) (Expression, error) {
	if _, err := p.expect(lexer.LeftParenToken); err != nil {
		return nil, err
	}

	call := FunctionCall{Name: name}
	if p.match(lexer.StarToken) {
		call.Star = true
	} else if p.current().Kind != lexer.RightParenToken {
		arguments, err := p.parseExpressionList()
		if err != nil {
			return nil, err
		}
		call.Args = arguments
	}
	if _, err := p.expect(lexer.RightParenToken); err != nil {
		return nil, err
	}
	return call, nil
}

func (p *parser) current() lexer.Token {
	return p.peek(0)
}

func (p *parser) peek(offset int) lexer.Token {
	index := p.index + offset
	if index >= len(p.tokens) {
		return lexer.Token{Kind: lexer.EOF}
	}
	return p.tokens[index]
}

func (p *parser) match(kind lexer.TokenKind) bool {
	if p.current().Kind != kind {
		return false
	}
	p.index++
	return true
}

func (p *parser) expect(kind lexer.TokenKind) (lexer.Token, error) {
	token := p.current()
	if token.Kind != kind {
		return lexer.Token{}, p.unexpected(token, kind.String())
	}
	p.index++
	return token, nil
}

func (p *parser) unexpected(token lexer.Token, expected string) error {
	return fmt.Errorf("error de sintaxis en la posicion %d: se esperaba %s y se encontro %s", token.Position, expected, token.Kind)
}

func isComparison(kind lexer.TokenKind) bool {
	switch kind {
	case lexer.EqualToken, lexer.NotEqualToken, lexer.LessToken, lexer.GreaterToken, lexer.LessEqualToken, lexer.GreaterEqualToken:
		return true
	default:
		return false
	}
}

// binaryOp traduce un token de operador al operador equivalente del AST.
func binaryOp(kind lexer.TokenKind) BinaryOp {
	switch kind {
	case lexer.EqualToken:
		return OpEqual
	case lexer.NotEqualToken:
		return OpNotEqual
	case lexer.LessToken:
		return OpLess
	case lexer.GreaterToken:
		return OpGreater
	case lexer.LessEqualToken:
		return OpLessEqual
	case lexer.GreaterEqualToken:
		return OpGreaterEqual
	case lexer.PlusToken:
		return OpAdd
	case lexer.MinusToken:
		return OpSubtract
	case lexer.StarToken:
		return OpMultiply
	case lexer.SlashToken:
		return OpDivide
	default:
		return OpInvalid
	}
}

// literalKind traduce un token literal al tipo de literal del AST.
func literalKind(kind lexer.TokenKind) LiteralKind {
	switch kind {
	case lexer.StringToken:
		return LiteralString
	case lexer.NumberToken:
		return LiteralNumber
	case lexer.BooleanToken:
		return LiteralBoolean
	case lexer.NullToken:
		return LiteralNull
	default:
		return LiteralInvalid
	}
}
