package pclinkcsv

import (
	"sort"
	"strconv"
	"strings"
)

// Guess holds auto-detected column indices (-1 when not found).
type Guess struct {
	Time, RPM, Speed, TPS, Lambda, Boost, Gear int
}

// GuessColumns auto-detects the interesting columns of a PCLink export,
// first by parameter name (Japanese and English variants), then by unit.
func (t *Table) GuessColumns() Guess {
	g := Guess{Time: -1, RPM: -1, Speed: -1, TPS: -1, Lambda: -1, Boost: -1, Gear: -1}

	g.Time = t.firstByNames("セクション", "時間", "time", "section", "section time")
	if g.Time < 0 {
		g.Time = t.firstByUnit("s")
	}

	g.RPM = t.firstByNames("エンジン回転数", "engine rpm", "engine speed", "rpm")
	if g.RPM < 0 {
		g.RPM = t.firstByUnitExcluding("rpm", "リミット", "limit", "アイドル", "idle", "target", "目標", "launch", "ローンチ")
	}

	g.Speed = t.firstByNames("駆動輪速", "非駆動輪速", "車速", "vehicle speed", "speed")
	if g.Speed < 0 {
		g.Speed = t.firstByNames("gps speed")
	}
	if g.Speed < 0 {
		g.Speed = t.firstByUnitAny("kph", "km/h", "kmh", "mph")
	}

	g.TPS = t.firstByNames("tps（メイン）", "tps (main)", "tps(main)", "スロットルポジション(メイン)")
	if g.TPS < 0 {
		g.TPS = t.firstByContainsExcluding("tps", "サブ", "sub", "delta", "デルタ", "cal", "エラー", "error", "累積")
	}

	g.Lambda = t.firstByNames("ラムダ 1", "lambda 1", "ラムダ1", "lambda1")
	if g.Lambda < 0 {
		g.Lambda = t.firstByContainsAny("wideband", "ワイドバンド", "afr", "空燃比", "air fuel")
	}

	g.Boost = t.firstByNames("mgp", "過給圧", "boost")
	if g.Boost < 0 {
		g.Boost = t.firstByContainsExcluding("boost", "target", "目標", "error", "エラー")
	}

	g.Gear = t.firstByNames("gear", "ギア位置(n)", "ギア")

	return g
}

func (t *Table) firstByNames(names ...string) int {
	for _, want := range names {
		want = strings.ToLower(want)
		for i, h := range t.Headers {
			if strings.ToLower(h) == want {
				return i
			}
		}
	}
	return -1
}

func (t *Table) firstByContainsAny(sub ...string) int {
	for i, h := range t.Headers {
		lh := strings.ToLower(h)
		for _, s := range sub {
			if strings.Contains(lh, strings.ToLower(s)) {
				return i
			}
		}
	}
	return -1
}

func (t *Table) firstByContainsExcluding(needle string, excl ...string) int {
	needle = strings.ToLower(needle)
	for i, h := range t.Headers {
		lh := strings.ToLower(h)
		if !strings.Contains(lh, needle) {
			continue
		}
		bad := false
		for _, e := range excl {
			if strings.Contains(lh, strings.ToLower(e)) {
				bad = true
				break
			}
		}
		if !bad {
			return i
		}
	}
	return -1
}

func (t *Table) firstByUnit(unit string) int {
	for i, u := range t.Units {
		if strings.EqualFold(u, unit) && t.NumericRatio(i, 400) > 0.9 {
			return i
		}
	}
	return -1
}

func (t *Table) firstByUnitAny(units ...string) int {
	for _, u := range units {
		if i := t.firstByUnit(u); i >= 0 {
			return i
		}
	}
	return -1
}

func (t *Table) firstByUnitExcluding(unit string, excl ...string) int {
	for i, u := range t.Units {
		if !strings.EqualFold(u, unit) {
			continue
		}
		bad := false
		lh := strings.ToLower(t.Headers[i])
		for _, e := range excl {
			if strings.Contains(lh, strings.ToLower(e)) {
				bad = true
				break
			}
		}
		if !bad && t.NumericRatio(i, 400) > 0.9 {
			return i
		}
	}
	return -1
}

// SniffLambdaIsAFR decides whether a lambda column actually holds AFR values
// (PCLink JA labels the A/F sensor "ラムダ" while displaying AFR).
// It samples non-zero values and compares the median against 3.0.
func (t *Table) SniffLambdaIsAFR(col int) bool {
	vals := []float64{}
	step := 1
	if len(t.Rows) > 1000 {
		step = len(t.Rows) / 1000
	}
	for r := 0; r < len(t.Rows); r += step {
		if v, err := strconv.ParseFloat(strings.TrimSpace(t.Rows[r][col]), 64); err == nil && v > 0 {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return true // no data: assume AFR (PCLink default display)
	}
	sort.Float64s(vals)
	med := vals[len(vals)/2]
	return med > 3.0
}
