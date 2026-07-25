package parser
//v2
import (
	"strings"
	"testing"
)

// tabla devuelve el nombre de la tabla de un FROM que es una sola referencia.
func tabla(t *testing.T, from FromItem) string {
	t.Helper()
	ref, ok := from.(*TableRef)
	if !ok {
		t.Fatalf("el FROM no es una TableRef: %T", from)
	}
	return ref.Name
}

func TestParseSelectWithWhere(t *testing.T) {
	query, err := Parse("SELECT nombre, edad FROM empleados WHERE (edad >= 18 AND activo = true) OR nombre = 'Ana'")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}

	if tabla(t, query.From) != "empleados" {
		t.Fatalf("tabla incorrecta: %#v", query.From)
	}
	if len(query.Select) != 2 {
		t.Fatalf("elementos del SELECT = %d; se esperaban 2", len(query.Select))
	}
	if Format(query.Select[0].Expression) != "nombre" || Format(query.Select[1].Expression) != "edad" {
		t.Errorf("columnas = %q, %q", Format(query.Select[0].Expression), Format(query.Select[1].Expression))
	}

	or, ok := query.Where.(BinaryExpression)
	if !ok || or.Operator != OpOr {
		t.Fatalf("WHERE no conserva OR: %#v", query.Where)
	}
	and, ok := or.Left.(BinaryExpression)
	if !ok || and.Operator != OpAnd {
		t.Fatalf("WHERE no conserva AND: %#v", or.Left)
	}
}

func TestParseRejectsInvalidQueries(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "sin FROM", input: "SELECT nombre empleados"},
		{name: "sin operador", input: "SELECT * FROM empleados WHERE edad 18"},
		{name: "parentesis sin cerrar", input: "SELECT * FROM empleados WHERE (edad = 18"},
		{name: "texto sin cerrar", input: "SELECT * FROM empleados WHERE nombre = 'Ana"},
		{name: "alias sin nombre", input: "SELECT nombre AS FROM empleados"},
		{name: "join sin ON", input: "SELECT * FROM empleados INNER JOIN areas"},
		{name: "llamada sin cerrar", input: "SELECT COUNT(*, FROM empleados"},
		{name: "offset no numerico", input: "SELECT * FROM empleados OFFSET x"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.input)
			if err == nil {
				t.Fatal("Parse no devolvio error")
			}
			if !strings.Contains(err.Error(), "posicion") {
				t.Errorf("el error no incluye posicion: %v", err)
			}
		})
	}
}

func TestParseOrderByAndLimit(t *testing.T) {
	query, err := Parse("SELECT nombre FROM empleados ORDER BY nombre DESC LIMIT 2")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if len(query.OrderBy) != 1 || !query.OrderBy[0].Descending {
		t.Fatalf("ORDER BY no se analizo correctamente: %#v", query.OrderBy)
	}
	if Format(query.OrderBy[0].Expression) != "nombre" {
		t.Errorf("expresion de orden = %q", Format(query.OrderBy[0].Expression))
	}
	if query.Limit == nil || *query.Limit != 2 {
		t.Fatalf("LIMIT no se analizo correctamente: %#v", query.Limit)
	}
	if query.Offset != nil {
		t.Errorf("OFFSET = %#v; se esperaba nil", query.Offset)
	}
}

func TestParseLimitConOffset(t *testing.T) {
	query, err := Parse("SELECT nombre FROM empleados LIMIT 10 OFFSET 5")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if query.Limit == nil || *query.Limit != 10 {
		t.Fatalf("LIMIT = %#v; se esperaba 10", query.Limit)
	}
	if query.Offset == nil || *query.Offset != 5 {
		t.Fatalf("OFFSET = %#v; se esperaba 5", query.Offset)
	}
}

func TestParseInnerJoin(t *testing.T) {
	query, err := Parse("SELECT empleados.nombre FROM empleados INNER JOIN areas ON empleados.area_id = areas.id")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	join, ok := query.From.(*JoinRef)
	if !ok {
		t.Fatalf("el FROM no es un JoinRef: %T", query.From)
	}
	if tabla(t, join.Left) != "empleados" || tabla(t, join.Right) != "areas" {
		t.Errorf("tablas del join incorrectas: %#v", join)
	}
	if Format(join.On) != "empleados.area_id = areas.id" {
		t.Errorf("condicion = %q", Format(join.On))
	}
}

