// Package tests contiene las pruebas de integracion del motor: recorren el
// flujo completo SQL -> lexer -> parser -> planner -> executor -> resultado.
package tests

import (
	"io"
	"strings"
	"testing"

	"motor-consultas-sql/internal/engine"
	"motor-consultas-sql/internal/storage"
)

const empleadosCSV = "nombre,edad,activo\nAna,30,true\nBeto,15,true\nCarla,40,false\n"

// nuevoMotor crea un motor con las tablas indicadas ya cargadas.
func nuevoMotor(t *testing.T, tablas map[string]string) *engine.Engine {
	t.Helper()
	motor := engine.New()
	for nombre, contenido := range tablas {
		if _, err := motor.LoadCSV(nombre, strings.NewReader(contenido)); err != nil {
			t.Fatalf("LoadCSV(%q) devolvio error: %v", nombre, err)
		}
	}
	return motor
}

// consultar ejecuta una consulta y devuelve todas sus filas.
func consultar(t *testing.T, motor *engine.Engine, sql string) []storage.Row {
	t.Helper()
	result, err := motor.Query(sql)
	if err != nil {
		t.Fatalf("Query devolvio error: %v", err)
	}
	defer result.Close()

	rows := []storage.Row{}
	for {
		row, err := result.Next()
		if err == io.EOF {
			return rows
		}
		if err != nil {
			t.Fatalf("Next devolvio error: %v", err)
		}
		rows = append(rows, row)
	}
}

// columnas devuelve los nombres de las columnas de una consulta.
func columnas(t *testing.T, motor *engine.Engine, sql string) []string {
	t.Helper()
	result, err := motor.Query(sql)
	if err != nil {
		t.Fatalf("Query devolvio error: %v", err)
	}
	defer result.Close()

	names := []string{}
	for _, column := range result.Columns() {
		names = append(names, column.Name)
	}
	return names
}

func TestConsultaFiltraYProyecta(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": empleadosCSV})

	result, err := motor.Query("SELECT nombre FROM empleados WHERE edad >= 18 AND activo = true")
	if err != nil {
		t.Fatalf("Query devolvio error: %v", err)
	}
	defer result.Close()

	row, err := result.Next()
	if err != nil {
		t.Fatalf("Next devolvio error: %v", err)
	}
	if len(row) != 1 || row[0].Data != "Ana" {
		t.Fatalf("fila = %#v; se esperaba solo Ana", row)
	}
	if _, err := result.Next(); err != io.EOF {
		t.Fatalf("Next = %v; se esperaba io.EOF", err)
	}
}

func TestConsultaRechazaColumnasDesconocidas(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": empleadosCSV})

	if _, err := motor.Query("SELECT sueldo FROM empleados"); err == nil {
		t.Fatal("Query no devolvio error para una columna inexistente")
	}
}

func TestConsultaOrdenaYLimita(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": empleadosCSV})

	rows := consultar(t, motor, "SELECT nombre FROM empleados ORDER BY nombre DESC LIMIT 2")
	if len(rows) != 2 {
		t.Fatalf("filas = %d; se esperaban 2", len(rows))
	}
	if rows[0][0].Data != "Carla" || rows[1][0].Data != "Beto" {
		t.Fatalf("orden incorrecto: %v, %v", rows[0][0].Data, rows[1][0].Data)
	}
}

// TestOrdenaPorColumnaNoSeleccionada cubre uno de los tres defectos corregidos:
// antes esta consulta fallaba porque el orden se aplicaba tras la proyeccion.
func TestOrdenaPorColumnaNoSeleccionada(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": empleadosCSV})

	rows := consultar(t, motor, "SELECT nombre FROM empleados ORDER BY edad DESC")
	if len(rows) != 3 {
		t.Fatalf("filas = %d; se esperaban 3", len(rows))
	}
	if rows[0][0].Data != "Carla" || rows[1][0].Data != "Ana" || rows[2][0].Data != "Beto" {
		t.Errorf("orden por edad incorrecto: %v, %v, %v", rows[0][0].Data, rows[1][0].Data, rows[2][0].Data)
	}
	if len(rows[0]) != 1 {
		t.Errorf("la fila tiene %d columnas; solo se selecciono nombre", len(rows[0]))
	}
}

