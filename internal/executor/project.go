package executor

import (
	"motor-consultas-sql/internal/parser"
	"motor-consultas-sql/internal/planner"
	"motor-consultas-sql/internal/storage"
)

// projection es una columna de salida. Un indice no negativo copia la columna
// de entrada directamente; en caso contrario se evalua la expresion.
type projection struct {
	index      int
	expression parser.Expression
}

// Project calcula las columnas de salida en el orden del SELECT.
type Project struct {
	input       Operator
	projections []projection
	columns     []storage.Column
}

// NewProject crea una proyeccion. Los elementos con comodin se expanden a una
// copia directa de cada columna de la entrada.
func NewProject(input Operator, items []planner.ProjectItem) (*Project, error) {
	project := &Project{input: input}
	inputColumns := input.Columns()

	for _, item := range items {
		if item.Star {
			for index, column := range inputColumns {
				project.projections = append(project.projections, projection{index: index})
				project.columns = append(project.columns, column)
			}
			continue
		}

		column, err := projectedColumn(inputColumns, item)
		if err != nil {
			return nil, err
		}
		project.projections = append(project.projections, projection{index: -1, expression: item.Expression})
		project.columns = append(project.columns, column)
	}
	return project, nil
}

// projectedColumn calcula el nombre y el tipo de una columna de salida. Una
// referencia directa conserva el tipo de la columna original; una expresion
// calculada toma el tipo que deduce el evaluador.
func projectedColumn(inputColumns []storage.Column, item planner.ProjectItem) (storage.Column, error) {
	if identifier, ok := item.Expression.(parser.Identifier); ok {
		_, column, err := findColumn(inputColumns, identifier.Name)
		if err != nil {
			return storage.Column{}, err
		}
		return storage.Column{Name: item.Name, Type: column.Type}, nil
	}

	dataType, err := expressionType(item.Expression, inputColumns)
	if err != nil {
		return storage.Column{}, err
	}
	return storage.Column{Name: item.Name, Type: dataType}, nil
}

// Next entrega una fila con las columnas proyectadas.
func (p *Project) Next() (storage.Row, error) {
	row, err := p.input.Next()
	if err != nil {
		return nil, err
	}

	projected := make(storage.Row, len(p.projections))
	for index, projection := range p.projections {
		if projection.index >= 0 {
			projected[index] = row[projection.index]
			continue
		}
		value, err := evaluate(projection.expression, row, p.input.Columns())
		if err != nil {
			return nil, err
		}
		projected[index] = value
	}
	return projected, nil
}

// Columns devuelve el esquema despues de aplicar SELECT.
func (p *Project) Columns() []storage.Column {
	return p.columns
}

// Close cierra el operador de entrada.
func (p *Project) Close() error {
	return p.input.Close()
}
