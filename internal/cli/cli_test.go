package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// escribirCSV crea un CSV temporal y devuelve su ruta.
func escribirCSV(t *testing.T, nombre, contenido string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), nombre)
	if err := os.WriteFile(path, []byte(contenido), 0o600); err != nil {
		t.Fatalf("no se pudo escribir %q: %v", path, err)
	}
	return path
}

// ejecutar corre el CLI capturando ambas salidas.
func ejecutar(arguments ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(arguments, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestRunSinArgumentosMuestraElUso(t *testing.T) {
	code, stdout, stderr := ejecutar()
	if code == 0 {
		t.Error("se esperaba un codigo de salida distinto de cero")
	}
	if stdout != "" {
		t.Errorf("stdout = %q; se esperaba vacio", stdout)
	}
	if !strings.Contains(stderr, "sqlmem cargar") || !strings.Contains(stderr, "sqlmem consultar") {
		t.Errorf("el uso no describe los subcomandos: %q", stderr)
	}
}

func TestRunSubcomandoDesconocido(t *testing.T) {
	code, _, stderr := ejecutar("borrar")
	if code == 0 {
		t.Error("se esperaba un codigo de salida distinto de cero")
	}
	if !strings.Contains(stderr, "Uso:") {
		t.Errorf("no se mostro el uso: %q", stderr)
	}
}

func TestRunCargarMuestraElEsquema(t *testing.T) {
	path := escribirCSV(t, "empleados.csv", "id,nombre,activo,salario\n7,Ana,true,2500.50\n9,Beto,false,3200.00\n")

	code, stdout, stderr := ejecutar("cargar", "empleados", path)
	if code != 0 {
		t.Fatalf("codigo = %d; stderr = %q", code, stderr)
	}
	for _, want := range []string{`Tabla "empleados" cargada: 2 filas`, "- id: entero", "- nombre: texto", "- activo: booleano", "- salario: decimal"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("la salida no incluye %q: %q", want, stdout)
		}
	}
}

func TestRunCargarConArgumentosIncorrectos(t *testing.T) {
	code, _, stderr := ejecutar("cargar", "empleados")
	if code == 0 {
		t.Error("se esperaba un codigo de salida distinto de cero")
	}
	if !strings.Contains(stderr, "Uso: sqlmem cargar") {
		t.Errorf("no se mostro el uso de cargar: %q", stderr)
	}
}

func TestRunCargarArchivoInexistente(t *testing.T) {
	code, _, stderr := ejecutar("cargar", "empleados", filepath.Join(t.TempDir(), "noexiste.csv"))
	if code == 0 {
		t.Error("se esperaba un codigo de salida distinto de cero")
	}
	if !strings.HasPrefix(stderr, "Error: ") {
		t.Errorf("stderr = %q; se esperaba que empezara por 'Error: '", stderr)
	}
}

func TestRunConsultarUnaTabla(t *testing.T) {
	path := escribirCSV(t, "empleados.csv", "nombre,edad\nAna,30\nBeto,15\n")

	code, stdout, stderr := ejecutar("consultar", "empleados", path, "SELECT nombre FROM empleados WHERE edad >= 18")
	if code != 0 {
		t.Fatalf("codigo = %d; stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "nombre") || !strings.Contains(stdout, "Ana") {
		t.Errorf("salida inesperada: %q", stdout)
	}
	if strings.Contains(stdout, "Beto") {
		t.Errorf("el filtro no se aplico: %q", stdout)
	}
}

func TestRunConsultarMuestraLosNulos(t *testing.T) {
	path := escribirCSV(t, "empleados.csv", "nombre,edad\nAna,\n")

	code, stdout, stderr := ejecutar("consultar", "empleados", path, "SELECT nombre, edad FROM empleados")
	if code != 0 {
		t.Fatalf("codigo = %d; stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "NULL") {
		t.Errorf("no se imprimio NULL: %q", stdout)
	}
}

func TestRunConsultarVariasTablas(t *testing.T) {
	directorio := t.TempDir()
	empleados := filepath.Join(directorio, "empleados.csv")
	areas := filepath.Join(directorio, "areas.csv")
	if err := os.WriteFile(empleados, []byte("nombre,area_id\nAna,1\n"), 0o600); err != nil {
		t.Fatalf("no se pudo escribir empleados: %v", err)
	}
	if err := os.WriteFile(areas, []byte("id,nombre\n1,Ventas\n"), 0o600); err != nil {
		t.Fatalf("no se pudo escribir areas: %v", err)
	}

	code, stdout, stderr := ejecutar(
		"consultar",
		"empleados="+empleados,
		"areas="+areas,
		"--",
		"SELECT empleados.nombre, areas.nombre FROM empleados INNER JOIN areas ON empleados.area_id = areas.id",
	)
	if code != 0 {
		t.Fatalf("codigo = %d; stderr = %q", code, stderr)
	}
	if !strings.Contains(stdout, "empleados.nombre") || !strings.Contains(stdout, "Ventas") {
		t.Errorf("salida inesperada: %q", stdout)
	}
}

// TestRunConsultarFuenteInvalida usa cuatro argumentos para entrar en el modo
// de varias tablas: con exactamente tres se interpreta <tabla> <archivo> <SQL>.
func TestRunConsultarFuenteInvalida(t *testing.T) {
	path := escribirCSV(t, "areas.csv", "id,nombre\n1,Ventas\n2,Soporte\n")

	code, _, stderr := ejecutar("consultar", "areas="+path, "empleados", "--", "SELECT * FROM areas")
	if code == 0 {
		t.Error("se esperaba un codigo de salida distinto de cero")
	}
	if !strings.Contains(stderr, "fuente invalida") {
		t.Errorf("stderr = %q; se esperaba el aviso de fuente invalida", stderr)
	}
}

// TestRunConsultarSinSeparador comprueba el aviso de uso del modo de varias
// tablas cuando falta el separador --.
func TestRunConsultarSinSeparador(t *testing.T) {
	code, _, stderr := ejecutar("consultar", "empleados", "archivo.csv", "SELECT * FROM empleados", "extra")
	if code == 0 {
		t.Error("se esperaba un codigo de salida distinto de cero")
	}
	if !strings.Contains(stderr, "tabla=archivo.csv") {
		t.Errorf("stderr = %q; se esperaba el uso del modo de varias tablas", stderr)
	}
}

func TestRunConsultarSqlInvalido(t *testing.T) {
	path := escribirCSV(t, "empleados.csv", "nombre\nAna\n")

	code, stdout, stderr := ejecutar("consultar", "empleados", path, "SELECT nombre empleados")
	if code == 0 {
		t.Error("se esperaba un codigo de salida distinto de cero")
	}
	if stdout != "" {
		t.Errorf("stdout = %q; no debe imprimirse ninguna fila", stdout)
	}
	if !strings.Contains(stderr, "posicion") {
		t.Errorf("stderr = %q; se esperaba la posicion del error", stderr)
	}
}