func TestConsultaAgrupaYAgrega(t *testing.T) {
	// El monto vacio de la zona Sur debe ignorarse en SUM, AVG, MIN y MAX,
	// pero COUNT(*) si cuenta la fila.
	motor := nuevoMotor(t, map[string]string{
		"ventas": "zona,monto\nNorte,10\nNorte,20\nSur,5\nSur,\n",
	})

	rows := consultar(t, motor, "SELECT zona, COUNT(*), SUM(monto), AVG(monto), MIN(monto), MAX(monto) FROM ventas GROUP BY zona")
	if len(rows) != 2 {
		t.Fatalf("grupos = %d; se esperaban 2", len(rows))
	}

	norte := rows[0]
	if norte[0].Data != "Norte" {
		t.Fatalf("primer grupo = %v; se esperaba Norte", norte[0].Data)
	}
	if norte[1].Data != int64(2) {
		t.Errorf("COUNT(*) = %#v; se esperaba int64(2)", norte[1].Data)
	}
	if norte[2].Data != 30.0 {
		t.Errorf("SUM = %#v; se esperaba 30.0", norte[2].Data)
	}
	if norte[3].Data != 15.0 {
		t.Errorf("AVG = %#v; se esperaba 15.0", norte[3].Data)
	}
	if norte[4].Data != int64(10) {
		t.Errorf("MIN = %#v; se esperaba int64(10)", norte[4].Data)
	}
	if norte[5].Data != int64(20) {
		t.Errorf("MAX = %#v; se esperaba int64(20)", norte[5].Data)
	}

	sur := rows[1]
	if sur[0].Data != "Sur" {
		t.Fatalf("segundo grupo = %v; se esperaba Sur", sur[0].Data)
	}
	if sur[1].Data != int64(2) {
		t.Errorf("COUNT(*) de Sur = %#v; se esperaba int64(2), el NULL tambien cuenta", sur[1].Data)
	}
	if sur[2].Data != 5.0 {
		t.Errorf("SUM de Sur = %#v; se esperaba 5.0, el NULL se ignora", sur[2].Data)
	}
	if sur[3].Data != 5.0 {
		t.Errorf("AVG de Sur = %#v; se esperaba 5.0, el NULL no entra en el promedio", sur[3].Data)
	}
}

// TestAgregadosRespetanElOrdenDelSelect cubre el segundo defecto corregido:
// antes la salida era siempre las agrupaciones primero.
func TestAgregadosRespetanElOrdenDelSelect(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"ventas": "zona,monto\nNorte,10\nNorte,20\n"})

	got := columnas(t, motor, "SELECT COUNT(*), zona FROM ventas GROUP BY zona")
	if len(got) != 2 || got[0] != "COUNT(*)" || got[1] != "zona" {
		t.Fatalf("columnas = %v; se esperaba el orden del SELECT", got)
	}

	rows := consultar(t, motor, "SELECT COUNT(*), zona FROM ventas GROUP BY zona")
	if rows[0][0].Data != int64(2) || rows[0][1].Data != "Norte" {
		t.Errorf("fila = %v, %v; se esperaba 2, Norte", rows[0][0].Data, rows[0][1].Data)
	}
}

// TestGroupByExigePertenencia cubre el tercer defecto corregido: antes la
// validacion solo contaba columnas y esta consulta pasaba en silencio.
func TestGroupByExigePertenencia(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": empleadosCSV})

	_, err := motor.Query("SELECT nombre, COUNT(*) FROM empleados GROUP BY activo")
	if err == nil {
		t.Fatal("Query no devolvio error para una columna fuera del GROUP BY")
	}
	if !strings.Contains(err.Error(), "debe aparecer en GROUP BY") {
		t.Errorf("error = %v; se esperaba el aviso de GROUP BY", err)
	}
}

