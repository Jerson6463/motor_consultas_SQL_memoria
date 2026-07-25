package executor

import (
	"io"
	"strings"
	"testing"

	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/planner"
	"motor-consultas-sql/internal/storage"
)

func tablaVentas(t *testing.T) *storage.Table {
	t.Helper()
	table, err := storage.LoadCSV("ventas", strings.NewReader("zona,monto\nNorte,10\nSur,20\nNorte,30\n"))
	if err != nil {
		t.Fatalf("LoadCSV devolvio error: %v", err)
	}
	return table
}

// expresion analiza una expresion suelta reutilizando el parser.
func expresion(t *testing.T, texto string) parser.Expression {
	t.Helper()
	query, err := parser.Parse("SELECT " + texto + " FROM t")
	if err != nil {
		t.Fatalf("Parse(%q) devolvio error: %v", texto, err)
	}
	return query.Select[0].Expression
}

// filas recorre un operador hasta agotarlo.
func filas(t *testing.T, operator Operator) []storage.Row {
	t.Helper()
	rows := []storage.Row{}
	for {
		row, err := operator.Next()
		if err == io.EOF {
			return rows
		}
		if err != nil {
			t.Fatalf("Next devolvio error: %v", err)
		}
		rows = append(rows, row)
	}
}

// TestBuildEnsamblaElPlan comprueba que Build traduce cada nodo logico al
// operador fisico correspondiente y respeta el orden del plan.
func TestBuildEnsamblaElPlan(t *testing.T) {
	plan := &planner.Limit{
		Max: 2,
		Input: &planner.Project{
			Items: []planner.ProjectItem{{Expression: parser.Identifier{Name: "monto"}, Name: "monto"}},
			Input: &planner.Sort{
				Terms: []parser.SortTerm{{Expression: parser.Identifier{Name: "monto"}, Descending: true}},
				Input: &planner.Scan{Table: tablaVentas(t)},
			},
		},
	}

	operator, err := Build(plan)
	if err != nil {
		t.Fatalf("Build devolvio error: %v", err)
	}
	defer operator.Close()

	if _, ok := operator.(*Limit); !ok {
		t.Fatalf("la raiz no es Limit: %T", operator)
	}
	rows := filas(t, operator)
	if len(rows) != 2 {
		t.Fatalf("filas = %d; se esperaban 2", len(rows))
	}
	if rows[0][0].Data != int64(30) || rows[1][0].Data != int64(20) {
		t.Errorf("orden incorrecto: %v, %v", rows[0][0].Data, rows[1][0].Data)
	}
}

// TestProjectEvaluaExpresiones cubre la proyeccion calculada, que es lo que
// permite aritmetica y alias en el SELECT.
func TestProjectEvaluaExpresiones(t *testing.T) {
	plan := &planner.Project{
		Items: []planner.ProjectItem{{Expression: expresion(t, "monto * 2"), Name: "doble"}},
		Input: &planner.Scan{Table: tablaVentas(t)},
	}

	operator, err := Build(plan)
	if err != nil {
		t.Fatalf("Build devolvio error: %v", err)
	}
	defer operator.Close()

	if columns := operator.Columns(); len(columns) != 1 || columns[0].Name != "doble" {
		t.Fatalf("columnas = %#v; se esperaba doble", columns)
	}
	if columns := operator.Columns(); columns[0].Type != storage.Integer {
		t.Errorf("tipo = %s; se esperaba entero", columns[0].Type)
	}
	rows := filas(t, operator)
	if len(rows) != 3 || rows[0][0].Data != int64(20) {
		t.Errorf("primera fila = %#v; se esperaba int64(20)", rows[0][0].Data)
	}
}

// TestProjectExpandeElComodin comprueba que el comodin copia las columnas de la
// entrada y se puede combinar con expresiones.
func TestProjectExpandeElComodin(t *testing.T) {
	plan := &planner.Project{
		Items: []planner.ProjectItem{
			{Star: true},
			{Expression: expresion(t, "monto + 1"), Name: "siguiente"},
		},
		Input: &planner.Scan{Table: tablaVentas(t)},
	}

	operator, err := Build(plan)
	if err != nil {
		t.Fatalf("Build devolvio error: %v", err)
	}
	defer operator.Close()

	columns := operator.Columns()
	want := []string{"zona", "monto", "siguiente"}
	if len(columns) != len(want) {
		t.Fatalf("columnas = %d; se esperaban %d", len(columns), len(want))
	}
	for index, name := range want {
		if columns[index].Name != name {
			t.Errorf("columna %d = %q; se esperaba %q", index, columns[index].Name, name)
		}
	}
}

func TestLimitAplicaElOffset(t *testing.T) {
	plan := &planner.Limit{
		Max:    1,
		Offset: 1,
		Input: &planner.Project{
			Items: []planner.ProjectItem{{Star: true}},
			Input: &planner.Scan{Table: tablaVentas(t)},
		},
	}

	operator, err := Build(plan)
	if err != nil {
		t.Fatalf("Build devolvio error: %v", err)
	}
	defer operator.Close()

	rows := filas(t, operator)
	if len(rows) != 1 {
		t.Fatalf("filas = %d; se esperaba 1", len(rows))
	}
	if rows[0][0].Data != "Sur" {
		t.Errorf("fila = %v; se esperaba Sur, la segunda", rows[0][0].Data)
	}
}

