package parser

import (
	"strings"
)

// Format devuelve la forma canonica de una expresion. Se usa para tres cosas:
// nombrar una columna de salida que no lleva alias, nombrar las columnas que
// produce la agregacion, y reconocer en el planner que dos expresiones son la
// misma (por ejemplo, una clave de GROUP BY citada en el SELECT).
//
// Los operandos de una operacion binaria que a su vez son binarios se escriben
// entre parentesis, de modo que dos expresiones distintas nunca comparten
// representacion.
func Format(expression Expression) string {
	var builder strings.Builder
	writeExpression(&builder, expression)
	return builder.String()
}

func writeExpression(builder *strings.Builder, expression Expression) {
	switch expression := expression.(type) {
	case Identifier:
		builder.WriteString(expression.Name)
	case Literal:
		writeLiteral(builder, expression)
	case FunctionCall:
		writeFunctionCall(builder, expression)
	case BinaryExpression:
		writeOperand(builder, expression.Left)
		builder.WriteString(" ")
		builder.WriteString(expression.Operator.String())
		builder.WriteString(" ")
		writeOperand(builder, expression.Right)
	case UnaryExpression:
		builder.WriteString(expression.Operator.String())
		writeOperand(builder, expression.Operand)
	default:
		builder.WriteString("?")
	}
}

// writeOperand encierra entre parentesis los operandos compuestos.
func writeOperand(builder *strings.Builder, expression Expression) {
	switch expression.(type) {
	case BinaryExpression, UnaryExpression:
		builder.WriteString("(")
		writeExpression(builder, expression)
		builder.WriteString(")")
	default:
		writeExpression(builder, expression)
	}
}

func writeLiteral(builder *strings.Builder, literal Literal) {
	if literal.Kind == LiteralString {
		builder.WriteString("'")
		builder.WriteString(strings.ReplaceAll(literal.Value, "'", "''"))
		builder.WriteString("'")
		return
	}
	builder.WriteString(literal.Value)
}

func writeFunctionCall(builder *strings.Builder, call FunctionCall) {
	builder.WriteString(strings.ToUpper(call.Name))
	builder.WriteString("(")
	if call.Star {
		builder.WriteString("*")
	}
	for index, argument := range call.Args {
		if index > 0 {
			builder.WriteString(", ")
		}
		writeExpression(builder, argument)
	}
	builder.WriteString(")")
}