// TestMinYMaxDeclaranElTipoDelArgumento cubre el cuarto cambio: antes la
// columna se declaraba decimal aunque devolviera un entero.
func TestMinYMaxDeclaranElTipoDelArgumento(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"ventas": "zona,monto\nNorte,10\nNorte,20\n"})

	result, err := motor.Query("SELECT MIN(monto), MAX(monto), SUM(monto) FROM ventas")
	if err != nil {
		t.Fatalf("Query devolvio error: %v", err)
	}
	defer result.Close()

	columns := result.Columns()
	if columns[0].Type != storage.Integer || columns[1].Type != storage.Integer {
		t.Errorf("MIN y MAX declaran %s y %s; se esperaba entero", columns[0].Type, columns[1].Type)
	}
	if columns[2].Type != storage.Decimal {
		t.Errorf("SUM declara %s; se esperaba decimal", columns[2].Type)
	}
}

func TestConsultaEjecutaInnerJoin(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{
		"empleados": "nombre,area_id\nAna,1\nBeto,2\n",
		"areas":     "id,nombre\n1,Ventas\n2,Soporte\n",
	})

	rows := consultar(t, motor, "SELECT empleados.nombre, areas.nombre FROM empleados INNER JOIN areas ON empleados.area_id = areas.id ORDER BY empleados.nombre")
	if len(rows) != 2 {
		t.Fatalf("filas = %d; se esperaban 2", len(rows))
	}
	if rows[0][0].Data != "Ana" || rows[0][1].Data != "Ventas" {
		t.Errorf("primera fila = %v, %v; se esperaba Ana, Ventas", rows[0][0].Data, rows[0][1].Data)
	}
	if rows[1][0].Data != "Beto" || rows[1][1].Data != "Soporte" {
		t.Errorf("segunda fila = %v, %v; se esperaba Beto, Soporte", rows[1][0].Data, rows[1][1].Data)
	}
}

// TestConsultaConVariosJoins comprueba que las columnas se califican una sola
// vez aunque haya mas de un JOIN.
func TestConsultaConVariosJoins(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{
		"empleados": "nombre,area_id\nAna,1\n",
		"areas":     "id,nombre,sede_id\n1,Ventas,7\n",
		"sedes":     "id,ciudad\n7,Lima\n",
	})

	sql := "SELECT empleados.nombre, areas.nombre, sedes.ciudad FROM empleados " +
		"INNER JOIN areas ON empleados.area_id = areas.id " +
		"INNER JOIN sedes ON areas.sede_id = sedes.id"

	got := columnas(t, motor, sql)
	want := []string{"empleados.nombre", "areas.nombre", "sedes.ciudad"}
	for index, name := range want {
		if got[index] != name {
			t.Errorf("columna %d = %q; se esperaba %q", index, got[index], name)
		}
	}

	rows := consultar(t, motor, sql)
	if len(rows) != 1 {
		t.Fatalf("filas = %d; se esperaba 1", len(rows))
	}
	if rows[0][0].Data != "Ana" || rows[0][1].Data != "Ventas" || rows[0][2].Data != "Lima" {
		t.Errorf("fila = %v; se esperaba Ana, Ventas, Lima", rows[0])
	}
}

func TestConsultaConservaColumnasCalificadas(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{
		"empleados": "nombre,area_id\nAna,1\n",
		"areas":     "id,nombre\n1,Ventas\n",
	})

	got := columnas(t, motor, "SELECT empleados.nombre, areas.nombre FROM empleados INNER JOIN areas ON empleados.area_id = areas.id")
	if len(got) != 2 || got[0] != "empleados.nombre" || got[1] != "areas.nombre" {
		t.Fatalf("columnas = %v", got)
	}
}

func TestConsultaNombraLasColumnasAgregadas(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"ventas": "zona,monto\nNorte,10\n"})

	got := columnas(t, motor, "SELECT zona, COUNT(*), SUM(monto) FROM ventas GROUP BY zona")
	want := []string{"zona", "COUNT(*)", "SUM(monto)"}
	if len(got) != len(want) {
		t.Fatalf("columnas = %d; se esperaban %d", len(got), len(want))
	}
	for index, name := range want {
		if got[index] != name {
			t.Errorf("columna %d = %q; se esperaba %q", index, got[index], name)
		}
	}
}

