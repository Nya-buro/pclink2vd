// Package pclinkcsv parses CSV log files exported by PCLink (LINK G4X).
//
// Exported file layout (as produced by PCLink Logging > Export > CSV):
//
//	line 1: title row            e.g. "名前","PCデータログ - 2026-09-23 22;04;31"
//	line 2: header row           parameter names (hundreds of columns)
//	line 3: units row            e.g. "s","%","kPa","RPM","kph",...
//	line 4+: data rows           quoted numeric values, time in seconds
package pclinkcsv

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// Table holds one parsed PCLink CSV export.
type Table struct {
	Title   []string
	Headers []string // trimmed parameter names
	Units   []string // trimmed unit strings (same length as Headers)
	Rows    [][]string
}

// Parse reads and decodes a PCLink CSV export.
func Parse(path string) (*Table, error) {
	b, err := readFile(path)
	if err != nil {
		return nil, err
	}
	text, err := decodeText(b)
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	var recs [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV解析エラー: %w", err)
		}
		if allEmpty(rec) {
			continue
		}
		recs = append(recs, rec)
	}
	if len(recs) < 3 {
		return nil, fmt.Errorf("想定より行数が少ないファイルです(%d行)。PCLinkからエクスポートしたCSVですか?", len(recs))
	}

	t := &Table{}
	start := 0
	// The title row has very few fields compared with the header row.
	if len(recs[0]) <= 2 && len(recs[1]) > len(recs[0]) {
		t.Title = trimAll(recs[0])
		start = 1
	}
	t.Headers = trimAll(recs[start])
	if start+1 < len(recs) && looksLikeUnits(recs[start+1]) {
		t.Units = padTo(trimAll(recs[start+1]), len(t.Headers))
		start += 2
	} else {
		t.Units = make([]string, len(t.Headers))
	}
	for _, rec := range recs[start:] {
		t.Rows = append(t.Rows, padTo(rec, len(t.Headers)))
	}
	return t, nil
}

// Unit returns the unit string for column i ("" when unknown).
func (t *Table) Unit(i int) string {
	if i >= 0 && i < len(t.Units) {
		return t.Units[i]
	}
	return ""
}

// Header returns the header for column i ("" when out of range).
func (t *Table) Header(i int) string {
	if i >= 0 && i < len(t.Headers) {
		return t.Headers[i]
	}
	return ""
}

// IndexOf returns the first column whose header equals name (case-insensitive).
func (t *Table) IndexOf(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	for i, h := range t.Headers {
		if strings.ToLower(h) == name {
			return i
		}
	}
	return -1
}

// ColumnLabel renders a column for UI display: "12: MAP [kPa]".
func (t *Table) ColumnLabel(i int) string {
	u := t.Unit(i)
	if u == "" {
		return fmt.Sprintf("%d: %s", i, t.Header(i))
	}
	return fmt.Sprintf("%d: %s [%s]", i, t.Header(i), u)
}

// NumericRatio reports what fraction of column i's values parse as floats
// (sampling at most max rows).
func (t *Table) NumericRatio(i int, max int) float64 {
	if i < 0 || i >= len(t.Headers) || len(t.Rows) == 0 {
		return 0
	}
	step := 1
	if len(t.Rows) > max && max > 0 {
		step = len(t.Rows) / max
	}
	n, ok := 0, 0
	for r := 0; r < len(t.Rows); r += step {
		v := strings.TrimSpace(t.Rows[r][i])
		if v == "" {
			continue
		}
		n++
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			ok++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(ok) / float64(n)
}

func readFile(path string) ([]byte, error) {
	// whole file: PCLink CSVs are at most a few dozen MB
	return readAllBytes(path)
}

// decodeText decodes UTF-8 (BOM optional); falls back to Shift_JIS (cp932).
func decodeText(b []byte) (string, error) {
	if s, err := decodeUTF8(b); err == nil {
		return s, nil
	}
	if s, err := decodeCP932(b); err == nil {
		return s, nil
	}
	return "", fmt.Errorf("テキストをデコードできませんでした(UTF-8/Shift_JIS)")
}

func looksLikeUnits(rec []string) bool {
	n, short := 0, 0
	for _, c := range rec {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		n++
		if len([]rune(c)) <= 8 && !containsDigit(c) {
			short++
		}
	}
	if n == 0 {
		return false
	}
	return float64(short)/float64(n) > 0.7
}

func containsDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func allEmpty(rec []string) bool {
	for _, c := range rec {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func trimAll(rec []string) []string {
	out := make([]string, len(rec))
	for i, c := range rec {
		out[i] = strings.TrimSpace(c)
	}
	return out
}

func padTo(rec []string, n int) []string {
	if len(rec) >= n {
		return rec[:n]
	}
	out := make([]string, n)
	copy(out, rec)
	for i := len(rec); i < n; i++ {
		out[i] = ""
	}
	return out
}
