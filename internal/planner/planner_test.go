package planner

import (
	"strings"
	"testing"

	"motor-consultas-sql/internal/catalog"
	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

func nuevoCatalogo(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat := catalog.New()
	for nombre, contenido := range map[string]string{
		"empleados": "nombre,area_id,salario\nAna,1,100\n",
		"areas":     "id,nombre\n1,Ventas\n",
		"sedes":     "id,ciudad\n1,Lima\n",
	} {
		table, err := storage.LoadCSV(nombre, strings.NewReader(contenido))
		if err != nil {
			t.Fatalf("LoadCSV(%q) devolvio error: %v", nombre, err)
		}
		if err := cat.Add(table); err != nil {
			t.Fatalf("Add(%q) devolvio error: %v", nombre, err)
		}
	}
	return cat
}

func planificar(t *testing.T, sql string) Node {
	t.Helper()
	node, err := planificarConError(t, sql)
	if err != nil {
		t.Fatalf("Plan devolvio error: %v", err)
	}
	return node
}

func planificarConError(t *testing.T, sql string) (Node, error) {
	t.Helper()
	statement, err := parser.Parse(sql)
	if err != nil {
		t.Fatalf("Parse devolvio error: %v", err)
	}
	return Plan(nuevoCatalogo(t), statement)
}

func TestPlanConsultaSimple(t *testing.T) {
	project, ok := planificar(t, "SELECT nombre FROM empleados").(*Project)
	if !ok {
		t.Fatalf("la raiz no es Project")
	}
	if len(project.Items) != 1 || project.Items[0].Name != "nombre" {
		t.Errorf("columnas proyectadas = %#v", project.Items)
	}
	if _, ok := project.Input.(*Scan); !ok {
		t.Fatalf("Project no lee de un Scan: %T", project.Input)
	}
}

func TestPlanSelectAllUsaElComodin(t *testing.T) {
	project, ok := planificar(t, "SELECT * FROM empleados").(*Project)
	if !ok {
		t.Fatalf("la raiz no es Project")
	}
	if len(project.Items) != 1 || !project.Items[0].Star {
		t.Errorf("SELECT * deberia producir un unico elemento comodin: %#v", project.Items)
	}
}

// TestPlanOrdenaAntesDeProyectar es lo que permite ORDER BY sobre columnas que
// no se seleccionan.
func TestPlanOrdenaAntesDeProyectar(t *testing.T) {
	limit, ok := planificar(t, "SELECT nombre FROM empleados WHERE area_id = 1 ORDER BY salario DESC LIMIT 2").(*Limit)
	if !ok {
		t.Fatalf("la raiz no es Limit")
	}
	if limit.Max != 2 || limit.Offset != 0 {
		t.Errorf("Limit = %#v; se esperaba Max 2 y Offset 0", limit)
	}
	project, ok := limit.Input.(*Project)
	if !ok {
		t.Fatalf("Limit no lee de un Project: %T", limit.Input)
	}
	sort, ok := project.Input.(*Sort)
	if !ok {
		t.Fatalf("Project no lee de un Sort: %T", project.Input)
	}
	if len(sort.Terms) != 1 || !sort.Terms[0].Descending {
		t.Errorf("terminos de orden = %#v", sort.Terms)
	}
	if got := parser.Format(sort.Terms[0].Expression); got != "salario" {
		t.Errorf("expresion de orden = %q; se esperaba salario", got)
	}
	filter, ok := sort.Input.(*Filter)
	if !ok {
		t.Fatalf("Sort no lee de un Filter: %T", sort.Input)
	}
	if _, ok := filter.Input.(*Scan); !ok {
		t.Fatalf("Filter no lee de un Scan: %T", filter.Input)
	}
}

func TestPlanOffsetSinLimit(t *testing.T) {
	limit, ok := planificar(t, "SELECT nombre FROM empleados OFFSET 3").(*Limit)
	if !ok {
		t.Fatalf("la raiz no es Limit")
	}
	if limit.Max >= 0 {
		t.Errorf("Max = %d; se esperaba negativo para indicar sin limite", limit.Max)
	}
	if limit.Offset != 3 {
		t.Errorf("Offset = %d; se esperaba 3", limit.Offset)
	}
}

// TestPlanReescribeLosAgregados comprueba la sustitucion de agregados y claves
// de grupo por referencias a las columnas que produce la agregacion.
func TestPlanReescribeLosAgregados(t *testing.T) {
	project, ok := planificar(t, "SELECT zona, COUNT(*) * 2 AS doble FROM empleados GROUP BY zona").(*Project)
	if !ok {
		t.Fatalf("la raiz no es Project")
	}

	if project.Items[0].Name != "zona" || project.Items[1].Name != "doble" {
		t.Errorf("nombres de salida = %q, %q", project.Items[0].Name, project.Items[1].Name)
	}
	if got := parser.Format(project.Items[1].Expression); got != "COUNT(*) * 2" {
		t.Errorf("expresion reescrita = %q; se esperaba \"COUNT(*) * 2\"", got)
	}
	// Tras la reescritura, COUNT(*) es una referencia a una columna, no una llamada.
	binary, ok := project.Items[1].Expression.(parser.BinaryExpression)
	if !ok {
		t.Fatalf("la expresion no es binaria: %T", project.Items[1].Expression)
	}
	if _, ok := binary.Left.(parser.Identifier); !ok {
		t.Errorf("el agregado no se sustituyo por una referencia: %T", binary.Left)
	}

	aggregate, ok := project.Input.(*Aggregate)
	if !ok {
		t.Fatalf("Project no lee de un Aggregate: %T", project.Input)
	}
	if len(aggregate.Calls) != 1 || !aggregate.Calls[0].Star {
		t.Errorf("llamadas = %#v; se esperaba un unico COUNT(*)", aggregate.Calls)
	}
	if len(aggregate.GroupBy) != 1 || parser.Format(aggregate.GroupBy[0]) != "zona" {
		t.Errorf("GROUP BY = %#v", aggregate.GroupBy)
	}
}

// TestPlanNoRepiteAgregadosIguales evita calcular dos veces la misma funcion.
func TestPlanNoRepiteAgregadosIguales(t *testing.T) {
	project := planificar(t, "SELECT COUNT(*), COUNT(*) + 1 FROM empleados").(*Project)
	aggregate, ok := project.Input.(*Aggregate)
	if !ok {
		t.Fatalf("Project no lee de un Aggregate: %T", project.Input)
	}
	if len(aggregate.Calls) != 1 {
		t.Errorf("llamadas = %d; se esperaba 1", len(aggregate.Calls))
	}
}

// TestPlanAgregaSinGroupBy comprueba que una funcion de agregacion basta para
// que la consulta agrupe.
func TestPlanAgregaSinGroupBy(t *testing.T) {
	project := planificar(t, "SELECT COUNT(*) FROM empleados").(*Project)
	if _, ok := project.Input.(*Aggregate); !ok {
		t.Fatalf("Project no lee de un Aggregate: %T", project.Input)
	}
}

func TestPlanColocaElHavingSobreLaAgregacion(t *testing.T) {
	project := planificar(t, "SELECT zona FROM empleados GROUP BY zona HAVING COUNT(*) > 2").(*Project)
	filter, ok := project.Input.(*Filter)
	if !ok {
		t.Fatalf("Project no lee de un Filter: %T", project.Input)
	}
	if _, ok := filter.Input.(*Aggregate); !ok {
		t.Fatalf("el Filter del HAVING no lee de un Aggregate: %T", filter.Input)
	}
	if got := parser.Format(filter.Condition); got != "COUNT(*) > 2" {
		t.Errorf("condicion del HAVING = %q", got)
	}
}

func TestPlanResuelveAliasEnOrderBy(t *testing.T) {
	project := planificar(t, "SELECT salario * 12 AS anual FROM empleados ORDER BY anual DESC").(*Project)
	sort, ok := project.Input.(*Sort)
	if !ok {
		t.Fatalf("Project no lee de un Sort: %T", project.Input)
	}
	if got := parser.Format(sort.Terms[0].Expression); got != "salario * 12" {
		t.Errorf("el alias no se resolvio: %q", got)
	}
}

// TestPlanRecorreLasExpresionesUnarias comprueba que el signo no esconde nada a
// los recorridos del planner: la reescritura de agregados, la validacion de
// GROUP BY y la resolucion de alias deben atravesarlo.
func TestPlanRecorreLasExpresionesUnarias(t *testing.T) {
	t.Run("reescribe el agregado bajo el signo", func(t *testing.T) {
		project := planificar(t, "SELECT -COUNT(*) FROM empleados").(*Project)
		aggregate, ok := project.Input.(*Aggregate)
		if !ok {
			t.Fatalf("Project no lee de un Aggregate: %T", project.Input)
		}
		if len(aggregate.Calls) != 1 {
			t.Fatalf("llamadas = %d; se esperaba 1", len(aggregate.Calls))
		}
		unary, ok := project.Items[0].Expression.(parser.UnaryExpression)
		if !ok {
			t.Fatalf("la expresion no es unaria: %T", project.Items[0].Expression)
		}
		if _, ok := unary.Operand.(parser.Identifier); !ok {
			t.Errorf("el agregado no se sustituyo bajo el signo: %T", unary.Operand)
		}
	})

	t.Run("detecta la columna suelta bajo el signo", func(t *testing.T) {
		_, err := planificarConError(t, "SELECT -nombre, COUNT(*) FROM empleados GROUP BY area_id")
		if err == nil {
			t.Fatal("Plan no devolvio error para una columna fuera del GROUP BY")
		}
		if !strings.Contains(err.Error(), "debe aparecer en GROUP BY") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("detecta la funcion desconocida bajo el signo", func(t *testing.T) {
		if _, err := planificarConError(t, "SELECT -stddev(salario) FROM empleados"); err == nil {
			t.Fatal("Plan no devolvio error para una funcion desconocida")
		}
	})

	t.Run("rechaza el agregado bajo el signo en WHERE", func(t *testing.T) {
		if _, err := planificarConError(t, "SELECT nombre FROM empleados WHERE -COUNT(*) < 1"); err == nil {
			t.Fatal("Plan no devolvio error para un agregado en WHERE")
		}
	})

	t.Run("resuelve el alias bajo el signo", func(t *testing.T) {
		project := planificar(t, "SELECT salario * 12 AS anual FROM empleados ORDER BY -anual").(*Project)
		sort, ok := project.Input.(*Sort)
		if !ok {
			t.Fatalf("Project no lee de un Sort: %T", project.Input)
		}
		if got := parser.Format(sort.Terms[0].Expression); got != "-(salario * 12)" {
			t.Errorf("el alias no se resolvio bajo el signo: %q", got)
		}
	})
}

func TestPlanCalificaLasTablasDelJoin(t *testing.T) {
	project := planificar(t, "SELECT empleados.nombre FROM empleados INNER JOIN areas ON empleados.area_id = areas.id").(*Project)
	join, ok := project.Input.(*Join)
	if !ok {
		t.Fatalf("Project no lee de un Join: %T", project.Input)
	}
	left, ok := join.Left.(*Scan)
	if !ok {
		t.Fatalf("el lado izquierdo no es un Scan: %T", join.Left)
	}
	if left.Alias != "empleados" {
		t.Errorf("alias izquierdo = %q; se esperaba empleados", left.Alias)
	}
	right, ok := join.Right.(*Scan)
	if !ok || right.Alias != "areas" {
		t.Errorf("lado derecho incorrecto: %#v", join.Right)
	}
}

// TestPlanAnidaVariosJoins comprueba que N joins producen nodos anidados y que
// cada tabla se califica una sola vez.
func TestPlanAnidaVariosJoins(t *testing.T) {
	sql := "SELECT empleados.nombre FROM empleados " +
		"INNER JOIN areas ON empleados.area_id = areas.id " +
		"INNER JOIN sedes ON areas.id = sedes.id"
	project := planificar(t, sql).(*Project)

	exterior, ok := project.Input.(*Join)
	if !ok {
		t.Fatalf("Project no lee de un Join: %T", project.Input)
	}
	if scan, ok := exterior.Right.(*Scan); !ok || scan.Alias != "sedes" {
		t.Errorf("el lado derecho exterior deberia ser sedes: %#v", exterior.Right)
	}
	interior, ok := exterior.Left.(*Join)
	if !ok {
		t.Fatalf("el lado izquierdo no es un Join: %T", exterior.Left)
	}
	if scan, ok := interior.Left.(*Scan); !ok || scan.Alias != "empleados" {
		t.Errorf("el lado izquierdo interior deberia ser empleados: %#v", interior.Left)
	}
}

func TestPlanNoCalificaSinJoin(t *testing.T) {
	project := planificar(t, "SELECT nombre FROM empleados").(*Project)
	scan := project.Input.(*Scan)
	if scan.Alias != "" {
		t.Errorf("alias = %q; sin JOIN no se deben calificar las columnas", scan.Alias)
	}
}

func TestPlanRechazaTablasInexistentes(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{name: "FROM", sql: "SELECT nombre FROM otra"},
		{name: "JOIN", sql: "SELECT nombre FROM empleados INNER JOIN otra ON empleados.area_id = otra.id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := planificarConError(t, test.sql); err == nil {
				t.Fatal("Plan no devolvio error")
			}
		})
	}
}

