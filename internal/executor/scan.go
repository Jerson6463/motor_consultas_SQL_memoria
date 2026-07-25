package executor

import (
	"io"

	"motor-consultas-sql/internal/storage"
)

// Scan recorre secuencialmente las filas de una tabla en memoria.
type Scan struct {
	table   *storage.Table
	columns []storage.Column
	index   int
}

// NewScan crea un escaneo para una tabla. Si alias no esta vacio, las columnas
// se publican como alias.columna; asi cada columna queda calificada una sola
// vez por muchos joins que tenga la consulta.
func NewScan(table *storage.Table, alias string) *Scan {
	columns := table.Columns
	if alias != "" {
		columns = make([]storage.Column, len(table.Columns))
		for index, column := range table.Columns {
			column.Name = alias + "." + column.Name
			columns[index] = column
		}
	}
	return &Scan{table: table, columns: columns}
}

// Next entrega la siguiente fila de la tabla.
func (s *Scan) Next() (storage.Row, error) {
	if s.index >= len(s.table.Rows) {
		return nil, io.EOF
	}
	row := s.table.Rows[s.index]
	s.index++
	return row, nil
}

// Columns devuelve el esquema de la tabla.
func (s *Scan) Columns() []storage.Column {
	return s.columns
}

// Close no requiere liberar recursos para una tabla en memoria.
func (s *Scan) Close() error {
	return nil
}
