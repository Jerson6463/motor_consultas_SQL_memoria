// Command sqlmem ejecuta consultas SQL sobre archivos CSV cargados en memoria.
package main

import (
	"os"

	"motor-consultas-sql/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
