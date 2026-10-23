package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nya-buro/pclink2vd/internal/convert"
	"github.com/nya-buro/pclink2vd/internal/pclinkcsv"
)

// runCLI implements headless conversion for batch use.
//
//	pclink2vd -in log.csv -out out.csv [-preset name.json]
//	          [-wot 90 | -full | -start 12.5 -end 22.0]
//	          [-speed mph|kph] [-lambda auto|lambda|afr] [-boost psi|kpa]
//	          [-gear 3] [-segments] [-split 1,2|all]
func runCLI(args []string) int {
	fs := flag.NewFlagSet("pclink2vd", flag.ContinueOnError)
	in := fs.String("in", "", "入力: PCLink CSV エクスポートファイル")
	out := fs.String("out", "", "出力: Virtual Dyno 用 CSV ファイル")
	presetPath := fs.String("preset", "", "マッピング/設定プリセット (JSON)")
	wot := fs.Float64("wot", -1, "WOTトリムTPS閾値%% (デフォルト90, 0で無効)")
	start := fs.Float64("start", 0, "手動開始時刻 (秒)")
	end := fs.Float64("end", 0, "手動終了時刻 (秒)")
	full := fs.Bool("full", false, "トリムせず全ログを変換")
	speed := fs.String("speed", "", "車速出力単位 mph|kph")
	lambda := fs.String("lambda", "", "空燃比出力 auto|lambda|afr")
	boost := fs.String("boost", "", "過給圧出力 psi|kpa")
	gear := fs.Int("gear", -1, "WOT区間をこのギアに絞り込む (0以上)")
	segments := fs.Bool("segments", false, "検出されるWOT区間を一覧表示して終了")
	split := fs.String("split", "", "出力するWOT区間番号 (例: 1,3 または all)。-out名に _splitN が付く")
	listCols := fs.Bool("list", false, "列一覧を表示して終了")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "PCLink G4X CSV → Virtual Dyno CSV converter")
		fmt.Fprintln(os.Stderr, "\nGUI: pclink2vd (引数なし)\n\nCLI:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *in == "" {
		fs.Usage()
		return 2
	}

	tbl, err := pclinkcsv.Parse(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		return 1
	}

	if *listCols {
		for i := range tbl.Headers {
			fmt.Printf("%d\t%s\t%s\n", i, tbl.Header(i), tbl.Unit(i))
		}
		return 0
	}
	needOut := !*segments || *split != ""
	if needOut && *out == "" {
		fmt.Fprintln(os.Stderr, "エラー: -out を指定してください")
		return 2
	}

	m := convert.NewMapping()
	o := convert.DefaultOptions()
	if *presetPath != "" {
		p, err := convert.LoadPreset(*presetPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			return 1
		}
		m, o = p.Mapping, p.Options
	} else {
		g := tbl.GuessColumns()
		m.Time, m.RPM, m.Speed, m.TPS = g.Time, g.RPM, g.Speed, g.TPS
		m.Lambda, m.Boost, m.Gear = g.Lambda, g.Boost, g.Gear
		fmt.Fprintln(os.Stderr, "自動認識:",
			fmt.Sprintf("Time=%s RPM=%s Speed=%s TPS=%s Lambda=%s Boost=%s Gear=%s",
				colDesc(tbl, m.Time), colDesc(tbl, m.RPM), colDesc(tbl, m.Speed),
				colDesc(tbl, m.TPS), colDesc(tbl, m.Lambda), colDesc(tbl, m.Boost), colDesc(tbl, m.Gear)))
	}

	if *wot >= 0 {
		o.WOTThreshold = *wot
	}
	if *speed != "" {
		o.SpeedOut = *speed
	}
	if *lambda != "" {
		o.LambdaOut = *lambda
	}
	if *boost != "" {
		o.BoostOut = *boost
	}
	if *start > 0 {
		o.HasStart, o.Start = true, *start
	}
	if *end > 0 {
		o.HasEnd, o.End = true, *end
	}
	if *full || *start > 0 || *end > 0 {
		o.WOTThreshold = 0
	}

	if *gear >= 0 {
		o.GearFilter = *gear
	}

	extraHdrs := make([]string, 0, len(m.Extra))
	for _, c := range m.Extra {
		extraHdrs = append(extraHdrs, tbl.Header(c))
	}

	// 区間一覧モード: 検出結果を表示して終了
	if *segments && *split == "" {
		segs, err := convert.FindSegments(tbl, m, o)
		if err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			return 1
		}
		if len(segs) == 0 {
			fmt.Fprintln(os.Stderr, "WOT区間が検出されませんでした（-wot の値を確認）")
			return 1
		}
		for _, s := range segs {
			fmt.Println(s.Describe())
		}
		return 0
	}

	// 分割出力モード: 指定区間を name_splitN.csv として個別出力
	if *split != "" {
		segs, err := convert.FindSegments(tbl, m, o)
		if err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			return 1
		}
		idxs, err := parseSplit(*split, segs)
		if err != nil {
			fmt.Fprintln(os.Stderr, "エラー:", err)
			return 2
		}
		base := strings.TrimSuffix(*out, filepath.Ext(*out))
		written := 0
		for _, idx := range idxs {
			o2 := o
			o2.SegmentIndex = idx
			res, err := convert.Run(tbl, m, o2, extraHdrs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "split%d エラー: %v\n", idx, err)
				continue
			}
			path := fmt.Sprintf("%s_split%d.csv", base, idx)
			if err := convert.WriteCSV(path, res); err != nil {
				fmt.Fprintf(os.Stderr, "split%d 出力エラー: %v\n", idx, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "split%d → %s（%d行 / %.1f s）\n", idx, path, res.Stats.RowsOut, res.Stats.RunEnd-res.Stats.RunStart)
			written++
		}
		if written == 0 {
			return 1
		}
		return 0
	}

	res, err := convert.Run(tbl, m, o, extraHdrs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		return 1
	}
	if err := convert.WriteCSV(*out, res); err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		return 1
	}
	fmt.Fprintln(os.Stderr, res.Stats.Describe())
	return 0
}

func colDesc(tbl *pclinkcsv.Table, i int) string {
	if i < 0 {
		return "-"
	}
	return strings.TrimSpace(fmt.Sprintf("%d(%s)", i, tbl.Header(i)))
}

// parseSplit interprets the -split value ("all" or "1,3") against detected segments.
func parseSplit(s string, segs []convert.Segment) ([]int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "all" {
		var out []int
		for _, sg := range segs {
			out = append(out, sg.Index)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("出力可能なWOT区間がありません")
		}
		return out, nil
	}
	valid := map[int]bool{}
	for _, sg := range segs {
		valid[sg.Index] = true
	}
	var out []int
	for _, tok := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(tok))
		if err != nil || n < 1 {
			return nil, fmt.Errorf("-split 指定不正: %s", tok)
		}
		if !valid[n] {
			return nil, fmt.Errorf("区間%dは検出結果に存在しません（-segments で一覧確認できます）", n)
		}
		out = append(out, n)
	}
	return out, nil
}
