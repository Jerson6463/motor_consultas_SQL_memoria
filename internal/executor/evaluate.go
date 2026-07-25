package executor

import (
	"fmt"
	"strconv"
	"strings"

	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

// EvaluatePredicate evalua una condicion WHERE o HAVING. Un resultado NULL no
// selecciona la fila.
func EvaluatePredicate(expression parser.Expression, row storage.Row, columns []storage.Column) (bool, error) {
	value, err := evaluate(expression, row, columns)
	if err != nil {
		return false, err
	}
	if value.Null {
		return false, nil
	}
	if value.Type != storage.Boolean {
		return false, fmt.Errorf("la condicion no produce un booleano")
	}
	return value.Data.(bool), nil
}

func evaluate(expression parser.Expression, row storage.Row, columns []storage.Column) (storage.Value, error) {
	switch expression := expression.(type) {
	case parser.Identifier:
		index, _, err := findColumn(columns, expression.Name)
		if err != nil {
			return storage.Value{}, err
		}
		return row[index], nil
	case parser.Literal:
		return literalValue(expression)
	case parser.BinaryExpression:
		if expression.Operator == parser.OpAnd || expression.Operator == parser.OpOr {
			return logicalValue(expression, row, columns)
		}
		if expression.Operator.IsArithmetic() {
			return arithmeticValue(expression, row, columns)
		}
		return comparisonValue(expression, row, columns)
	case parser.UnaryExpression:
		return unaryValue(expression, row, columns)
	case parser.FunctionCall:
		// El planner sustituye las llamadas de agregacion por referencias a las
		// columnas que produce la agregacion, asi que aqui solo llegan nombres
		// que no estan registrados.
		return storage.Value{}, fmt.Errorf("la funcion %q no existe", expression.Name)
	default:
		return storage.Value{}, fmt.Errorf("expresion no soportada")
	}
}

func literalValue(literal parser.Literal) (storage.Value, error) {
	switch literal.Kind {
	case parser.LiteralString:
		return storage.Value{Type: storage.Text, Data: literal.Value}, nil
	case parser.LiteralBoolean:
		value, err := strconv.ParseBool(strings.ToLower(literal.Value))
		return storage.Value{Type: storage.Boolean, Data: value}, err
	case parser.LiteralNumber:
		if integer, err := strconv.ParseInt(literal.Value, 10, 64); err == nil {
			return storage.Value{Type: storage.Integer, Data: integer}, nil
		}
		decimal, err := strconv.ParseFloat(literal.Value, 64)
		return storage.Value{Type: storage.Decimal, Data: decimal}, err
	case parser.LiteralNull:
		return storage.Value{Null: true}, nil
	default:
		return storage.Value{}, fmt.Errorf("literal no soportado")
	}
}

func logicalValue(expression parser.BinaryExpression, row storage.Row, columns []storage.Column) (storage.Value, error) {
	left, err := evaluate(expression.Left, row, columns)
	if err != nil {
		return storage.Value{}, err
	}
	right, err := evaluate(expression.Right, row, columns)
	if err != nil {
		return storage.Value{}, err
	}
	if (left.Null || left.Type != storage.Boolean) && !left.Null {
		return storage.Value{}, fmt.Errorf("el operando izquierdo de %s no es booleano", expression.Operator)
	}
	if (right.Null || right.Type != storage.Boolean) && !right.Null {
		return storage.Value{}, fmt.Errorf("el operando derecho de %s no es booleano", expression.Operator)
	}

	if expression.Operator == parser.OpAnd {
		if (!left.Null && !left.Data.(bool)) || (!right.Null && !right.Data.(bool)) {
			return storage.Value{Type: storage.Boolean, Data: false}, nil
		}
	} else if (!left.Null && left.Data.(bool)) || (!right.Null && right.Data.(bool)) {
		return storage.Value{Type: storage.Boolean, Data: true}, nil
	}
	if left.Null || right.Null {
		return storage.Value{Type: storage.Boolean, Null: true}, nil
	}
	return storage.Value{Type: storage.Boolean, Data: expression.Operator == parser.OpAnd}, nil
}

func comparisonValue(expression parser.BinaryExpression, row storage.Row, columns []storage.Column) (storage.Value, error) {
	left, err := evaluate(expression.Left, row, columns)
	if err != nil {
		return storage.Value{}, err
	}
	right, err := evaluate(expression.Right, row, columns)
	if err != nil {
		return storage.Value{}, err
	}
	if left.Null || right.Null {
		return storage.Value{Type: storage.Boolean, Null: true}, nil
	}

	comparison, err := storage.Compare(left, right)
	if err != nil {
		return storage.Value{}, err
	}
	result := false
	switch expression.Operator {
	case parser.OpEqual:
		result = comparison == 0
	case parser.OpNotEqual:
		result = comparison != 0
	case parser.OpLess:
		result = comparison < 0
	case parser.OpGreater:
		result = comparison > 0
	case parser.OpLessEqual:
		result = comparison <= 0
	case parser.OpGreaterEqual:
		result = comparison >= 0
	}
	return storage.Value{Type: storage.Boolean, Data: result}, nil
}

// arithmeticValue evalua una operacion aritmetica. Entero con entero da entero,
// cualquier decimal da decimal y la division siempre da decimal. Un operando
// NULL propaga NULL.
func arithmeticValue(expression parser.BinaryExpression, row storage.Row, columns []storage.Column) (storage.Value, error) {
	left, err := evaluate(expression.Left, row, columns)
	if err != nil {
		return storage.Value{}, err
	}
	right, err := evaluate(expression.Right, row, columns)
	if err != nil {
		return storage.Value{}, err
	}

	resultType := storage.Decimal
	if expression.Operator != parser.OpDivide && left.Type == storage.Integer && right.Type == storage.Integer {
		resultType = storage.Integer
	}
	if left.Null || right.Null {
		return storage.Value{Type: resultType, Null: true}, nil
	}
	if !storage.IsNumber(left.Type) || !storage.IsNumber(right.Type) {
		return storage.Value{}, fmt.Errorf("el operador %s requiere valores numericos", expression.Operator)
	}

	leftNumber, rightNumber := storage.AsFloat(left), storage.AsFloat(right)
	if expression.Operator == parser.OpDivide && rightNumber == 0 {
		return storage.Value{}, fmt.Errorf("division por cero")
	}

	var result float64
	switch expression.Operator {
	case parser.OpAdd:
		result = leftNumber + rightNumber
	case parser.OpSubtract:
		result = leftNumber - rightNumber
	case parser.OpMultiply:
		result = leftNumber * rightNumber
	default:
		result = leftNumber / rightNumber
	}

	if resultType == storage.Integer {
		return storage.Value{Type: storage.Integer, Data: int64(result)}, nil
	}
	return storage.Value{Type: storage.Decimal, Data: result}, nil
}

// unaryValue aplica el signo a un valor numerico. NULL se propaga conservando
// el tipo del operando.
func unaryValue(expression parser.UnaryExpression, row storage.Row, columns []storage.Column) (storage.Value, error) {
	value, err := evaluate(expression.Operand, row, columns)
	if err != nil {
		return storage.Value{}, err
	}
	if value.Null {
		return storage.Value{Type: value.Type, Null: true}, nil
	}
	if !storage.IsNumber(value.Type) {
		return storage.Value{}, fmt.Errorf("el operador %s requiere un valor numerico", expression.Operator)
	}
	if expression.Operator == parser.OpPositive {
		return value, nil
	}
	if value.Type == storage.Integer {
		return storage.Value{Type: storage.Integer, Data: -value.Data.(int64)}, nil
	}
	return storage.Value{Type: storage.Decimal, Data: -value.Data.(float64)}, nil
}

// expressionType deduce el tipo de una expresion sin evaluarla. Solo sirve para
// declarar el esquema de salida; los errores de tipo reales afloran al evaluar.
func expressionType(expression parser.Expression, columns []storage.Column) (storage.DataType, error) {
	switch expression := expression.(type) {
	case parser.Identifier:
		_, column, err := findColumn(columns, expression.Name)
		if err != nil {
			return storage.Text, err
		}
		return column.Type, nil
	case parser.Literal:
		value, err := literalValue(expression)
		if err != nil {
			return storage.Text, err
		}
		return value.Type, nil
	case parser.BinaryExpression:
		if !expression.Operator.IsArithmetic() {
			return storage.Boolean, nil
		}
		if expression.Operator == parser.OpDivide {
			return storage.Decimal, nil
		}
		left, err := expressionType(expression.Left, columns)
		if err != nil {
			return storage.Text, err
		}
		right, err := expressionType(expression.Right, columns)
		if err != nil {
			return storage.Text, err
		}
		if left == storage.Integer && right == storage.Integer {
			return storage.Integer, nil
		}
		return storage.Decimal, nil
	case parser.UnaryExpression:
		return expressionType(expression.Operand, columns)
	case parser.FunctionCall:
		return storage.Text, fmt.Errorf("la funcion %q no existe", expression.Name)
	default:
		return storage.Text, fmt.Errorf("expresion no soportada")
	}
}
