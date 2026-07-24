// Package functions mantiene el registro de funciones de agregacion. Anadir una
// funcion nueva consiste en registrar aqui su especificacion: ni el lexer ni el
// parser conocen los nombres de las funciones.
package functions

import (
	"fmt"
	"strings"

	"motor-consultas-sql/internal/storage"
)

// Accumulator acumula los valores de un grupo y produce su resultado.
type Accumulator interface {
	// Add incorpora un valor al grupo. Cada funcion decide que hacer con los
	// nulos; por convencion se ignoran.
	Add(value storage.Value) error
	// Result devuelve el valor acumulado del grupo.
	Result() storage.Value
}

// Spec describe una funcion de agregacion.
type Spec struct {
	Name string
	// AcceptsStar indica si la funcion admite la forma FUNCION(*).
	AcceptsStar bool
	// ResultType calcula el tipo de la columna de salida a partir del tipo del
	// argumento.
	ResultType func(argument storage.DataType) storage.DataType
	// New crea un acumulador para un grupo.
	New func(argument storage.DataType) Accumulator
}

var registry = map[string]Spec{}

func register(spec Spec) {
	registry[strings.ToUpper(spec.Name)] = spec
}

// Lookup busca una funcion por nombre, sin distinguir mayusculas de minusculas.
func Lookup(name string) (Spec, bool) {
	spec, ok := registry[strings.ToUpper(strings.TrimSpace(name))]
	return spec, ok
}

// IsAggregate indica si un nombre corresponde a una funcion de agregacion.
// Lo usa el planner para separar los agregados del resto de la lista SELECT.
func IsAggregate(name string) bool {
	_, ok := Lookup(name)
	return ok
}

// Names devuelve los nombres registrados, en orden alfabetico.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

func init() {
	register(Spec{
		Name:        "COUNT",
		AcceptsStar: true,
		ResultType:  func(storage.DataType) storage.DataType { return storage.Integer },
		New:         func(storage.DataType) Accumulator { return &countAccumulator{} },
	})
	register(Spec{
		Name:       "SUM",
		ResultType: func(storage.DataType) storage.DataType { return storage.Decimal },
		New:        func(storage.DataType) Accumulator { return &sumAccumulator{name: "SUM"} },
	})
	register(Spec{
		Name:       "AVG",
		ResultType: func(storage.DataType) storage.DataType { return storage.Decimal },
		New:        func(storage.DataType) Accumulator { return &sumAccumulator{name: "AVG", average: true} },
	})
	register(Spec{
		Name:       "MIN",
		ResultType: func(argument storage.DataType) storage.DataType { return argument },
		New: func(argument storage.DataType) Accumulator {
			return &extremeAccumulator{argument: argument, keepGreater: false}
		},
	})
	register(Spec{
		Name:       "MAX",
		ResultType: func(argument storage.DataType) storage.DataType { return argument },
		New: func(argument storage.DataType) Accumulator {
			return &extremeAccumulator{argument: argument, keepGreater: true}
		},
	})
}

// countAccumulator cuenta los valores no nulos. Para COUNT(*) el executor
// entrega un valor no nulo por fila, de modo que se cuentan todas.
type countAccumulator struct {
	count int64
}

func (a *countAccumulator) Add(value storage.Value) error {
	if !value.Null {
		a.count++
	}
	return nil
}

func (a *countAccumulator) Result() storage.Value {
	return storage.Value{Type: storage.Integer, Data: a.count}
}

// sumAccumulator suma los valores no nulos y, si average esta activo, divide
// entre la cantidad de valores sumados.
type sumAccumulator struct {
	name    string
	average bool
	sum     float64
	count   int
}

func (a *sumAccumulator) Add(value storage.Value) error {
	if value.Null {
		return nil
	}
	if !storage.IsNumber(value.Type) {
		return fmt.Errorf("%s requiere una columna numerica", a.name)
	}
	a.count++
	a.sum += storage.AsFloat(value)
	return nil
}

func (a *sumAccumulator) Result() storage.Value {
	if a.count == 0 {
		return storage.Value{Type: storage.Decimal, Null: true}
	}
	if a.average {
		return storage.Value{Type: storage.Decimal, Data: a.sum / float64(a.count)}
	}
	return storage.Value{Type: storage.Decimal, Data: a.sum}
}

// extremeAccumulator conserva el menor o el mayor de los valores no nulos.
type extremeAccumulator struct {
	argument    storage.DataType
	keepGreater bool
	value       storage.Value
	seen        bool
}

func (a *extremeAccumulator) Add(value storage.Value) error {
	if value.Null {
		return nil
	}
	if !a.seen {
		a.value = value
		a.seen = true
		return nil
	}
	comparison, err := storage.Compare(value, a.value)
	if err != nil {
		return err
	}
	if (a.keepGreater && comparison > 0) || (!a.keepGreater && comparison < 0) {
		a.value = value
	}
	return nil
}

func (a *extremeAccumulator) Result() storage.Value {
	if !a.seen {
		return storage.Value{Type: a.argument, Null: true}
	}
	return a.value
}
