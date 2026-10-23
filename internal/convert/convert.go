// Package convert maps parsed PCLink columns to Virtual Dyno log columns,
// applies unit conversion and WOT segment trimming, then writes the CSV.
package convert

import (
	"fmt"
	"strings"
)

// Source abstracts the parsed PCLink table (implemented by *pclinkcsv.Table).
type Source interface {
	RowCount() int
	Cell(r, c int) string
	Num(r, c int) (float64, bool)
	Unit(c int) string
	SniffLambdaIsAFR(c int) bool
}

// Mapping selects source columns (indices into the PCLink table). -1 = unmapped.
type Mapping struct {
	Time   int   `json:"time"`
	RPM    int   `json:"rpm"`
	Speed  int   `json:"speed"`
	TPS    int   `json:"tps"`
	Lambda int   `json:"lambda"`
	Boost  int   `json:"boost"`
	Gear   int   `json:"gear"`
	Extra  []int `json:"extra"`
}

// NewMapping returns an all-unmapped mapping.
func NewMapping() Mapping {
	return Mapping{Time: -1, RPM: -1, Speed: -1, TPS: -1, Lambda: -1, Boost: -1, Gear: -1}
}

// Options controls unit conversion and trimming.
type Options struct {
	// SpeedOut: "mph" (Virtual Dyno default) or "kph".
	SpeedOut string `json:"speedOut"`
	// LambdaOut: "auto" | "lambda" | "afr".
	LambdaOut string `json:"lambdaOut"`
	// StoichAFR for lambda<->AFR conversion, typically 14.7.
	StoichAFR float64 `json:"stoichAfr"`
	// BoostOut: "psi" (Virtual Dyno) or "kpa".
	BoostOut string `json:"boostOut"`
	// WOTThreshold: TPS %% considered "wide open throttle"; 0 disables auto trim.
	WOTThreshold float64 `json:"wotThreshold"`
	// Manual time window in seconds; HasStart/HasEnd enable either bound.
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	HasStart bool    `json:"hasStart"`
	HasEnd   bool    `json:"hasEnd"`
}

// DefaultOptions returns Virtual Dyno friendly defaults.
func DefaultOptions() Options {
	return Options{
		SpeedOut: "mph", LambdaOut: "auto", StoichAFR: 14.7, BoostOut: "psi",
		WOTThreshold: 90,
	}
}

// HeaderNames are the output CSV column names.
type HeaderNames struct {
	Time, Speed, RPM, TPS, Lambda, Boost, Gear string
}

// DefaultHeaders returns names recognized by Virtual Dyno.
func DefaultHeaders(o Options) HeaderNames {
	speed := "MPH"
	if o.SpeedOut == "kph" {
		speed = "km/h"
	}
	lambda := "Lambda"
	if o.LambdaOut == "afr" {
		lambda = "AFR"
	}
	return HeaderNames{Time: "Time", Speed: speed, RPM: "RPM", TPS: "TPS", Lambda: lambda, Boost: "Boost", Gear: "Gear"}
}

// Stats summarizes a conversion run.
type Stats struct {
	RowsIn      int
	RowsOut     int
	RunCount    int    // WOT runs found (0 when auto trim disabled)
	RunStart    float64 // chosen segment bounds (s)
	RunEnd      float64
	TrimMode    string // "manual" | "wot" | "all"
	LambdaFromAFR bool // source lambda values were AFR converted to lambda
}

// Result is the converted table.
type Result struct {
	Headers []string
	Rows    [][]string
	Stats   Stats
}

type rec struct {
	t, rpm, spd, tps, lam, bst, gear float64
	hasTPS, hasLAM, hasBST, hasGear  bool
	extra                            []string
}

