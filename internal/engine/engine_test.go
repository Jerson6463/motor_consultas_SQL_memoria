package engine

import (
	"io"
	"strings"
	"testing"
)

func TestEngineCargaYConsulta(t *testing.T) {
	motor := New()
	table, err := motor.LoadCSV("empleados", strings.NewReader("nombre,edad\nAna,30\n"))
	if err != nil {
		t.Fatalf("LoadCSV devolvio error: %v", err)
	}
	if table.Name != "empleados" || len(table.Rows) != 1 {
		t.Fatalf("tabla cargada incorrectamente: %#v", table)
	}

	result, err := motor.Query("SELECT nombre FROM empleados")
	if err != nil {
		t.Fatalf("Query devolvio error: %v", err)
	}
	defer result.Close()

	row, err := result.Next()
	if err != nil {
		t.Fatalf("Next devolvio error: %v", err)
	}
	if row[0].Data != "Ana" {
		t.Errorf("fila = %#v; se esperaba Ana", row[0].Data)
	}
	if _, err := result.Next(); err != io.EOF {
		t.Errorf("Next = %v; se esperaba io.EOF", err)
	}
}

func TestEngineRechazaTablasDuplicadas(t *testing.T) {
	motor := New()
	if _, err := motor.LoadCSV("empleados", strings.NewReader("nombre\nAna\n")); err != nil {
		t.Fatalf("LoadCSV devolvio error: %v", err)
	}
	if _, err := motor.LoadCSV("EMPLEADOS", strings.NewReader("nombre\nBeto\n")); err == nil {
		t.Fatal("LoadCSV permitio una tabla duplicada")
	}
}

// TestEnginePropagaErroresDeCadaEtapa comprueba que la fachada devuelve los
// errores de las tres etapas sin envolverlos ni ocultarlos.
func TestEnginePropagaErroresDeCadaEtapa(t *testing.T) {
	motor := New()
	if _, err := motor.LoadCSV("empleados", strings.NewReader("nombre,edad\nAna,30\n")); err != nil {
		t.Fatalf("LoadCSV devolvio error: %v", err)
	}

	tests := []struct {
		name string
		sql  string
		want string
	}{
		{name: "parser", sql: "SELECT nombre empleados", want: "posicion"},
		{name: "planner", sql: "SELECT nombre FROM otra", want: "no existe"},
		{name: "executor", sql: "SELECT sueldo FROM empleados", want: "no existe"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := motor.Query(test.sql)
			if err == nil {
				t.Fatal("Query no devolvio error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %v; se esperaba que incluyera %q", err, test.want)
			}
		})
	}
}

// TestEngineEsPerezoso comprueba que Query no consume filas por adelantado:
// el modelo Volcano exige que se calculen al llamar a Next.
func TestEngineEsPerezoso(t *testing.T) {
	motor := New()
	if _, err := motor.LoadCSV("ventas", strings.NewReader("monto\n10\n20\n30\n")); err != nil {
		t.Fatalf("LoadCSV devolvio error: %v", err)
	}

	result, err := motor.Query("SELECT monto FROM ventas LIMIT 1")
	if err != nil {
		t.Fatalf("Query devolvio error: %v", err)
	}
	defer result.Close()

	if _, err := result.Next(); err != nil {
		t.Fatalf("Next devolvio error: %v", err)
	}
	if _, err := result.Next(); err != io.EOF {
		t.Errorf("Next = %v; se esperaba io.EOF tras el limite", err)
	}
}
