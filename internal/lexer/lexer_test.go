package lexer

import "testing"

func TestLexRecognizesKeywordsAndOperators(t *testing.T) {
	tokens, err := Lex("select * from empleados where edad >= 18 and activo <> false")
	if err != nil {
		t.Fatalf("Lex devolvio error: %v", err)
	}

	want := []TokenKind{SelectToken, StarToken, FromToken, IdentifierToken, WhereToken, IdentifierToken, GreaterEqualToken, NumberToken, AndToken, IdentifierToken, NotEqualToken, BooleanToken, EOF}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %d; se esperaban %d", len(tokens), len(want))
	}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Errorf("token %d = %s; se esperaba %s", index, tokens[index].Kind, kind)
		}
	}
}

func TestLexReconoceLasPalabrasClaveNuevas(t *testing.T) {
	tokens, err := Lex("having as offset")
	if err != nil {
		t.Fatalf("Lex devolvio error: %v", err)
	}

	want := []TokenKind{HavingToken, AsToken, OffsetToken, EOF}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Errorf("token %d = %s; se esperaba %s", index, tokens[index].Kind, kind)
		}
	}
}

func TestLexReconoceLosOperadoresAritmeticos(t *testing.T) {
	tokens, err := Lex("a + b - c * d / e")
	if err != nil {
		t.Fatalf("Lex devolvio error: %v", err)
	}

	want := []TokenKind{IdentifierToken, PlusToken, IdentifierToken, MinusToken, IdentifierToken, StarToken, IdentifierToken, SlashToken, IdentifierToken, EOF}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %d; se esperaban %d", len(tokens), len(want))
	}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Errorf("token %d = %s; se esperaba %s", index, tokens[index].Kind, kind)
		}
	}
}

// TestLexTrataLasFuncionesComoIdentificadores fija que el lexer ya no conoce
// los nombres de las funciones: quien las resuelve es el registro.
func TestLexTrataLasFuncionesComoIdentificadores(t *testing.T) {
	tokens, err := Lex("COUNT(*) SUM AVG MIN MAX STDDEV")
	if err != nil {
		t.Fatalf("Lex devolvio error: %v", err)
	}

	want := []TokenKind{
		IdentifierToken, LeftParenToken, StarToken, RightParenToken,
		IdentifierToken, IdentifierToken, IdentifierToken, IdentifierToken, IdentifierToken,
		EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %d; se esperaban %d", len(tokens), len(want))
	}
	for index, kind := range want {
		if tokens[index].Kind != kind {
			t.Errorf("token %d = %s; se esperaba %s", index, tokens[index].Kind, kind)
		}
	}
	if tokens[0].Lexeme != "COUNT" {
		t.Errorf("lexema = %q; se esperaba COUNT", tokens[0].Lexeme)
	}
}
