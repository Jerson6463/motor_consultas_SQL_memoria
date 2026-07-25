// Package storage define la representacion en memoria de los datos: tipos,
// valores, filas y tablas, junto con su carga desde CSV.
package storage

import "strings"

// DataType representa los tipos soportados por el motor.
type DataType uint8

const (
	Text DataType = iota
	Integer
	Decimal
	Boolean
)

func (t DataType) String() string {
	switch t {
	case Integer:
		return "entero"
	case Decimal:
		return "decimal"
	case Boolean:
		return "booleano"
	default:
		return "texto"
	}
}

// Value guarda un valor tipado. Null indica que no tiene valor.
type Value struct {
	Type DataType
	Data any
	Null bool
}

// NormalizeName normaliza un nombre de tabla o columna. Es la regla unica de
// insensibilidad a mayusculas que comparten el catalogo y las tablas.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
