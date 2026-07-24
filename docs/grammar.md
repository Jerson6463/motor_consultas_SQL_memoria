# Gramatica soportada

```ebnf
query          = "SELECT" , select_list ,
                 "FROM" , from_item ,
                 [ "WHERE" , expression ] ,
                 [ group_by ] ,
                 [ "HAVING" , expression ] ,
                 [ order_by ] ,
                 [ limit ] , [ offset ] , EOF ;

select_list    = select_item , { "," , select_item } ;
select_item    = "*" | expression , [ "AS" , identifier ] ;

from_item      = identifier , { inner_join } ;
inner_join     = "INNER" , "JOIN" , identifier , "ON" , expression ;

group_by       = "GROUP" , "BY" , expression , { "," , expression } ;
order_by       = "ORDER" , "BY" , order_term , { "," , order_term } ;
order_term     = expression , [ "ASC" | "DESC" ] ;
limit          = "LIMIT" , integer ;
offset         = "OFFSET" , integer ;

(* Precedencia, de menor a mayor *)
expression     = and_expression , { "OR" , and_expression } ;
and_expression = comparison , { "AND" , comparison } ;
comparison     = additive , [ comparison_operator , additive ] ;
additive       = multiplicative , { ( "+" | "-" ) , multiplicative } ;
multiplicative = unary , { ( "*" | "/" ) , unary } ;
unary          = ( "+" | "-" ) , unary | primary ;
primary        = "(" , expression , ")"
               | function_call
               | identifier
               | number | string | boolean | "NULL" ;

function_call  = identifier , "(" , ( "*" | expression , { "," , expression } | ) , ")" ;
comparison_operator = "=" | "<>" | "<" | ">" | "<=" | ">=" ;
```

## Notas

- Las palabras clave no distinguen mayusculas de minusculas. Los textos se
  escriben entre comillas simples y una comilla simple interna se escapa
  duplicandola, por ejemplo: `'O''Brien'`.
- **El asterisco tiene tres significados** que distingue el contexto: comodin al
  inicio de un elemento de la lista de seleccion (cuando le sigue una coma o
  `FROM`), argumento de `COUNT(*)`, y operador de multiplicacion en cualquier
  otra posicion.
- **El parser no conoce los nombres de las funciones.** Cualquier identificador
  seguido de `(` se analiza como llamada; que la funcion exista lo decide el
  registro de `internal/functions`, y el planner lo comprueba.
- El signo unario (`-saldo`, `-100`, `+x`) liga mas fuerte que la
  multiplicacion, de modo que `-a * b` es `(-a) * b`, y se puede encadenar
  (`- -a`). Exige un operando numerico y propaga `NULL`.
- Los alias de tabla (`FROM empleados e`) todavia no se analizan.

## Semantica de la lista de seleccion

Las columnas de salida siguen el orden del `SELECT`, tambien en consultas
agregadas. Una columna sin `AS` toma como nombre la forma canonica de su
expresion: `SUM(salario)`, `salario * 12`, `a + (b * c)`.

Cuando la consulta agrupa, toda columna del `SELECT`, del `HAVING` y del
`ORDER BY` debe ser una clave de `GROUP BY` o el argumento de una funcion de
agregacion; en caso contrario la consulta se rechaza.

`ORDER BY` se evalua antes de la proyeccion, asi que puede ordenar por columnas
que no aparecen en el `SELECT`, y ademas admite citar un alias definido en el.

## Estrategias de JOIN

- `NestedLoopJoin`: implementacion de referencia que compara cada par de filas.
- `HashJoin`: estrategia activa para condiciones de igualdad; indexa la tabla
  derecha por su clave de union.

Los `INNER JOIN` encadenados se pliegan hacia la izquierda, de modo que
`a JOIN b JOIN c` se planifica como `(a JOIN b) JOIN c`. Cuando hay algun join,
cada tabla califica sus columnas como `tabla.columna` una sola vez.

## Ejemplos

```bash
go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT nombre, salario * 12 AS anual FROM empleados ORDER BY anual DESC"

go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT activo, COUNT(*), AVG(salario) FROM empleados GROUP BY activo HAVING COUNT(*) > 1"

go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT nombre FROM empleados ORDER BY salario DESC LIMIT 2 OFFSET 1"

go run ./cmd/sqlmem consultar empleados=data/empleados.csv areas=data/areas.csv -- "SELECT empleados.nombre, areas.nombre FROM empleados INNER JOIN areas ON empleados.id = areas.id"
```
