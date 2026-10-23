package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nya-buro/pclink2vd/internal/convert"
	"github.com/nya-buro/pclink2vd/internal/pclinkcsv"
)

// runCLI implements headless conversion for batch use.
//
//	pclink2vd -in log.csv -out out.csv [-preset name.json]
//	          [-wot 90 | -full | -start 12.5 -end 22.0]
//	          [-speed mph|kph] [-lambda auto|lambda|afr] [-boost psi|kpa]
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
	if *out == "" {
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

	extraHdrs := make([]string, 0, len(m.Extra))
	for _, c := range m.Extra {
		extraHdrs = append(extraHdrs, tbl.Header(c))
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
