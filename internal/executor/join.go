package executor

import (
	"io"

	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

// NestedLoopJoin compara cada fila izquierda contra las filas derechas.
// Se conserva como implementacion de referencia: el plan usa HashJoin.
type NestedLoopJoin struct {
	left      Operator
	rightRows []storage.Row
	condition parser.Expression
	columns   []storage.Column
	current   storage.Row
	index     int
}

func NewNestedLoopJoin(left, right Operator, condition parser.Expression) (*NestedLoopJoin, error) {
	rightRows := []storage.Row{}
	for {
		row, err := right.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		rightRows = append(rightRows, row)
	}

	columns := make([]storage.Column, 0, len(left.Columns())+len(right.Columns()))
	columns = append(columns, left.Columns()...)
	columns = append(columns, right.Columns()...)
	return &NestedLoopJoin{left: left, rightRows: rightRows, condition: condition, columns: columns}, nil
}

func (j *NestedLoopJoin) Next() (storage.Row, error) {
	for {
		if j.current == nil {
			row, err := j.left.Next()
			if err != nil {
				return nil, err
			}
			j.current = row
			j.index = 0
		}
		for j.index < len(j.rightRows) {
			right := j.rightRows[j.index]
			j.index++
			row := append(append(storage.Row{}, j.current...), right...)
			matches, err := EvaluatePredicate(j.condition, row, j.columns)
			if err != nil {
				return nil, err
			}
			if matches {
				return row, nil
			}
		}
		j.current = nil
	}
}

func (j *NestedLoopJoin) Columns() []storage.Column { return j.columns }
func (j *NestedLoopJoin) Close() error              { return j.left.Close() }
