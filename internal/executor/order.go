package executor

import (
	"io"
	"sort"

	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

// Order materializa y ordena las filas del operador de entrada. Va por debajo
// de la proyeccion, de modo que puede ordenar por expresiones que no aparecen
// en el SELECT.
type Order struct {
	input   Operator
	terms   []parser.SortTerm
	entries []sortEntry
	index   int
	loaded  bool
}

// sortEntry guarda una fila junto a las claves de ordenamiento ya evaluadas,
// para no volver a calcularlas en cada comparacion.
type sortEntry struct {
	row  storage.Row
	keys []storage.Value
}

// NewOrder crea un operador de ordenamiento.
func NewOrder(input Operator, terms []parser.SortTerm) (*Order, error) {
	columns := input.Columns()
	for _, term := range terms {
		if _, err := expressionType(term.Expression, columns); err != nil {
			return nil, err
		}
	}
	return &Order{input: input, terms: terms}, nil
}

func (o *Order) Next() (storage.Row, error) {
	if !o.loaded {
		if err := o.load(); err != nil {
			return nil, err
		}
	}
	if o.index >= len(o.entries) {
		return nil, io.EOF
	}
	row := o.entries[o.index].row
	o.index++
	return row, nil
}

func (o *Order) Columns() []storage.Column { return o.input.Columns() }
func (o *Order) Close() error              { return o.input.Close() }

func (o *Order) load() error {
	columns := o.input.Columns()
	for {
		row, err := o.input.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		keys := make([]storage.Value, len(o.terms))
		for index, term := range o.terms {
			value, err := evaluate(term.Expression, row, columns)
			if err != nil {
				return err
			}
			keys[index] = value
		}
		o.entries = append(o.entries, sortEntry{row: row, keys: keys})
	}

	var sortErr error
	sort.SliceStable(o.entries, func(leftIndex, rightIndex int) bool {
		if sortErr != nil {
			return false
		}
		left, right := o.entries[leftIndex], o.entries[rightIndex]
		for termIndex := range o.terms {
			comparison, err := compareForOrder(left.keys[termIndex], right.keys[termIndex])
			if err != nil {
				sortErr = err
				return false
			}
			if comparison == 0 {
				continue
			}
			if o.terms[termIndex].Descending {
				return comparison > 0
			}
			return comparison < 0
		}
		return false
	})
	if sortErr != nil {
		return sortErr
	}
	o.loaded = true
	return nil
}
