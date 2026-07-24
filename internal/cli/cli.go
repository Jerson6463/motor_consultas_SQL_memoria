// Package cli interpreta los argumentos de linea de comandos y muestra los
// resultados. Es la unica capa que conoce la entrada y la salida del programa.
package cli

import (
	"fmt"
	"io"
	"strings"

	"motor-consultas-sql/internal/engine"
)

// Run ejecuta un subcomando y devuelve el codigo de salida del proceso.
func Run(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) < 1 {
		printUsage(stderr)
		return 1
	}

	switch arguments[0] {
	case "cargar":
		return load(arguments[1:], stdout, stderr)
	case "consultar":
		return query(arguments[1:], stdout, stderr)
	default:
		printUsage(stderr)
		return 1
	}
}

// load carga un CSV e imprime el esquema inferido.
func load(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) != 2 {
		fmt.Fprintln(stderr, "Uso: sqlmem cargar <tabla> <archivo.csv>")
		return 1
	}
	table, err := engine.New().LoadCSVFile(arguments[0], arguments[1])
	if err != nil {
		return reportError(stderr, err)
	}

	fmt.Fprintf(stdout, "Tabla %q cargada: %d filas\n", table.Name, len(table.Rows))
	for _, column := range table.Columns {
		fmt.Fprintf(stdout, "- %s: %s\n", column.Name, column.Type)
	}
	return 0
}

// query admite dos formas: una sola tabla con <tabla> <archivo.csv> <SQL>, o
// varias tablas como tabla=archivo.csv separadas del SQL por --.
func query(arguments []string, stdout, stderr io.Writer) int {
	if len(arguments) != 3 {
		return queryMultiple(arguments, stdout, stderr)
	}

	motor := engine.New()
	if _, err := motor.LoadCSVFile(arguments[0], arguments[1]); err != nil {
		return reportError(stderr, err)
	}
	return runQuery(motor, arguments[2], stdout, stderr)
}

func queryMultiple(arguments []string, stdout, stderr io.Writer) int {
	separator := -1
	for index, argument := range arguments {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 1 || separator != len(arguments)-2 {
		fmt.Fprintln(stderr, "Uso: sqlmem consultar <tabla=archivo.csv> [tabla=archivo.csv ...] -- <consulta SQL>")
		return 1
	}

	motor := engine.New()
	for _, source := range arguments[:separator] {
		parts := strings.SplitN(source, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			fmt.Fprintf(stderr, "Error: fuente invalida %q; use tabla=archivo.csv\n", source)
			return 1
		}
		if _, err := motor.LoadCSVFile(parts[0], parts[1]); err != nil {
			return reportError(stderr, err)
		}
	}
	return runQuery(motor, arguments[separator+1], stdout, stderr)
}

func runQuery(motor *engine.Engine, sql string, stdout, stderr io.Writer) int {
	result, err := motor.Query(sql)
	if err != nil {
		return reportError(stderr, err)
	}
	defer result.Close()

	if err := render(stdout, result); err != nil {
		return reportError(stderr, err)
	}
	return 0
}

func reportError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "Error: %v\n", err)
	return 1
}

func printUsage(stderr io.Writer) {
	fmt.Fprintln(stderr, "Uso:")
	fmt.Fprintln(stderr, "  sqlmem cargar <tabla> <archivo.csv>")
	fmt.Fprintln(stderr, "  sqlmem consultar <tabla> <archivo.csv> <consulta SQL>")
	fmt.Fprintln(stderr, "  sqlmem consultar <tabla=archivo.csv> [tabla=archivo.csv ...] -- <consulta SQL>")
}
