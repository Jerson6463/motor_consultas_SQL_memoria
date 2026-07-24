# Motor de consultas SQL en memoria

Proyecto para el Taller de Programacion en Go. Implementa un subconjunto de SQL que consulta tablas cargadas desde archivos CSV y mantenidas en memoria.

## Estado

Hitos 1 a 5 completados. El motor admite:

```sql
SELECT <expresion> [AS alias], ...
  FROM tabla [INNER JOIN tabla ON condicion]...
 [WHERE condicion]
 [GROUP BY expresion, ...]
 [HAVING condicion]
 [ORDER BY expresion [ASC|DESC], ...]
 [LIMIT n] [OFFSET n]
```

Los elementos del `SELECT` son expresiones: admiten aritmetica (`+ - * /`),
alias con `AS` y funciones de agregacion (`COUNT`, `SUM`, `AVG`, `MIN`, `MAX`)
mezcladas en cualquier orden. La gramatica completa esta en
[docs/grammar.md](docs/grammar.md).

## Estructura

Cada paquete interno cubre una etapa del recorrido de una consulta:

```text
cmd/sqlmem/        Punto de entrada del CLI.
internal/lexer/    SQL -> tokens.
internal/parser/   Tokens -> AST.
internal/storage/  Datos en memoria: tipos, valores, filas, tablas y carga de CSV.
internal/catalog/  Registro de tablas por nombre.
internal/functions/ Registro de funciones de agregacion.
internal/planner/  AST -> plan logico.
internal/executor/ Plan logico -> operadores (modelo Volcano).
internal/engine/   Fachada que encadena las etapas.
internal/cli/      Subcomandos y formato de salida.
tests/             Pruebas de integracion de SQL a resultado.
data/              Archivos CSV de ejemplo.
docs/              Arquitectura, gramatica y decisiones de diseno.
```

El detalle del flujo y del grafo de dependencias esta en
[docs/architecture.md](docs/architecture.md).

## Requisitos

- Go 1.24 o superior.

## Comandos de desarrollo

Antes de entregar el proyecto, ejecutar los siguientes comandos desde la carpeta raiz del repositorio.

### 1. Formatear todos los archivos Go

En PowerShell:

```powershell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
```

Este comando busca todos los archivos con extension `.go` y aplica el formato estandar de Go. Debe ejecutarse primero porque puede corregir automaticamente sangrias, espacios y saltos de linea.

> No usar `gofmt -w .`: `gofmt` recibe archivos, no directorios.

### 2. Compilar todos los paquetes

```bash
go build ./...
```

Compila el ejecutable y todos los paquetes internos. Si aparece un error, no se debe entregar hasta corregirlo.

### 3. Ejecutar las pruebas

```bash
go test ./...
```

Ejecuta las pruebas unitarias de cada paquete y las de integracion de `tests/`, que recorren el flujo completo de SQL a resultado. Cada paquete debe terminar con `ok`.

### 4. Revisar problemas comunes

```bash
go vet ./...
```

Busca usos sospechosos del lenguaje que pueden compilar, pero provocar errores de comportamiento. El comando debe terminar sin mensajes.

### Resultado esperado

Los cuatro pasos deben finalizar sin errores. Si se corrigio codigo despues de ejecutar las pruebas, repetir desde el paso 1.

## Ejecucion actual

```bash
go run ./cmd/sqlmem cargar empleados data/empleados.csv
```

Salida esperada:

```text
Tabla "empleados" cargada: 3 filas
- id: entero
- nombre: texto
- edad: entero
- salario: decimal
- activo: booleano
```

Consultar los datos cargados desde un CSV:

```bash
go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT nombre, salario FROM empleados WHERE activo = true AND edad >= 25"
```

Ordenar y limitar resultados:

```bash
go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT nombre, salario FROM empleados ORDER BY salario DESC LIMIT 2"
```

Agrupar resultados y usar agregados:

```bash
go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT activo, COUNT(*), AVG(salario) FROM empleados GROUP BY activo ORDER BY activo"
```

Expresiones y alias en el `SELECT`:

```bash
go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT nombre, salario * 12 AS anual FROM empleados ORDER BY anual DESC"
```

Filtrar grupos con `HAVING` y paginar con `OFFSET`:

```bash
go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT activo, COUNT(*) FROM empleados GROUP BY activo HAVING COUNT(*) > 1"

go run ./cmd/sqlmem consultar empleados data/empleados.csv "SELECT nombre FROM empleados ORDER BY salario DESC LIMIT 2 OFFSET 1"
```

## Alcance previsto

1. Carga de CSV como tablas en memoria con catalogo, esquemas y tipos.
2. Lexer, parser y AST para `SELECT ... FROM ... WHERE ...`.
3. Operadores `Scan`, `Filter` y `Project` mediante el modelo Volcano. Completado.
4. `ORDER BY`, `LIMIT`, `GROUP BY` y agregados. Completado.
5. `INNER JOIN` con nested-loop y hash join. Completado.
6. Expresiones y alias en el `SELECT`, `HAVING`, `OFFSET` y varios `JOIN`. Completado.

## Consultas con varias tablas

```bash
go run ./cmd/sqlmem consultar empleados=data/empleados.csv areas=data/areas.csv -- "SELECT empleados.nombre, areas.nombre FROM empleados INNER JOIN areas ON empleados.id = areas.id ORDER BY empleados.nombre"
```