func TestConsultaConAliasYAritmetica(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": "nombre,salario\nAna,100\nBeto,200\n"})

	got := columnas(t, motor, "SELECT nombre, salario * 12 AS anual FROM empleados")
	if got[1] != "anual" {
		t.Errorf("columna = %q; se esperaba anual", got[1])
	}

	rows := consultar(t, motor, "SELECT nombre, salario * 12 AS anual FROM empleados ORDER BY anual DESC")
	if len(rows) != 2 {
		t.Fatalf("filas = %d; se esperaban 2", len(rows))
	}
	if rows[0][0].Data != "Beto" || rows[0][1].Data != int64(2400) {
		t.Errorf("primera fila = %v, %v; se esperaba Beto, 2400", rows[0][0].Data, rows[0][1].Data)
	}
}

// TestNombrePorDefectoDeUnaExpresion comprueba que una columna calculada sin
// alias toma la forma canonica de su expresion.
func TestNombrePorDefectoDeUnaExpresion(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": "nombre,salario\nAna,100\n"})

	got := columnas(t, motor, "SELECT salario * 12 FROM empleados")
	if len(got) != 1 || got[0] != "salario * 12" {
		t.Errorf("columnas = %v; se esperaba \"salario * 12\"", got)
	}
}

func TestConsultaConHaving(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{
		"ventas": "zona,monto\nNorte,10\nNorte,20\nSur,5\n",
	})

	rows := consultar(t, motor, "SELECT zona, COUNT(*) FROM ventas GROUP BY zona HAVING COUNT(*) > 1")
	if len(rows) != 1 {
		t.Fatalf("filas = %d; se esperaba solo el grupo Norte", len(rows))
	}
	if rows[0][0].Data != "Norte" || rows[0][1].Data != int64(2) {
		t.Errorf("fila = %v, %v; se esperaba Norte, 2", rows[0][0].Data, rows[0][1].Data)
	}
}

func TestConsultaConOffset(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": empleadosCSV})

	rows := consultar(t, motor, "SELECT nombre FROM empleados ORDER BY nombre LIMIT 1 OFFSET 1")
	if len(rows) != 1 {
		t.Fatalf("filas = %d; se esperaba 1", len(rows))
	}
	if rows[0][0].Data != "Beto" {
		t.Errorf("fila = %v; se esperaba Beto", rows[0][0].Data)
	}

	sinLimite := consultar(t, motor, "SELECT nombre FROM empleados ORDER BY nombre OFFSET 2")
	if len(sinLimite) != 1 || sinLimite[0][0].Data != "Carla" {
		t.Errorf("OFFSET sin LIMIT = %v; se esperaba solo Carla", sinLimite)
	}
}

// TestConsultaCompleta ejercita todas las clausulas juntas.
func TestConsultaCompleta(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{
		"empleados": "nombre,area_id,salario,activo\nAna,1,100,true\nBeto,1,200,true\nCarla,2,300,true\nDiego,2,400,false\n",
		"areas":     "id,nombre\n1,Ventas\n2,Soporte\n",
	})

	sql := "SELECT areas.nombre, COUNT(*), SUM(empleados.salario) * 2 AS doble " +
		"FROM empleados INNER JOIN areas ON empleados.area_id = areas.id " +
		"WHERE empleados.activo = true " +
		"GROUP BY areas.nombre " +
		"HAVING COUNT(*) > 0 " +
		"ORDER BY doble DESC " +
		"LIMIT 5 OFFSET 0"

	got := columnas(t, motor, sql)
	want := []string{"areas.nombre", "COUNT(*)", "doble"}
	for index, name := range want {
		if got[index] != name {
			t.Errorf("columna %d = %q; se esperaba %q", index, got[index], name)
		}
	}

	rows := consultar(t, motor, sql)
	if len(rows) != 2 {
		t.Fatalf("filas = %d; se esperaban 2", len(rows))
	}
	// Ventas suma 300 y Soporte 300 (Diego esta inactivo); empatan a 600.
	if rows[0][2].Data != 600.0 || rows[1][2].Data != 600.0 {
		t.Errorf("dobles = %v, %v; se esperaba 600 en ambos", rows[0][2].Data, rows[1][2].Data)
	}
	if rows[0][1].Data != int64(2) || rows[1][1].Data != int64(1) {
		t.Errorf("cuentas = %v, %v; se esperaba 2 y 1", rows[0][1].Data, rows[1][1].Data)
	}
}