func TestScanCalificaConElAlias(t *testing.T) {
	scan := NewScan(tablaVentas(t), "v")
	columns := scan.Columns()
	if columns[0].Name != "v.zona" || columns[1].Name != "v.monto" {
		t.Errorf("columnas = %#v; se esperaban v.zona y v.monto", columns)
	}

	sinAlias := NewScan(tablaVentas(t), "")
	if sinAlias.Columns()[0].Name != "zona" {
		t.Errorf("sin alias no se debe calificar: %#v", sinAlias.Columns())
	}
}

// TestBuildPropagaErroresDeColumna comprueba que la resolucion de nombres, que
// es responsabilidad del executor, falla al construir el operador.
func TestBuildPropagaErroresDeColumna(t *testing.T) {
	plan := &planner.Project{
		Items: []planner.ProjectItem{{Expression: parser.Identifier{Name: "noexiste"}, Name: "noexiste"}},
		Input: &planner.Scan{Table: tablaVentas(t)},
	}

	if _, err := Build(plan); err == nil {
		t.Fatal("Build no devolvio error para una columna inexistente")
	}
}

// TestBuildValidaElWhereAlConstruirElFiltro fija que el WHERE se comprueba al
// construir, no al recorrer las filas.
func TestBuildValidaElWhereAlConstruirElFiltro(t *testing.T) {
	plan := &planner.Filter{
		Condition: expresion(t, "noexiste = 1"),
		Input:     &planner.Scan{Table: tablaVentas(t)},
	}

	if _, err := Build(plan); err == nil {
		t.Fatal("Build no devolvio error para un WHERE con una columna inexistente")
	}
}

func TestBuildRechazaNodosDesconocidos(t *testing.T) {
	if _, err := Build(nil); err == nil {
		t.Fatal("Build no devolvio error para un nodo nulo")
	}
}

// TestOperadorUnario cubre la evaluacion del signo: conserva el tipo, propaga
// NULL y rechaza los valores no numericos.
func TestOperadorUnario(t *testing.T) {
	columns := []storage.Column{
		{Name: "entero", Type: storage.Integer},
		{Name: "decimal", Type: storage.Decimal},
		{Name: "texto", Type: storage.Text},
		{Name: "vacio", Type: storage.Integer},
	}
	row := storage.Row{
		{Type: storage.Integer, Data: int64(10)},
		{Type: storage.Decimal, Data: 2.5},
		{Type: storage.Text, Data: "Ana"},
		{Type: storage.Integer, Null: true},
	}

	negado, err := evaluate(expresion(t, "-entero"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if negado.Type != storage.Integer || negado.Data != int64(-10) {
		t.Errorf("negacion = %#v; se esperaba int64(-10)", negado)
	}

	decimal, err := evaluate(expresion(t, "-decimal"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if decimal.Type != storage.Decimal || decimal.Data != -2.5 {
		t.Errorf("negacion decimal = %#v; se esperaba -2.5", decimal)
	}

	positivo, err := evaluate(expresion(t, "+entero"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if positivo.Data != int64(10) {
		t.Errorf("signo positivo = %#v; se esperaba int64(10)", positivo)
	}

	doble, err := evaluate(expresion(t, "- -entero"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if doble.Data != int64(10) {
		t.Errorf("doble negacion = %#v; se esperaba int64(10)", doble)
	}

	nulo, err := evaluate(expresion(t, "-vacio"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if !nulo.Null || nulo.Type != storage.Integer {
		t.Errorf("negacion de NULL = %#v; se esperaba NULL entero", nulo)
	}

	if _, err := evaluate(expresion(t, "-texto"), row, columns); err == nil {
		t.Error("evaluate no devolvio error al negar un texto")
	}

	// La precedencia del signo tambien debe respetarse al evaluar.
	precedencia, err := evaluate(expresion(t, "-entero * 2"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if precedencia.Data != int64(-20) {
		t.Errorf("-entero * 2 = %#v; se esperaba int64(-20)", precedencia)
	}
}

// TestUnarioEnLaValidacionDelWhere comprueba que validateExpression atraviesa
// el signo en vez de darlo por bueno.
func TestUnarioEnLaValidacionDelWhere(t *testing.T) {
	plan := &planner.Filter{
		Condition: expresion(t, "-noexiste < 0"),
		Input:     &planner.Scan{Table: tablaVentas(t)},
	}

	if _, err := Build(plan); err == nil {
		t.Fatal("Build no devolvio error para una columna inexistente bajo el signo")
	}
}

// TestAritmeticaConNulosYDivisionPorCero cubre las reglas de la evaluacion
// aritmetica.
func TestAritmeticaConNulosYDivisionPorCero(t *testing.T) {
	columns := []storage.Column{{Name: "a", Type: storage.Integer}, {Name: "b", Type: storage.Integer}}
	row := storage.Row{
		{Type: storage.Integer, Data: int64(10)},
		{Type: storage.Integer, Null: true},
	}

	suma, err := evaluate(expresion(t, "a + b"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if !suma.Null {
		t.Errorf("suma = %#v; un operando NULL debe propagar NULL", suma)
	}

	division, err := evaluate(expresion(t, "a / 0"), row, columns)
	if err == nil {
		t.Errorf("division = %#v; se esperaba error de division por cero", division)
	}

	entera, err := evaluate(expresion(t, "a * 3"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if entera.Type != storage.Integer || entera.Data != int64(30) {
		t.Errorf("producto = %#v; se esperaba int64(30)", entera)
	}

	decimal, err := evaluate(expresion(t, "a / 4"), row, columns)
	if err != nil {
		t.Fatalf("evaluate devolvio error: %v", err)
	}
	if decimal.Type != storage.Decimal || decimal.Data != 2.5 {
		t.Errorf("division = %#v; se esperaba 2.5 decimal", decimal)
	}
}
