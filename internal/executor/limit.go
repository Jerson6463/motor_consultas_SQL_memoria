package executor

import (
	"io"

	"motor-consultas-sql/internal/storage"
)

// Limit descarta las primeras offset filas y despues entrega como mucho max.
// Un max negativo significa sin limite, lo que permite usar OFFSET sin LIMIT.
type Limit struct {
	input   Operator
	max     int
	offset  int
	skipped int
	count   int
}

func NewLimit(input Operator, max, offset int) *Limit {
	return &Limit{input: input, max: max, offset: offset}
}

func (l *Limit) Next() (storage.Row, error) {
	for l.skipped < l.offset {
		if _, err := l.input.Next(); err != nil {
			return nil, err
		}
		l.skipped++
	}
	if l.max >= 0 && l.count >= l.max {
		return nil, io.EOF
	}
	row, err := l.input.Next()
	if err == nil {
		l.count++
	}
	return row, err
}

func (l *Limit) Columns() []storage.Column { return l.input.Columns() }
func (l *Limit) Close() error              { return l.input.Close() }
