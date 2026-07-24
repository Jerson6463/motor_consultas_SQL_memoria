package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"motor-consultas-sql/internal/engine"
)

// render recorre el resultado e imprime las filas en columnas alineadas.
// Se consume de forma perezosa: nunca se materializa el resultado completo.
func render(output io.Writer, result *engine.Result) error {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	for _, column := range result.Columns() {
		fmt.Fprintf(writer, "%s\t", column.Name)
	}
	fmt.Fprintln(writer)

	for {
		row, err := result.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		for _, value := range row {
			if value.Null {
				fmt.Fprint(writer, "NULL\t")
				continue
			}
			fmt.Fprintf(writer, "%v\t", value.Data)
		}
		fmt.Fprintln(writer)
	}
	return writer.Flush()
}