// Run performs the conversion.
func Run(src Source, m Mapping, o Options, extraHeaders []string) (*Result, error) {
	if m.Time < 0 || m.RPM < 0 || m.Speed < 0 {
		return nil, fmt.Errorf("Time / RPM / Speed の3列は必須です（未選択の項目があります）")
	}
	if o.WOTThreshold > 0 && m.TPS < 0 {
		return nil, fmt.Errorf("WOT自動トリムにはTPS列の指定が必要です（チェックを外すかTPSを選択してください）")
	}
	if o.StoichAFR <= 0 {
		o.StoichAFR = 14.7
	}

	// PCLink labels the A/F sensor "ラムダ" while often holding AFR values.
	// Sniff the source, then convert to the requested output (λ unless "afr").
	srcIsAFR := m.Lambda >= 0 && src.SniffLambdaIsAFR(m.Lambda)
	wantLambda := !strings.EqualFold(o.LambdaOut, "afr")

	// 1) extract parseable rows
	recs := make([]rec, 0, src.RowCount())
	for r := 0; r < src.RowCount(); r++ {
		var x rec
		var ok bool
		if x.t, ok = src.Num(r, m.Time); !ok {
			continue
		}
		if x.rpm, ok = src.Num(r, m.RPM); !ok {
			continue
		}
		if x.spd, ok = src.Num(r, m.Speed); !ok {
			continue
		}
		if m.TPS >= 0 {
			x.tps, x.hasTPS = src.Num(r, m.TPS)
		}
		if m.Lambda >= 0 {
			x.lam, x.hasLAM = src.Num(r, m.Lambda)
		}
		if m.Boost >= 0 {
			x.bst, x.hasBST = src.Num(r, m.Boost)
		}
		if m.Gear >= 0 {
			x.gear, x.hasGear = src.Num(r, m.Gear)
		}
		for _, c := range m.Extra {
			x.extra = append(x.extra, src.Cell(r, c))
		}
		recs = append(recs, x)
	}
	stats := Stats{RowsIn: src.RowCount(), LambdaFromAFR: srcIsAFR && wantLambda}

	// 2) trim segment
	seg := recs
	switch {
	case o.HasStart || o.HasEnd:
		stats.TrimMode = "manual"
		var out []rec
		for _, x := range recs {
			if o.HasStart && x.t < o.Start {
				continue
			}
			if o.HasEnd && x.t > o.End {
				continue
			}
			out = append(out, x)
		}
		seg = out
	case o.WOTThreshold > 0:
		stats.TrimMode = "wot"
		runs := findRuns(recs, func(x rec) (float64, bool) { return x.tps, x.hasTPS }, o.WOTThreshold)
		stats.RunCount = len(runs)
		if len(runs) == 0 {
			return nil, fmt.Errorf("TPS≥%.0f%% の区間が見つかりませんでした。閾値を下げるか手動範囲を指定してください", o.WOTThreshold)
		}
		best := runs[0]
		for _, r := range runs[1:] {
			if r.length > best.length {
				best = r
			}
		}
		seg = recs[best.start : best.start+best.length]
	default:
		stats.TrimMode = "all"
	}
	if len(seg) == 0 {
		return nil, fmt.Errorf("選択区間にデータがありません")
	}
	stats.RunStart, stats.RunEnd = seg[0].t, seg[len(seg)-1].t

	// 3) emit with unit conversion
	h := DefaultHeaders(o)
	headers := []string{h.Time}
	if m.Speed >= 0 {
		headers = append(headers, h.Speed)
	}
	headers = append(headers, h.RPM)
	if m.TPS >= 0 {
		headers = append(headers, h.TPS)
	}
	if m.Lambda >= 0 {
		headers = append(headers, h.Lambda)
	}
	if m.Boost >= 0 {
		headers = append(headers, h.Boost)
	}
	if m.Gear >= 0 {
		headers = append(headers, h.Gear)
	}
	headers = append(headers, extraHeaders...)

	speedFactor, speedKnown := speedToMPHFactor(src.Unit(m.Speed), o.SpeedOut)

	out := &Result{Headers: headers}
	for _, x := range seg {
		row := []string{ftoa(x.t, 3)}
		if m.Speed >= 0 {
			v := x.spd
			if speedKnown {
				v *= speedFactor
			}
			row = append(row, ftoa(v, 2))
		}
		row = append(row, ftoa(x.rpm, 0))
		if m.TPS >= 0 {
			row = append(row, ftoa(x.tps, 1))
		}
		if m.Lambda >= 0 {
			v := x.lam
			if srcIsAFR && wantLambda {
				v /= o.StoichAFR
			} else if !srcIsAFR && !wantLambda {
				v *= o.StoichAFR
			}
			row = append(row, ftoa(v, 3))
		}
		if m.Boost >= 0 {
			v := x.bst
			if strings.ToLower(o.BoostOut) == "psi" {
				v *= 0.1450377 // kPa -> psi (PCLink logs boost/gauge pressures in kPa)
			}
			row = append(row, ftoa(v, 1))
		}
		if m.Gear >= 0 {
			row = append(row, ftoa(x.gear, 0))
		}
		row = append(row, x.extra...)
		out.Rows = append(out.Rows, row)
	}
	stats.RowsOut = len(out.Rows)
	out.Stats = stats
	return out, nil
}

type run struct {
	start, length int
}

// findRuns returns contiguous runs where the TPS value stays at or above the
// threshold. Runs shorter than 3 samples are ignored as noise.
func findRuns(recs []rec, tps func(rec) (float64, bool), threshold float64) []run {
	var runs []run
	curStart := -1
	for i, x := range recs {
		v, ok := tps(x)
		hot := ok && v >= threshold
		if hot && curStart < 0 {
			curStart = i
		} else if !hot && curStart >= 0 {
			if i-curStart >= 3 {
				runs = append(runs, run{curStart, i - curStart})
			}
			curStart = -1
		}
	}
	if curStart >= 0 && len(recs)-curStart >= 3 {
		runs = append(runs, run{curStart, len(recs) - curStart})
	}
	return runs
}

// speedToMPHFactor determines the conversion factor from the source unit string
// and desired output unit. Returns (factor, known).
func speedToMPHFactor(srcUnit, outUnit string) (float64, bool) {
	su := strings.ToLower(strings.TrimSpace(srcUnit))
	srcKPH := su == "kph" || su == "km/h" || su == "kmh"
	srcMPH := su == "mph" || su == "mi/h"
	switch strings.ToLower(outUnit) {
	case "mph":
		if srcMPH {
			return 1, true
		}
		if srcKPH {
			return 0.621371, true
		}
	case "kph":
		if srcKPH {
			return 1, true
		}
		if srcMPH {
			return 1.609344, true
		}
	}
	return 1, false
}

// ftoa formats a float with at most dec digits, without trailing zero clutter.
func ftoa(v float64, dec int) string {
	s := fmt.Sprintf("%.*f", dec, v)
	if dec > 0 {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
		if s == "" || s == "-" {
			s = "0"
		}
	}
	return s
}