// TestParseVariosJoins comprueba que los joins encadenados se pliegan hacia la
// izquierda, que es lo que permite mas de dos tablas.
func TestParseVariosJoins(t *testing.T) {
	query, err := Parse("SELECT * FROM a INNER JOIN b ON a.id = b.id INNER JOIN c ON b.id = c.id")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}

	exterior, ok := query.From.(*JoinRef)
	if !ok {
		t.Fatalf("el FROM no es un JoinRef: %T", query.From)
	}
	if tabla(t, exterior.Right) != "c" {
		t.Errorf("tabla derecha exterior = %q; se esperaba c", tabla(t, exterior.Right))
	}
	interior, ok := exterior.Left.(*JoinRef)
	if !ok {
		t.Fatalf("el lado izquierdo no es un JoinRef: %T", exterior.Left)
	}
	if tabla(t, interior.Left) != "a" || tabla(t, interior.Right) != "b" {
		t.Errorf("join interior incorrecto: %#v", interior)
	}
}

func TestParseAliasYExpresiones(t *testing.T) {
	query, err := Parse("SELECT nombre, salario * 12 AS anual FROM empleados")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if len(query.Select) != 2 {
		t.Fatalf("elementos = %d; se esperaban 2", len(query.Select))
	}
	if query.Select[0].Alias != "" {
		t.Errorf("alias inesperado: %q", query.Select[0].Alias)
	}
	if query.Select[1].Alias != "anual" {
		t.Errorf("alias = %q; se esperaba anual", query.Select[1].Alias)
	}
	if got := Format(query.Select[1].Expression); got != "salario * 12" {
		t.Errorf("expresion = %q; se esperaba \"salario * 12\"", got)
	}
}

// TestParsePrecedenciaAritmetica comprueba que la multiplicacion liga mas
// fuerte que la suma y que los parentesis la alteran.
func TestParsePrecedenciaAritmetica(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "SELECT a + b * c FROM t", want: "a + (b * c)"},
		{input: "SELECT (a + b) * c FROM t", want: "(a + b) * c"},
		{input: "SELECT a - b - c FROM t", want: "(a - b) - c"},
		{input: "SELECT a / b * c FROM t", want: "(a / b) * c"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			query, err := Parse(test.input)
			if err != nil {
				t.Fatalf("Parse devolvio error: %v", err)
			}
			if got := Format(query.Select[0].Expression); got != test.want {
				t.Errorf("expresion = %q; se esperaba %q", got, test.want)
			}
		})
	}
}

// TestParseOperadorUnario comprueba que el signo liga mas fuerte que la
// multiplicacion y que se puede encadenar.
func TestParseOperadorUnario(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "SELECT -salario FROM t", want: "-salario"},
		{input: "SELECT +salario FROM t", want: "+salario"},
		{input: "SELECT -5 FROM t", want: "-5"},
		{input: "SELECT - -a FROM t", want: "-(-a)"},
		{input: "SELECT -a * b FROM t", want: "(-a) * b"},
		{input: "SELECT a * -b FROM t", want: "a * (-b)"},
		{input: "SELECT -(a + b) FROM t", want: "-(a + b)"},
		{input: "SELECT -a + b FROM t", want: "(-a) + b"},
		{input: "SELECT -SUM(a) FROM t", want: "-SUM(a)"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			query, err := Parse(test.input)
			if err != nil {
				t.Fatalf("Parse devolvio error: %v", err)
			}
			if got := Format(query.Select[0].Expression); got != test.want {
				t.Errorf("expresion = %q; se esperaba %q", got, test.want)
			}
		})
	}
}

func TestParseUnarioEnWhere(t *testing.T) {
	query, err := Parse("SELECT nombre FROM t WHERE saldo < -100")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if got := Format(query.Where); got != "saldo < (-100)" {
		t.Errorf("WHERE = %q", got)
	}

	comparison, ok := query.Where.(BinaryExpression)
	if !ok {
		t.Fatalf("el WHERE no es una comparacion: %T", query.Where)
	}
	unary, ok := comparison.Right.(UnaryExpression)
	if !ok || unary.Operator != OpNegate {
		t.Fatalf("el operando derecho no es una negacion: %#v", comparison.Right)
	}
}

func TestUnaryOpString(t *testing.T) {
	tests := []struct {
		operator UnaryOp
		want     string
	}{
		{OpNegate, "-"},
		{OpPositive, "+"},
		{UnaryInvalid, "invalido"},
	}
	for _, test := range tests {
		if got := test.operator.String(); got != test.want {
			t.Errorf("String() = %q; se esperaba %q", got, test.want)
		}
	}
}

