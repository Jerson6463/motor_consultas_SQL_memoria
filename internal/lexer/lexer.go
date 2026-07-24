// Package lexer convierte el texto de una consulta SQL en tokens.
package lexer

import (
	"fmt"
	"strings"
	"unicode"
)

// Lex convierte una consulta SQL en tokens.
func Lex(input string) ([]Token, error) {
	tokens := make([]Token, 0)
	for position := 0; position < len(input); {
		char := rune(input[position])
		if unicode.IsSpace(char) {
			position++
			continue
		}

		if isIdentifierStart(char) {
			start := position
			position++
			for position < len(input) && isIdentifierPart(rune(input[position])) {
				position++
			}
			lexeme := input[start:position]
			tokens = append(tokens, Token{Kind: keywordKind(lexeme), Lexeme: lexeme, Position: start})
			continue
		}

		if unicode.IsDigit(char) {
			start := position
			position++
			for position < len(input) && unicode.IsDigit(rune(input[position])) {
				position++
			}
			if position < len(input) && input[position] == '.' {
				position++
				if position == len(input) || !unicode.IsDigit(rune(input[position])) {
					return nil, fmt.Errorf("numero decimal incompleto en la posicion %d", start)
				}
				for position < len(input) && unicode.IsDigit(rune(input[position])) {
					position++
				}
			}
			tokens = append(tokens, Token{Kind: NumberToken, Lexeme: input[start:position], Position: start})
			continue
		}

		if input[position] == '\'' {
			start := position
			position++
			var value strings.Builder
			closed := false
			for position < len(input) {
				if input[position] != '\'' {
					value.WriteByte(input[position])
					position++
					continue
				}
				if position+1 < len(input) && input[position+1] == '\'' {
					value.WriteByte('\'')
					position += 2
					continue
				}
				position++
				closed = true
				break
			}
			if !closed {
				return nil, fmt.Errorf("texto sin cerrar en la posicion %d", start)
			}
			tokens = append(tokens, Token{Kind: StringToken, Lexeme: value.String(), Position: start})
			continue
		}

		kind, width := symbolKind(input[position:])
		if kind == Invalid {
			return nil, fmt.Errorf("caracter inesperado %q en la posicion %d", input[position], position)
		}
		tokens = append(tokens, Token{Kind: kind, Lexeme: input[position : position+width], Position: position})
		position += width
	}

	tokens = append(tokens, Token{Kind: EOF, Position: len(input)})
	return tokens, nil
}

func isIdentifierStart(char rune) bool {
	return unicode.IsLetter(char) || char == '_'
}

func isIdentifierPart(char rune) bool {
	return isIdentifierStart(char) || unicode.IsDigit(char) || char == '.'
}

func keywordKind(lexeme string) TokenKind {
	switch strings.ToUpper(lexeme) {
	case "SELECT":
		return SelectToken
	case "FROM":
		return FromToken
	case "WHERE":
		return WhereToken
	case "AND":
		return AndToken
	case "OR":
		return OrToken
	case "ORDER":
		return OrderToken
	case "BY":
		return ByToken
	case "ASC":
		return AscToken
	case "DESC":
		return DescToken
	case "LIMIT":
		return LimitToken
	case "OFFSET":
		return OffsetToken
	case "GROUP":
		return GroupToken
	case "HAVING":
		return HavingToken
	case "AS":
		return AsToken
	case "INNER":
		return InnerToken
	case "JOIN":
		return JoinToken
	case "ON":
		return OnToken
	case "TRUE", "FALSE":
		return BooleanToken
	case "NULL":
		return NullToken
	default:
		return IdentifierToken
	}
}

func symbolKind(input string) (TokenKind, int) {
	if len(input) >= 2 {
		switch input[:2] {
		case "<>":
			return NotEqualToken, 2
		case "<=":
			return LessEqualToken, 2
		case ">=":
			return GreaterEqualToken, 2
		}
	}

	switch input[0] {
	case ',':
		return CommaToken, 1
	case '*':
		return StarToken, 1
	case '+':
		return PlusToken, 1
	case '-':
		return MinusToken, 1
	case '/':
		return SlashToken, 1
	case '(':
		return LeftParenToken, 1
	case ')':
		return RightParenToken, 1
	case '=':
		return EqualToken, 1
	case '<':
		return LessToken, 1
	case '>':
		return GreaterToken, 1
	default:
		return Invalid, 0
	}
}
