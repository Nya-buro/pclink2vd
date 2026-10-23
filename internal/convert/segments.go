package convert

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// collect validates the mapping and extracts numeric records from the source.
func collect(src Source, m Mapping, o Options) ([]rec, error) {
	if m.Time < 0 || m.RPM < 0 || m.Speed < 0 {
		return nil, fmt.Errorf("Time / RPM / Speed の3列は必須です（未選択の項目があります）")
	}
	if o.WOTThreshold > 0 && m.TPS < 0 {
		return nil, fmt.Errorf("WOT自動トリムにはTPS列の指定が必要です（チェックを外すかTPSを選択してください）")
	}
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
	return recs, nil
}

// Segment describes one detected WOT run with summary statistics, so the
// user can verify and pick pulls before conversion.
type Segment struct {
	Index    int     // 1-based detection number (also used for splitN filenames)
	Start    float64 // seconds
	End      float64
	Rows     int
	SpeedMin float64 // source units (typically km/h)
	SpeedMax float64
	RPMMin   float64
	RPMMax   float64
	BoostMin float64 // kPa
	BoostMax float64
	HasBoost bool
	Gear     int // dominant gear during the run
	HasGear  bool
}

// FindSegments detects all WOT runs (TPS >= threshold) and summarizes each.
// When o.GearFilter > 0 only runs whose dominant gear matches are returned;
// the Index always refers to the original detection order.
func FindSegments(src Source, m Mapping, o Options) ([]Segment, error) {
	if o.WOTThreshold <= 0 {
		return nil, fmt.Errorf("WOTトリム閾値が設定されていません")
	}
	recs, err := collect(src, m, o)
	if err != nil {
		return nil, err
	}
	runs := findRuns(recs, func(x rec) (float64, bool) { return x.tps, x.hasTPS }, o.WOTThreshold)
	segs := make([]Segment, 0, len(runs))
	for i, r := range runs {
		s := summarize(recs[r.start:r.start+r.length])
		s.Index = i + 1
		segs = append(segs, s)
	}
	if o.GearFilter > 0 {
		var kept []Segment
		for _, s := range segs {
			if s.HasGear && s.Gear == o.GearFilter {
				kept = append(kept, s)
			}
		}
		segs = kept
	}
	return segs, nil
}

func summarize(seg []rec) Segment {
	s := Segment{
		Rows:     len(seg),
		SpeedMin: math.Inf(1), SpeedMax: math.Inf(-1),
		RPMMin: math.Inf(1), RPMMax: math.Inf(-1),
		BoostMin: math.Inf(1), BoostMax: math.Inf(-1),
	}
	if len(seg) > 0 {
		s.Start, s.End = seg[0].t, seg[len(seg)-1].t
	}
	gears := map[int]int{}
	for _, x := range seg {
		s.SpeedMin, s.SpeedMax = math.Min(s.SpeedMin, x.spd), math.Max(s.SpeedMax, x.spd)
		s.RPMMin, s.RPMMax = math.Min(s.RPMMin, x.rpm), math.Max(s.RPMMax, x.rpm)
		if x.hasBST {
			s.HasBoost = true
			s.BoostMin, s.BoostMax = math.Min(s.BoostMin, x.bst), math.Max(s.BoostMax, x.bst)
		}
		if x.hasGear && x.gear > 0 {
			gears[int(x.gear)]++
		}
	}
	if len(gears) > 0 {
		best, bestN := 0, -1
		keys := make([]int, 0, len(gears))
		for g := range gears {
			keys = append(keys, g)
		}
		sort.Ints(keys)
		for _, g := range keys {
			if gears[g] > bestN {
				best, bestN = g, gears[g]
			}
		}
		s.Gear, s.HasGear = best, true
	}
	return s
}

// dominantGear returns the most frequent gear in seg (0 when none).
func dominantGear(seg []rec) int {
	s := summarize(seg)
	if !s.HasGear {
		return 0
	}
	return s.Gear
}

// Describe renders a one-line summary for UI/CLI display.
// Speed is shown in the source unit (usually km/h), boost in kPa.
func (s Segment) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "split%d: %.3f〜%.3f s (%.1f s) %dサンプル 車速 %.1f〜%.1f 回転 %.0f〜%.0f",
		s.Index, s.Start, s.End, s.End-s.Start, s.Rows, s.SpeedMin, s.SpeedMax, s.RPMMin, s.RPMMax)
	if s.HasBoost {
		fmt.Fprintf(&b, " 過給圧 %.1f〜%.1f kPa", s.BoostMin, s.BoostMax)
	}
	if s.HasGear {
		fmt.Fprintf(&b, " ギア %d", s.Gear)
	}
	return b.String()
}
