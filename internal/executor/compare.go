package executor

import (
	"fmt"

	"motor-consultas-sql/internal/storage"
)

// compareForOrder aplica las reglas de ORDER BY: los NULL van al final en ASC.
func compareForOrder(left, right storage.Value) (int, error) {
	if left.Null && right.Null {
		return 0, nil
	}
	if left.Null {
		return 1, nil
	}
	if right.Null {
		return -1, nil
	}
	comparison, err := storage.Compare(left, right)
	if err != nil {
		return 0, fmt.Errorf("ordenar: %w", err)
	}
	return comparison, nil
}
