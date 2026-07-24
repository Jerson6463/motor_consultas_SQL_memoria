// Package engine es la fachada del motor: encadena las etapas de una consulta
// sin contener logica propia de analisis, planificacion ni ejecucion.
package engine

import (
	"io"

	"motor-consultas-sql/internal/catalog"
	"motor-consultas-sql/internal/executor"
	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/planner"
	"motor-consultas-sql/internal/storage"
)

// Engine agrupa las tablas disponibles y ejecuta consultas contra ellas.
type Engine struct {
	catalog *catalog.Catalog
}

// New crea un motor sin tablas cargadas.
func New() *Engine {
	return &Engine{catalog: catalog.New()}
}

// LoadCSV carga una tabla desde un CSV y la registra en el catalogo.
func (e *Engine) LoadCSV(name string, input io.Reader) (*storage.Table, error) {
	table, err := storage.LoadCSV(name, input)
	if err != nil {
		return nil, err
	}
	if err := e.catalog.Add(table); err != nil {
		return nil, err
	}
	return table, nil
}

// LoadCSVFile carga una tabla desde un archivo CSV y la registra en el catalogo.
func (e *Engine) LoadCSVFile(name, path string) (*storage.Table, error) {
	table, err := storage.LoadCSVFile(name, path)
	if err != nil {
		return nil, err
	}
	if err := e.catalog.Add(table); err != nil {
		return nil, err
	}
	return table, nil
}

// Query recorre las etapas del motor: SQL -> AST -> plan logico -> operadores.
// Las filas no se calculan aqui; se obtienen al recorrer el resultado.
func (e *Engine) Query(sql string) (*Result, error) {
	statement, err := parser.Parse(sql)
	if err != nil {
		return nil, err
	}
	plan, err := planner.Plan(e.catalog, statement)
	if err != nil {
		return nil, err
	}
	operator, err := executor.Build(plan)
	if err != nil {
		return nil, err
	}
	return &Result{operator: operator}, nil
}

// Result recorre las filas de una consulta de forma perezosa.
type Result struct {
	operator executor.Operator
}

// Columns devuelve el esquema del resultado.
func (r *Result) Columns() []storage.Column { return r.operator.Columns() }

// Next entrega la siguiente fila y devuelve io.EOF cuando no quedan mas.
func (r *Result) Next() (storage.Row, error) { return r.operator.Next() }

// Close libera los recursos de la consulta.
func (r *Result) Close() error { return r.operator.Close() }
