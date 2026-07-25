package executor

import (
	"fmt"

	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

// Filter entrega solo las filas que cumplen una condicion. Lo usan tanto WHERE
// como HAVING; la diferencia esta en su posicion dentro del plan.
type Filter struct {
	input     Operator
	condition parser.Expression
}

// NewFilter crea un filtro sobre un operador existente. Comprueba por
// adelantado que la condicion solo use columnas del esquema de entrada.
func NewFilter(input Operator, condition parser.Expression) (*Filter, error) {
	if err := validateExpression(condition, input.Columns()); err != nil {
		return nil, err
	}
	return &Filter{input: input, condition: condition}, nil
}

// Next busca y entrega la siguiente fila que cumple la condicion.
func (f *Filter) Next() (storage.Row, error) {
	for {
		row, err := f.input.Next()
		if err != nil {
			return nil, err
		}

		matches, err := EvaluatePredicate(f.condition, row, f.input.Columns())
		if err != nil {
			return nil, err
		}
		if matches {
			return row, nil
		}
	}
}

// Columns conserva el esquema del operador de entrada.
func (f *Filter) Columns() []storage.Column {
	return f.input.Columns()
}

// Close cierra el operador de entrada.
func (f *Filter) Close() error {
	return f.input.Close()
}

func validateExpression(expression parser.Expression, columns []storage.Column) error {
	switch expression := expression.(type) {
	case parser.Identifier:
		_, _, err := findColumn(columns, expression.Name)
		return err
	case parser.Literal:
		return nil
	case parser.FunctionCall:
		return fmt.Errorf("la funcion %q no existe", expression.Name)
	case parser.BinaryExpression:
		if err := validateExpression(expression.Left, columns); err != nil {
			return err
		}
		return validateExpression(expression.Right, columns)
	case parser.UnaryExpression:
		return validateExpression(expression.Operand, columns)
	default:
		return fmt.Errorf("expresion no soportada")
	}
}

var _ Operator = (*Filter)(nil)
