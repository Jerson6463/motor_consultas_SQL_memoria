package functions

import (
	"strings"
	"testing"

	"motor-consultas-sql/internal/storage"
)

func entero(n int64) storage.Value {
	return storage.Value{Type: storage.Integer, Data: n}
}

func nulo(t storage.DataType) storage.Value {
	return storage.Value{Type: t, Null: true}
}

// acumular alimenta un acumulador nuevo con los valores indicados.
func acumular(t *testing.T, nombre string, argumento storage.DataType, valores ...storage.Value) storage.Value {
	t.Helper()
	spec, ok := Lookup(nombre)
	if !ok {
		t.Fatalf("la funcion %q no esta registrada", nombre)
	}
	accumulator := spec.New(argumento)
	for _, value := range valores {
		if err := accumulator.Add(value); err != nil {
			t.Fatalf("Add devolvio error: %v", err)
		}
	}
	return accumulator.Result()
}

func TestLookupNoDistingueMayusculas(t *testing.T) {
	for _, nombre := range []string{"count", "COUNT", "Count", " sum "} {
		if !IsAggregate(nombre) {
			t.Errorf("IsAggregate(%q) = false; se esperaba true", nombre)
		}
	}
	if IsAggregate("stddev") {
		t.Error("IsAggregate(\"stddev\") = true; esa funcion no esta registrada")
	}
}

func TestNamesDevuelveLasFuncionesRegistradas(t *testing.T) {
	got := strings.Join(Names(), ",")
	want := "AVG,COUNT,MAX,MIN,SUM"
	if got != want {
		t.Errorf("Names() = %q; se esperaba %q", got, want)
	}
}

func TestCountIgnoraLosNulos(t *testing.T) {
	result := acumular(t, "COUNT", storage.Integer, entero(1), nulo(storage.Integer), entero(3))
	if result.Data != int64(2) {
		t.Errorf("COUNT = %#v; se esperaba int64(2)", result.Data)
	}
}

func TestCountDeGrupoVacioEsCero(t *testing.T) {
	result := acumular(t, "COUNT", storage.Integer)
	if result.Null || result.Data != int64(0) {
		t.Errorf("COUNT = %#v; se esperaba int64(0) no nulo", result)
	}
}

func TestSumYAvgIgnoranLosNulos(t *testing.T) {
	suma := acumular(t, "SUM", storage.Integer, entero(10), nulo(storage.Integer), entero(20))
	if suma.Data != 30.0 {
		t.Errorf("SUM = %#v; se esperaba 30.0", suma.Data)
	}

	promedio := acumular(t, "AVG", storage.Integer, entero(10), nulo(storage.Integer), entero(20))
	if promedio.Data != 15.0 {
		t.Errorf("AVG = %#v; se esperaba 15.0, el nulo no entra en el promedio", promedio.Data)
	}
}

func TestSumDeGrupoSinValoresEsNulo(t *testing.T) {
	result := acumular(t, "SUM", storage.Integer, nulo(storage.Integer))
	if !result.Null {
		t.Errorf("SUM = %#v; se esperaba NULL", result)
	}
}

func TestSumRechazaColumnasNoNumericas(t *testing.T) {
	spec, _ := Lookup("SUM")
	accumulator := spec.New(storage.Text)
	err := accumulator.Add(storage.Value{Type: storage.Text, Data: "Ana"})
	if err == nil {
		t.Fatal("Add no devolvio error para un texto")
	}
	if !strings.Contains(err.Error(), "SUM requiere una columna numerica") {
		t.Errorf("error = %v; se esperaba el aviso de columna numerica", err)
	}
}

func TestMinYMaxConservanElValorOriginal(t *testing.T) {
	minimo := acumular(t, "MIN", storage.Integer, entero(20), entero(10), entero(30))
	if minimo.Data != int64(10) {
		t.Errorf("MIN = %#v; se esperaba int64(10)", minimo.Data)
	}
	if minimo.Type != storage.Integer {
		t.Errorf("MIN.Type = %s; se esperaba entero", minimo.Type)
	}

	maximo := acumular(t, "MAX", storage.Integer, entero(20), entero(10), entero(30))
	if maximo.Data != int64(30) {
		t.Errorf("MAX = %#v; se esperaba int64(30)", maximo.Data)
	}
}

func TestMinSobreTextos(t *testing.T) {
	texto := func(s string) storage.Value { return storage.Value{Type: storage.Text, Data: s} }
	result := acumular(t, "MIN", storage.Text, texto("Carla"), texto("Ana"), texto("Beto"))
	if result.Data != "Ana" {
		t.Errorf("MIN = %#v; se esperaba Ana", result.Data)
	}
}

func TestMinPropagaErroresDeComparacion(t *testing.T) {
	spec, _ := Lookup("MIN")
	accumulator := spec.New(storage.Integer)
	if err := accumulator.Add(entero(1)); err != nil {
		t.Fatalf("Add devolvio error: %v", err)
	}
	if err := accumulator.Add(storage.Value{Type: storage.Text, Data: "Ana"}); err == nil {
		t.Fatal("Add no devolvio error al comparar un entero con un texto")
	}
}

// TestResultTypeDeMinYMax fija que el tipo declarado coincide con el valor
// devuelto, a diferencia del comportamiento anterior que declaraba decimal.
func TestResultTypeDeMinYMax(t *testing.T) {
	for _, nombre := range []string{"MIN", "MAX"} {
		spec, _ := Lookup(nombre)
		if got := spec.ResultType(storage.Integer); got != storage.Integer {
			t.Errorf("%s.ResultType(entero) = %s; se esperaba entero", nombre, got)
		}
		if got := spec.ResultType(storage.Text); got != storage.Text {
			t.Errorf("%s.ResultType(texto) = %s; se esperaba texto", nombre, got)
		}
	}
	count, _ := Lookup("COUNT")
	if got := count.ResultType(storage.Text); got != storage.Integer {
		t.Errorf("COUNT.ResultType = %s; se esperaba entero", got)
	}
}

func TestSoloCountAdmiteAsterisco(t *testing.T) {
	count, _ := Lookup("COUNT")
	if !count.AcceptsStar {
		t.Error("COUNT deberia admitir COUNT(*)")
	}
	for _, nombre := range []string{"SUM", "AVG", "MIN", "MAX"} {
		spec, _ := Lookup(nombre)
		if spec.AcceptsStar {
			t.Errorf("%s no deberia admitir la forma con asterisco", nombre)
		}
	}
}
