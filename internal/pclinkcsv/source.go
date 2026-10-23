package pclinkcsv

import "strconv"

// RowCount implements convert.Source.
func (t *Table) RowCount() int { return len(t.Rows) }

// Cell implements convert.Source.
func (t *Table) Cell(r, c int) string {
	if r >= 0 && r < len(t.Rows) && c >= 0 && c < len(t.Rows[r]) {
		return t.Rows[r][c]
	}
	return ""
}

// Num implements convert.Source.
func (t *Table) Num(r, c int) (float64, bool) {
	s := t.Cell(r, c)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
