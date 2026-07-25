// Package executor recorre un plan logico ejecutandolo con el modelo Volcano:
// cada operador entrega sus filas de una en una tirando del operador inferior.
package executor

import (
	"fmt"

	"motor-consultas-sql/internal/planner"
)

// Build crea el arbol de operadores fisicos que ejecuta un plan logico.
// Los errores de resolucion de columnas afloran aqui, al construir cada operador.
func Build(node planner.Node) (Operator, error) {
	switch node := node.(type) {
	case *planner.Scan:
		return NewScan(node.Table, node.Alias), nil
	case *planner.Join:
		left, err := Build(node.Left)
		if err != nil {
			return nil, err
		}
		right, err := Build(node.Right)
		if err != nil {
			return nil, err
		}
		// HashJoin es la estrategia activa para las condiciones de igualdad.
		return NewHashJoin(left, right, node.Condition)
	case *planner.Filter:
		input, err := Build(node.Input)
		if err != nil {
			return nil, err
		}
		return NewFilter(input, node.Condition)
	case *planner.Aggregate:
		input, err := Build(node.Input)
		if err != nil {
			return nil, err
		}
		return NewAggregate(input, node.GroupBy, node.Calls)
	case *planner.Project:
		input, err := Build(node.Input)
		if err != nil {
			return nil, err
		}
		return NewProject(input, node.Items)
	case *planner.Sort:
		input, err := Build(node.Input)
		if err != nil {
			return nil, err
		}
		return NewOrder(input, node.Terms)
	case *planner.Limit:
		input, err := Build(node.Input)
		if err != nil {
			return nil, err
		}
		return NewLimit(input, node.Max, node.Offset), nil
	default:
		return nil, fmt.Errorf("nodo de plan no soportado")
	}
}
