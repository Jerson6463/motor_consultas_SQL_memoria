package storage

import "fmt"

// Table es una tabla cargada por completo en memoria.
type Table struct {
	Name    string
	Columns []Column
	Rows    []Row
}

// ColumnIndex devuelve la posicion de una columna.
func (t *Table) ColumnIndex(name string) (int, error) {
	for index, column := range t.Columns {
		if NormalizeName(column.Name) == NormalizeName(name) {
			return index, nil
		}
	}
	return 0, fmt.Errorf("la columna %q no existe en la tabla %q", name, t.Name)
}
