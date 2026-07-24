package storage

import (
	"fmt"
	"strings"
)

// Compare ordena dos valores no nulos. Los numeros se comparan entre si aunque
// mezclen enteros y decimales; el resto exige tipos iguales.
func Compare(left, right Value) (int, error) {
	if IsNumber(left.Type) && IsNumber(right.Type) {
		leftNumber := AsFloat(left)
		rightNumber := AsFloat(right)
		if leftNumber < rightNumber {
			return -1, nil
		}
		if leftNumber > rightNumber {
			return 1, nil
		}
		return 0, nil
	}
	if left.Type != right.Type {
		return 0, fmt.Errorf("no se puede comparar %s con %s", left.Type, right.Type)
	}

	switch left.Type {
	case Text:
		return strings.Compare(left.Data.(string), right.Data.(string)), nil
	case Boolean:
		leftBoolean, rightBoolean := left.Data.(bool), right.Data.(bool)
		if leftBoolean == rightBoolean {
			return 0, nil
		}
		if !leftBoolean {
			return -1, nil
		}
		return 1, nil
	default:
		return 0, fmt.Errorf("tipo no soportado: %s", left.Type)
	}
}

// IsNumber indica si un tipo admite operaciones aritmeticas.
func IsNumber(dataType DataType) bool {
	return dataType == Integer || dataType == Decimal
}

// AsFloat convierte un valor numerico no nulo a float64.
func AsFloat(value Value) float64 {
	if value.Type == Integer {
		return float64(value.Data.(int64))
	}
	return value.Data.(float64)
}
