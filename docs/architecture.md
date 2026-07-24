# Arquitectura

El motor esta organizado en etapas. Cada paquete cubre una etapa del recorrido
de una consulta y depende solo de las etapas anteriores, de modo que el flujo se
lee en la propia estructura de directorios.

## Flujo de una consulta

```text
SQL
  -> lexer      Lex(sql)                  -> []Token
  -> parser     Parse(sql)                -> *Query (AST)
  -> planner    Plan(catalog, ast)        -> Node (plan logico)
  -> executor   Build(plan)               -> Operator (arbol fisico)
  -> storage    Operator.Next()           -> Row
  -> resultado
```

El paquete `engine` encadena esas llamadas y `cli` presenta el resultado.

## Paquetes

| Paquete | Responsabilidad |
| --- | --- |
| `internal/lexer` | Convierte el texto SQL en tokens. Define `Token` y `TokenKind`. |
| `internal/parser` | Analisis sintactico y AST. `Parse` produce `*Query`; `Format` da la forma canonica de una expresion. |
| `internal/storage` | Representacion en memoria de los datos: `DataType`, `Value`, `Column`, `Row`, `Table`, la carga desde CSV y la comparacion de valores. |
| `internal/catalog` | Registro de tablas por nombre, sin distinguir mayusculas de minusculas. |
| `internal/functions` | Registro de funciones de agregacion y sus acumuladores. |
| `internal/planner` | Traduce el AST a un plan logico: resuelve tablas y separa la agregacion del resto del SELECT. |
| `internal/executor` | Ejecuta el plan con el modelo Volcano: un operador por nodo logico. |
| `internal/engine` | Fachada que encadena las etapas y expone `Result`. |
| `internal/cli` | Subcomandos, argumentos y formato de salida. |
| `cmd/sqlmem` | Punto de entrada; delega en `cli.Run`. |

## Dependencias

```text
lexer     -> (biblioteca estandar)
parser    -> lexer
storage   -> (biblioteca estandar)
catalog   -> storage
functions -> storage
planner   -> parser, catalog, storage, functions
executor  -> planner, parser, storage, functions
engine    -> parser, planner, executor, catalog, storage
cli       -> engine
```

El grafo es aciclico y las flechas siguen el sentido del flujo. En particular:

- El AST **no** usa tipos del lexer. `parser` define `BinaryOp` y `LiteralKind`,
  y traduce los tokens a esos valores en un unico punto.
- El `executor` **no** conoce el catalogo: el `planner` ya resolvio las tablas y
  el nodo `Scan` lleva la `*storage.Table` correspondiente.
- `functions` es un paquete propio, y no parte del executor, porque el planner
  necesita saber que nombres son agregados para separarlos del SELECT, y
  `executor` ya importa `planner`.

## Como se representa el SELECT

La lista de seleccion es una unica lista ordenada de expresiones con alias
opcional:

```go
type SelectItem struct {
    Star       bool
    Expression Expression
    Alias      string
}
```

Que sea **una sola lista** y que sus elementos sean **expresiones** es lo que
permite alias, aritmetica y agregados mezclados en cualquier orden. Las llamadas
a funcion son un nodo de expresion mas (`FunctionCall`), asi que un agregado
puede aparecer dentro de una operacion: `COUNT(*) * 2`.

## La reescritura de agregados

Con los agregados dentro de las expresiones, el planner tiene que separar lo que
calcula el operador `Aggregate` de lo que calcula el `Project` que va encima.
Lo hace sustituyendo cada llamada de agregacion, y cada expresion que coincide
con una clave de `GROUP BY`, por una referencia a la columna que produce la
agregacion (`internal/planner/rewrite.go`):

```text
SELECT zona, COUNT(*) * 2 AS doble FROM ventas GROUP BY zona

  Aggregate produce las columnas:   zona | COUNT(*)
  Project evalua, ya reescrito:     zona , COUNT(*) * 2   -> como referencias
```

Las columnas que produce `Aggregate` se nombran con `parser.Format`, la misma
funcion que da nombre a las columnas de salida sin alias. Esa forma canonica es
tambien la clave con la que se reconoce que dos expresiones son la misma.

De la reescritura sale gratis la validacion correcta de `GROUP BY`: si tras
sustituir queda un identificador suelto, es una columna que no es clave de
agrupacion ni argumento de un agregado, y la consulta se rechaza.

## Reparto entre planner y executor

El plan logico describe **que** operaciones se aplican y en que orden; el
executor decide **como** se ejecutan y resuelve los nombres de columna.

El `planner` comprueba lo que puede saber sin esquemas: que las tablas existan,
que las funciones esten registradas, que no haya agregados en el `WHERE`, que
`HAVING` acompane a una agregacion y que el `GROUP BY` cubra las columnas
citadas.

El `executor` comprueba el resto al construir cada operador: existencia y
ambiguedad de columnas (`findColumn`), validez del `WHERE` (`NewFilter`) y forma
de la condicion del join (`NewHashJoin`).

## Orden de los operadores

```text
FROM             arbol de Scan y Join
  -> [Filter]    WHERE
  -> [Aggregate] si hay GROUP BY o alguna funcion de agregacion
  -> [Filter]    HAVING
  -> [Sort]      ORDER BY
  -> Project     SELECT
  -> [Limit]     LIMIT y OFFSET
```

Dos decisiones que se leen aqui:

- `Sort` va **por debajo** de `Project`, de modo que `ORDER BY` puede usar
  columnas que no se seleccionan.
- `Project` se aplica **siempre**, tambien en consultas agregadas, de modo que
  las columnas de salida siguen el orden del `SELECT`.

La calificacion de columnas (`tabla.columna`) la hace el `Scan` cuando la
consulta tiene algun join, no el operador de join. Asi cada columna se prefija
exactamente una vez por muchos joins que haya.

## Modelo de ejecucion

Todos los operadores implementan `Operator`:

```go
type Operator interface {
    Next() (storage.Row, error) // io.EOF cuando se agotan las filas
    Columns() []storage.Column
    Close() error
}
```

Cada operador tira de las filas del inferior, de una en una. Solo materializan
filas los que no pueden evitarlo: `Order` (ordenar exige todas las filas
candidatas), `Aggregate` (agrupar) y el lado derecho de `HashJoin` (construir el
indice).

## Como se anade SQL nuevo

| Que se anade | Donde se toca |
| --- | --- |
| Funcion de agregacion | Una entrada en el registro de `internal/functions`. Nada mas. |
| Operador de comparacion | Token en `lexer`, constante `BinaryOp` y `isComparison` en `parser`, un `case` en `evaluate.go`. |
| Clausula nueva | Token en `lexer`, campo en `Query` y su analisis en `parser`, nodo en `logical_plan.go` y su sitio en `Plan`, operador en `executor` y un `case` en `Build`. |
| Tipo de dato | `storage/value.go`, la inferencia en `storage/csv.go` y `storage.Compare`. |

## Pruebas

- Pruebas unitarias junto a cada paquete.
- Pruebas de integracion en `tests/`, que recorren el flujo completo SQL a
  resultado a traves de `engine`.
