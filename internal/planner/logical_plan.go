package planner

import (
	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/storage"
)

// Node es un nodo del plan logico. El plan describe que operaciones se aplican
// y en que orden, pero no como se ejecutan: la resolucion de nombres de columna
// y la eleccion de operadores fisicos son responsabilidad del executor.
type Node interface {
	node()
}

// Scan lee todas las filas de una tabla ya resuelta en el catalogo. Si Alias no
// esta vacio, las columnas se publican como alias.columna; el planner lo activa
// cuando la consulta tiene algun JOIN, para que cada columna quede calificada
// exactamente una vez por muchos joins que haya.
type Scan struct {
	Table *storage.Table
	Alias string
}

// Join combina dos entradas concatenando sus esquemas.
type Join struct {
	Left      Node
	Right     Node
	Condition parser.Expression
}

// Filter conserva las filas que cumplen una condicion. Lo usan tanto WHERE
// como HAVING; la diferencia es su posicion en el plan.
type Filter struct {
	Input     Node
	Condition parser.Expression
}

// Aggregate agrupa las filas por las expresiones de GroupBy y evalua las
// llamadas de Calls. Sus columnas de salida son, en orden, las expresiones de
// agrupacion y despues las llamadas, nombradas con parser.Format.
type Aggregate struct {
	Input   Node
	GroupBy []parser.Expression
	Calls   []parser.FunctionCall
}

// ProjectItem es una columna de salida: una expresion y el nombre con el que se
// publica, o el comodin que copia todas las columnas de la entrada.
type ProjectItem struct {
	Star       bool
	Expression parser.Expression
	Name       string
}

// Project calcula las columnas de salida en el orden del SELECT.
type Project struct {
	Input Node
	Items []ProjectItem
}

// Sort ordena las filas evaluando las expresiones de ORDER BY. Va por debajo de
// Project, de modo que puede ordenar por columnas que no se seleccionan.
type Sort struct {
	Input Node
	Terms []parser.SortTerm
}

// Limit corta el resultado. Max negativo significa sin limite, lo que permite
// usar OFFSET sin LIMIT.
type Limit struct {
	Input  Node
	Max    int
	Offset int
}

func (*Scan) node()      {}
func (*Join) node()      {}
func (*Filter) node()    {}
func (*Aggregate) node() {}
func (*Project) node()   {}
func (*Sort) node()      {}
func (*Limit) node()     {}
