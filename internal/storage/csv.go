package storage

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// LoadCSV carga un CSV con cabecera, infiere el tipo de cada columna y crea una tabla.
func LoadCSV(name string, input io.Reader) (*Table, error) {
	reader := csv.NewReader(input)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("leer CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("el CSV no tiene cabecera")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("el nombre de la tabla no puede estar vacio")
	}

	headers := records[0]
	if len(headers) == 0 {
		return nil, fmt.Errorf("el CSV no tiene columnas")
	}
	if err := validateHeaders(headers); err != nil {
		return nil, err
	}

	types := inferTypes(headers, records[1:])
	table := &Table{Name: name, Columns: make([]Column, len(headers)), Rows: make([]Row, 0, len(records)-1)}
	for index, header := range headers {
		table.Columns[index] = Column{Name: strings.TrimSpace(header), Type: types[index]}
	}

	for rowIndex, record := range records[1:] {
		if len(record) != len(headers) {
			return nil, fmt.Errorf("fila %d: se esperaban %d columnas y se recibieron %d", rowIndex+2, len(headers), len(record))
		}

		row := make(Row, len(record))
		for columnIndex, raw := range record {
			value, err := parseValue(raw, types[columnIndex])
			if err != nil {
				return nil, fmt.Errorf("fila %d, columna %q: %w", rowIndex+2, headers[columnIndex], err)
			}
			row[columnIndex] = value
		}
		table.Rows = append(table.Rows, row)
	}

	return table, nil
}

// LoadCSVFile abre un archivo CSV y lo carga como tabla.
func LoadCSVFile(name, path string) (*Table, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrir %q: %w", path, err)
	}
	defer file.Close()

	return LoadCSV(name, file)
}

func validateHeaders(headers []string) error {
	seen := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		name := strings.TrimSpace(header)
		if name == "" {
			return fmt.Errorf("hay una columna sin nombre")
		}
		key := NormalizeName(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("la columna %q esta repetida", name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func inferTypes(headers []string, rows [][]string) []DataType {
	types := make([]DataType, len(headers))
	for columnIndex := range headers {
		allBoolean := true
		allInteger := true
		allDecimal := true
		hasValue := false
		for _, row := range rows {
			if columnIndex >= len(row) || strings.TrimSpace(row[columnIndex]) == "" {
				continue
			}

			hasValue = true
			raw := strings.TrimSpace(row[columnIndex])
			if _, err := strconv.ParseBool(strings.ToLower(raw)); err != nil {
				allBoolean = false
			}
			if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
				allInteger = false
			}
			if _, err := strconv.ParseFloat(raw, 64); err != nil {
				allDecimal = false
			}
		}

		switch {
		case !hasValue:
			types[columnIndex] = Text
		case allBoolean:
			types[columnIndex] = Boolean
		case allInteger:
			types[columnIndex] = Integer
		case allDecimal:
			types[columnIndex] = Decimal
		default:
			types[columnIndex] = Text
		}
	}
	return types
}

func parseValue(raw string, dataType DataType) (Value, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Value{Type: dataType, Null: true}, nil
	}

	value := Value{Type: dataType}
	var err error
	switch dataType {
	case Boolean:
		value.Data, err = strconv.ParseBool(strings.ToLower(raw))
	case Integer:
		value.Data, err = strconv.ParseInt(raw, 10, 64)
	case Decimal:
		value.Data, err = strconv.ParseFloat(raw, 64)
	default:
		value.Data = raw
	}
	if err != nil {
		return Value{}, fmt.Errorf("%q no es un %s valido", raw, dataType)
	}
	return value, nil
}
