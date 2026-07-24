// Package planner traduce el AST de una consulta a un plan logico de operaciones.
package planner

import (
	"fmt"

	"motor-consultas-sql/internal/catalog"
	"motor-consultas-sql/internal/parser"
)

// Plan resuelve las tablas de la consulta contra el catalogo y devuelve el plan
// logico. El orden de los nodos codifica la semantica SQL:
//
//	FROM -> WHERE -> agregacion -> HAVING -> ORDER BY -> SELECT -> LIMIT
//
// El Project va por encima del Sort para que ORDER BY pueda usar columnas que no
// se seleccionan, y se aplica siempre, tambien en consultas agregadas, para que
// las columnas de salida sigan el orden del SELECT.
func Plan(cat *catalog.Catalog, statement *parser.Query) (Node, error) {
	node, err := planFrom(cat, statement.From, hasJoin(statement.From))
	if err != nil {
		return nil, err
	}

	analysis, err := analyze(statement)
	if err != nil {
		return nil, err
	}

	if statement.Where != nil {
		node = &Filter{Input: node, Condition: statement.Where}
	}
	if analysis.aggregated {
		node = &Aggregate{Input: node, GroupBy: statement.GroupBy, Calls: analysis.calls}
		if analysis.having != nil {
			node = &Filter{Input: node, Condition: analysis.having}
		}
	}
	if len(analysis.order) > 0 {
		node = &Sort{Input: node, Terms: analysis.order}
	}
	node = &Project{Input: node, Items: analysis.items}

	if statement.Limit != nil || statement.Offset != nil {
		limit := &Limit{Input: node, Max: -1}
		if statement.Limit != nil {
			limit.Max = *statement.Limit
		}
		if statement.Offset != nil {
			limit.Offset = *statement.Offset
		}
		node = limit
	}
	return node, nil
}

// planFrom recorre el arbol del FROM. Cuando la consulta tiene algun JOIN, cada
// tabla califica sus columnas con su nombre para poder distinguir las homonimas.
func planFrom(cat *catalog.Catalog, from parser.FromItem, qualify bool) (Node, error) {
	switch from := from.(type) {
	case *parser.TableRef:
		table, ok := cat.Table(from.Name)
		if !ok {
			return nil, fmt.Errorf("la tabla %q no existe", from.Name)
		}
		scan := &Scan{Table: table}
		if qualify {
			scan.Alias = table.Name
		}
		return scan, nil
	case *parser.JoinRef:
		left, err := planFrom(cat, from.Left, qualify)
		if err != nil {
			return nil, err
		}
		right, err := planFrom(cat, from.Right, qualify)
		if err != nil {
			return nil, err
		}
		return &Join{Left: left, Right: right, Condition: from.On}, nil
	default:
		return nil, fmt.Errorf("clausula FROM no soportada")
	}
}

func hasJoin(from parser.FromItem) bool {
	_, ok := from.(*parser.JoinRef)
	return ok
}
