package catalog

import (
	"testing"

	"motor-consultas-sql/internal/storage"
)

func TestCatalogAddAndFindTable(t *testing.T) {
	catalog := New()
	table := &storage.Table{Name: "Empleados"}
	if err := catalog.Add(table); err != nil {
		t.Fatalf("Add devolvio error: %v", err)
	}

	got, ok := catalog.Table("empleados")
	if !ok || got != table {
		t.Fatal("no se encontro la tabla agregada")
	}

	if err := catalog.Add(&storage.Table{Name: "EMPLEADOS"}); err == nil {
		t.Fatal("se permitio una tabla duplicada")
	}
}