// TestParseAsteriscoSegunContexto cubre los tres significados de `*`.
func TestParseAsteriscoSegunContexto(t *testing.T) {
	comodin, err := Parse("SELECT * FROM t")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if len(comodin.Select) != 1 || !comodin.Select[0].Star {
		t.Errorf("SELECT * no produjo el comodin: %#v", comodin.Select)
	}

	llamada, err := Parse("SELECT COUNT(*) FROM t")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	call, ok := llamada.Select[0].Expression.(FunctionCall)
	if !ok || !call.Star || call.Name != "COUNT" {
		t.Errorf("COUNT(*) no se analizo correctamente: %#v", llamada.Select[0].Expression)
	}

	producto, err := Parse("SELECT a * b FROM t")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if got := Format(producto.Select[0].Expression); got != "a * b" {
		t.Errorf("expresion = %q; se esperaba \"a * b\"", got)
	}

	mixto, err := Parse("SELECT *, a * b FROM t")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if !mixto.Select[0].Star {
		t.Error("el primer elemento debia ser el comodin")
	}
	if got := Format(mixto.Select[1].Expression); got != "a * b" {
		t.Errorf("segundo elemento = %q", got)
	}
}

func TestParseFuncionesGenericas(t *testing.T) {
	query, err := Parse("SELECT stddev(salario), COUNT(*), SUM(a + b) FROM t")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}

	// El parser acepta cualquier nombre: la validacion es del registro.
	primera, ok := query.Select[0].Expression.(FunctionCall)
	if !ok || primera.Name != "stddev" || len(primera.Args) != 1 {
		t.Errorf("stddev no se analizo como llamada: %#v", query.Select[0].Expression)
	}
	tercera, ok := query.Select[2].Expression.(FunctionCall)
	if !ok || len(tercera.Args) != 1 {
		t.Fatalf("SUM no se analizo como llamada: %#v", query.Select[2].Expression)
	}
	if got := Format(tercera); got != "SUM(a + b)" {
		t.Errorf("formato = %q; se esperaba \"SUM(a + b)\"", got)
	}
}

func TestParseGroupByYHaving(t *testing.T) {
	query, err := Parse("SELECT zona, COUNT(*) FROM ventas GROUP BY zona HAVING COUNT(*) > 2")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if len(query.GroupBy) != 1 || Format(query.GroupBy[0]) != "zona" {
		t.Errorf("GROUP BY = %#v", query.GroupBy)
	}
	if query.Having == nil {
		t.Fatal("HAVING no se analizo")
	}
	if got := Format(query.Having); got != "COUNT(*) > 2" {
		t.Errorf("HAVING = %q", got)
	}
}

func TestFormatEscapaLosTextos(t *testing.T) {
	query, err := Parse("SELECT nombre FROM t WHERE nombre = 'O''Brien'")
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	if got := Format(query.Where); got != "nombre = 'O''Brien'" {
		t.Errorf("formato = %q", got)
	}
}

func TestBinaryOpString(t *testing.T) {
	tests := []struct {
		operator BinaryOp
		want     string
	}{
		{OpAnd, "AND"},
		{OpOr, "OR"},
		{OpEqual, "="},
		{OpNotEqual, "<>"},
		{OpLess, "<"},
		{OpGreater, ">"},
		{OpLessEqual, "<="},
		{OpGreaterEqual, ">="},
		{OpAdd, "+"},
		{OpSubtract, "-"},
		{OpMultiply, "*"},
		{OpDivide, "/"},
		{OpInvalid, "invalido"},
	}
	for _, test := range tests {
		if got := test.operator.String(); got != test.want {
			t.Errorf("String() = %q; se esperaba %q", got, test.want)
		}
	}
}

func TestIsArithmetic(t *testing.T) {
	for _, operator := range []BinaryOp{OpAdd, OpSubtract, OpMultiply, OpDivide} {
		if !operator.IsArithmetic() {
			t.Errorf("%s deberia ser aritmetico", operator)
		}
	}
	for _, operator := range []BinaryOp{OpAnd, OpOr, OpEqual, OpLess} {
		if operator.IsArithmetic() {
			t.Errorf("%s no deberia ser aritmetico", operator)
		}
	}
}