// TestPlanExigePertenenciaAlGroupBy comprueba la validacion correcta: no basta
// con que coincida la cantidad de columnas.
func TestPlanExigePertenenciaAlGroupBy(t *testing.T) {
	_, err := planificarConError(t, "SELECT nombre, COUNT(*) FROM empleados GROUP BY area_id")
	if err == nil {
		t.Fatal("Plan no devolvio error para una columna fuera del GROUP BY")
	}
	if !strings.Contains(err.Error(), "debe aparecer en GROUP BY") {
		t.Errorf("error = %v; se esperaba el aviso de GROUP BY", err)
	}
}

func TestPlanAceptaColumnasAgrupadas(t *testing.T) {
	if _, err := planificarConError(t, "SELECT area_id, COUNT(*) FROM empleados GROUP BY area_id"); err != nil {
		t.Fatalf("Plan devolvio error para una consulta valida: %v", err)
	}
}

func TestPlanRechazaUsosInvalidos(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "funcion desconocida",
			sql:  "SELECT stddev(salario) FROM empleados",
			want: "no existe",
		},
		{
			name: "agregado en WHERE",
			sql:  "SELECT nombre FROM empleados WHERE COUNT(*) > 1",
			want: "WHERE",
		},
		{
			name: "HAVING sin agregacion",
			sql:  "SELECT nombre FROM empleados HAVING nombre = 'Ana'",
			want: "HAVING requiere",
		},
		{
			name: "comodin con agregados",
			sql:  "SELECT *, COUNT(*) FROM empleados",
			want: "*",
		},
		{
			name: "agregados anidados",
			sql:  "SELECT SUM(COUNT(salario)) FROM empleados",
			want: "anidar",
		},
		{
			name: "asterisco en SUM",
			sql:  "SELECT SUM(*) FROM empleados",
			want: "requiere una columna",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := planificarConError(t, test.sql)
			if err == nil {
				t.Fatal("Plan no devolvio error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %v; se esperaba que incluyera %q", err, test.want)
			}
		})
	}
}
