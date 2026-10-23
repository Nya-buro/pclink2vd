package convert

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// WriteCSV writes the converted table as a plain CSV (CRLF, minimal quoting).
func WriteCSV(path string, res *Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	writeRow(w, res.Headers)
	for _, r := range res.Rows {
		writeRow(w, r)
	}
	return w.Flush()
}

func writeRow(w *bufio.Writer, cells []string) {
	for i, c := range cells {
		if i > 0 {
			w.WriteByte(',')
		}
		w.WriteString(escapeCSV(c))
	}
	w.WriteString("\r\n")
}

func escapeCSV(s string) string {
	if strings.ContainsAny(s, ",\"\r\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// Describe returns a short human summary of conversion stats (Japanese).
func (s Stats) Describe() string {
	mode := map[string]string{"manual": "手動範囲", "wot": "WOT自動", "all": "全ログ"}[s.TrimMode]
	var b strings.Builder
	fmt.Fprintf(&b, "変換完了: %d / %d 行 [%s]", s.RowsOut, s.RowsIn, mode)
	fmt.Fprintf(&b, " 区間 %.3f〜%.3f s (%.1f s)", s.RunStart, s.RunEnd, s.RunEnd-s.RunStart)
	if s.TrimMode == "wot" {
		fmt.Fprintf(&b, " WOT検出数: %d", s.RunCount)
	}
	if s.LambdaFromAFR {
		b.WriteString(" (λ: AFR→λ 変換)")
	}
	return b.String()
}
