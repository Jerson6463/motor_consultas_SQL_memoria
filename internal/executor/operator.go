package executor

import "motor-consultas-sql/internal/storage"

// Operator entrega filas una por una. Retorna io.EOF cuando no hay mas filas.
type Operator interface {
	Next() (storage.Row, error)
	Columns() []storage.Column
	Close() error
}
