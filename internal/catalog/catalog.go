// Package catalog mantiene el registro de tablas disponibles para una consulta.
package catalog

import (
	"fmt"

	"motor-consultas-sql/internal/storage"
)

// Catalog almacena tablas por nombre.
type Catalog struct {
	tables map[string]*storage.Table
}

// New crea un catalogo vacio.
func New() *Catalog {
	return &Catalog{tables: make(map[string]*storage.Table)}
}

// Add registra una tabla. Los nombres no distinguen mayusculas de minusculas.
func (c *Catalog) Add(table *storage.Table) error {
	if table == nil {
		return fmt.Errorf("no se puede agregar una tabla nula")
	}

	key := storage.NormalizeName(table.Name)
	if key == "" {
		return fmt.Errorf("el nombre de la tabla no puede estar vacio")
	}
	if _, exists := c.tables[key]; exists {
		return fmt.Errorf("la tabla %q ya existe", table.Name)
	}

	c.tables[key] = table
	return nil
}

// Table devuelve una tabla por nombre.
func (c *Catalog) Table(name string) (*storage.Table, bool) {
	table, ok := c.tables[storage.NormalizeName(name)]
	return table, ok
}

// Tables devuelve todas las tablas registradas.
func (c *Catalog) Tables() []*storage.Table {
	tables := make([]*storage.Table, 0, len(c.tables))
	for _, table := range c.tables {
		tables = append(tables, table)
	}
	return tables
}