// TestConsultaConOperadorUnario recorre el signo por todas las clausulas, que
// es donde se vería si algun recorrido de expresiones se hubiera quedado sin
// tratarlo.
func TestConsultaConOperadorUnario(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{
		"cuentas": "titular,saldo\nAna,100\nBeto,-50\nCarla,-200\n",
	})

	t.Run("literal negativo en WHERE", func(t *testing.T) {
		rows := consultar(t, motor, "SELECT titular FROM cuentas WHERE saldo < -100")
		if len(rows) != 1 || rows[0][0].Data != "Carla" {
			t.Errorf("filas = %v; se esperaba solo Carla", rows)
		}
	})

	t.Run("negacion en el SELECT", func(t *testing.T) {
		rows := consultar(t, motor, "SELECT titular, -saldo AS deuda FROM cuentas WHERE saldo < 0 ORDER BY titular")
		if len(rows) != 2 {
			t.Fatalf("filas = %d; se esperaban 2", len(rows))
		}
		if rows[0][1].Data != int64(50) || rows[1][1].Data != int64(200) {
			t.Errorf("deudas = %v, %v; se esperaba 50 y 200", rows[0][1].Data, rows[1][1].Data)
		}
	})

	t.Run("nombre por defecto", func(t *testing.T) {
		got := columnas(t, motor, "SELECT -saldo FROM cuentas")
		if len(got) != 1 || got[0] != "-saldo" {
			t.Errorf("columnas = %v; se esperaba \"-saldo\"", got)
		}
	})

	t.Run("orden por expresion negada", func(t *testing.T) {
		// Ordenar por -saldo ascendente equivale a ordenar por saldo
		// descendente: Ana (100), Beto (-50), Carla (-200).
		rows := consultar(t, motor, "SELECT titular FROM cuentas ORDER BY -saldo")
		want := []string{"Ana", "Beto", "Carla"}
		for index, name := range want {
			if rows[index][0].Data != name {
				t.Errorf("fila %d = %v; se esperaba %s", index, rows[index][0].Data, name)
			}
		}

		// La comprobacion cruzada: debe coincidir con el orden descendente.
		descendente := consultar(t, motor, "SELECT titular FROM cuentas ORDER BY saldo DESC")
		for index := range want {
			if rows[index][0].Data != descendente[index][0].Data {
				t.Errorf("fila %d: -saldo ASC dio %v y saldo DESC dio %v",
					index, rows[index][0].Data, descendente[index][0].Data)
			}
		}
	})

	t.Run("agregado negado", func(t *testing.T) {
		rows := consultar(t, motor, "SELECT -SUM(saldo) AS invertido FROM cuentas")
		if len(rows) != 1 || rows[0][0].Data != 150.0 {
			t.Errorf("suma invertida = %v; se esperaba 150", rows[0][0].Data)
		}
	})

	t.Run("having con signo", func(t *testing.T) {
		rows := consultar(t, motor, "SELECT titular FROM cuentas GROUP BY titular HAVING -SUM(saldo) > 0 ORDER BY titular")
		if len(rows) != 2 {
			t.Fatalf("filas = %d; se esperaban 2 (los saldos negativos)", len(rows))
		}
		if rows[0][0].Data != "Beto" || rows[1][0].Data != "Carla" {
			t.Errorf("filas = %v, %v", rows[0][0].Data, rows[1][0].Data)
		}
	})

	t.Run("signo sobre un texto", func(t *testing.T) {
		result, err := motor.Query("SELECT -titular FROM cuentas")
		if err != nil {
			t.Fatalf("Query devolvio error: %v", err)
		}
		defer result.Close()
		if _, err := result.Next(); err == nil {
			t.Error("no se rechazo la negacion de un texto")
		}
	})
}

func TestConsultaRechazaFuncionesDesconocidas(t *testing.T) {
	motor := nuevoMotor(t, map[string]string{"empleados": empleadosCSV})

	_, err := motor.Query("SELECT stddev(edad) FROM empleados")
	if err == nil {
		t.Fatal("Query no devolvio error para una funcion desconocida")
	}
	if !strings.Contains(err.Error(), "no existe") {
		t.Errorf("error = %v; se esperaba que dijera que no existe", err)
	}
}
