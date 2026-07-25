package executor

import (
	"fmt"
	"io"
	"strings"

	"motor-consultas-sql/internal/functions"
	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

// Aggregate agrupa las filas y calcula una funcion por cada llamada. Sus
// columnas de salida son las expresiones de agrupacion seguidas de las
// llamadas, nombradas con parser.Format; el planner reescribe el SELECT para
// que apunte a esos nombres.
type Aggregate struct {
	input     Operator
	groups    []parser.Expression
	calls     []parser.FunctionCall
	specs     []functions.Spec
	arguments []storage.DataType
	columns   []storage.Column
	rows      []storage.Row
	loaded    bool
	index     int
}

type aggregateState struct {
	group        storage.Row
	accumulators []functions.Accumulator
}

func NewAggregate(input Operator, groups []parser.Expression, calls []parser.FunctionCall) (*Aggregate, error) {
	a := &Aggregate{input: input, groups: groups, calls: calls}
	inputColumns := input.Columns()

	for _, expression := range groups {
		dataType, err := expressionType(expression, inputColumns)
		if err != nil {
			return nil, err
		}
		a.columns = append(a.columns, storage.Column{Name: parser.Format(expression), Type: dataType})
	}

	for _, call := range calls {
		spec, ok := functions.Lookup(call.Name)
		if !ok {
			return nil, fmt.Errorf("la funcion %q no existe", call.Name)
		}
		// COUNT(*) no tiene argumento: se cuenta una unidad por fila.
		argument := storage.Integer
		if !call.Star {
			if len(call.Args) != 1 {
				return nil, fmt.Errorf("%s requiere exactamente un argumento", strings.ToUpper(call.Name))
			}
			dataType, err := expressionType(call.Args[0], inputColumns)
			if err != nil {
				return nil, err
			}
			argument = dataType
		}
		a.specs = append(a.specs, spec)
		a.arguments = append(a.arguments, argument)
		a.columns = append(a.columns, storage.Column{Name: parser.Format(call), Type: spec.ResultType(argument)})
	}
	return a, nil
}

func (a *Aggregate) Next() (storage.Row, error) {
	if !a.loaded {
		if err := a.load(); err != nil {
			return nil, err
		}
	}
	if a.index >= len(a.rows) {
		return nil, io.EOF
	}
	row := a.rows[a.index]
	a.index++
	return row, nil
}

func (a *Aggregate) Columns() []storage.Column { return a.columns }
func (a *Aggregate) Close() error              { return a.input.Close() }

func (a *Aggregate) load() error {
	states := map[string]*aggregateState{}
	order := []string{}
	columns := a.input.Columns()

	for {
		row, err := a.input.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		key, group, err := a.groupOf(row, columns)
		if err != nil {
			return err
		}
		state := states[key]
		if state == nil {
			state = &aggregateState{group: group, accumulators: a.newAccumulators()}
			states[key] = state
			order = append(order, key)
		}

		for index, call := range a.calls {
			value := storage.Value{Type: storage.Integer, Data: int64(1)}
			if !call.Star {
				value, err = evaluate(call.Args[0], row, columns)
				if err != nil {
					return err
				}
			}
			if err := state.accumulators[index].Add(value); err != nil {
				return err
			}
		}
	}

	// Sin GROUP BY, una tabla vacia produce igualmente una fila de resultados.
	if len(order) == 0 && len(a.groups) == 0 {
		states[""] = &aggregateState{accumulators: a.newAccumulators()}
		order = []string{""}
	}

	for _, key := range order {
		state := states[key]
		row := append(storage.Row{}, state.group...)
		for _, accumulator := range state.accumulators {
			row = append(row, accumulator.Result())
		}
		a.rows = append(a.rows, row)
	}
	a.loaded = true
	return nil
}

func (a *Aggregate) newAccumulators() []functions.Accumulator {
	accumulators := make([]functions.Accumulator, len(a.specs))
	for index, spec := range a.specs {
		accumulators[index] = spec.New(a.arguments[index])
	}
	return accumulators
}

// groupOf evalua las expresiones de agrupacion y devuelve la clave del grupo
// junto con sus valores.
func (a *Aggregate) groupOf(row storage.Row, columns []storage.Column) (string, storage.Row, error) {
	values := make(storage.Row, len(a.groups))
	parts := make([]string, len(a.groups))
	for index, expression := range a.groups {
		value, err := evaluate(expression, row, columns)
		if err != nil {
			return "", nil, err
		}
		values[index] = value
		parts[index] = fmt.Sprintf("%d:%v:%t", value.Type, value.Data, value.Null)
	}
	return strings.Join(parts, "|"), values, nil
}
