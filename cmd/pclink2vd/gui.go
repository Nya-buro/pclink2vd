package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"

	"github.com/nya-buro/pclink2vd/internal/convert"
	"github.com/nya-buro/pclink2vd/internal/pclinkcsv"
)

type app struct {
	mw *walk.MainWindow

	inEdit *walk.LineEdit

	cbTime   *walk.ComboBox
	cbRPM    *walk.ComboBox
	cbSpeed  *walk.ComboBox
	cbTPS    *walk.ComboBox
	cbLambda *walk.ComboBox
	cbBoost  *walk.ComboBox
	cbGear   *walk.ComboBox

	cbSpeedOut  *walk.ComboBox
	cbLambdaOut *walk.ComboBox
	cbBoostOut  *walk.ComboBox

	chkWOT   *walk.CheckBox
	wotEdit  *walk.LineEdit
	startEdit *walk.LineEdit
	endEdit  *walk.LineEdit

	colList    *walk.ListBox
	filterEdit *walk.LineEdit
	extrasEdit *walk.TextEdit

	logEdit *walk.TextEdit

	tbl    *pclinkcsv.Table
	labels []string
}

func runGUI() {
	a := &app{}
	mw := &MainWindow{
		AssignTo: &a.mw,
		Title:    "PCLink G4X ログ → Virtual Dyno 変換",
		Size:     Size{Width: 1020, Height: 800},
		Layout:   VBox{},
		OnDropFiles: func(paths []string) {
			for _, p := range paths {
				if strings.EqualFold(filepath.Ext(p), ".csv") {
					a.loadFile(p)
					return
				}
			}
		},
		Children: []Widget{
			Composite{Layout: HBox{},
				Children: []Widget{
					Label{Text: "入力CSV(PCLinkエクスポート):"},
					LineEdit{AssignTo: &a.inEdit, ReadOnly: true, StretchFactor: 1},
					PushButton{Text: "参照...", OnClicked: a.onBrowse},
				},
			},
			GroupBox{
				Title:  "Virtual Dyno フィールドマッピング（ログから自動認識、変更可）",
				Layout: Grid{Columns: 4},
				Children: []Widget{
					Label{Text: "Time (秒):"}, ComboBox{AssignTo: &a.cbTime},
					Label{Text: "RPM:"}, ComboBox{AssignTo: &a.cbRPM},

					Label{Text: "車速:"}, ComboBox{AssignTo: &a.cbSpeed},
					Label{Text: "TPS (スロットル):"}, ComboBox{AssignTo: &a.cbTPS},

					Label{Text: "ラムダ/AFR:"}, ComboBox{AssignTo: &a.cbLambda},
					Label{Text: "過給圧 (MGP):"}, ComboBox{AssignTo: &a.cbBoost},

					Label{Text: "ギア (任意):"}, ComboBox{AssignTo: &a.cbGear},
					Label{Text: ""}, Label{Text: ""},
				},
			},
			GroupBox{
				Title:  "単位・区間トリム",
				Layout: Grid{Columns: 4},
				Children: []Widget{
					Label{Text: "車速出力:", Row: 0, Column: 0},
					ComboBox{AssignTo: &a.cbSpeedOut, Model: []string{"mph (Virtual Dyno既定)", "km/h"}, Row: 0, Column: 1},
					Label{Text: "空燃比出力:", Row: 0, Column: 2},
					ComboBox{AssignTo: &a.cbLambdaOut, Model: []string{"自動判定", "λ (ランブダ)", "AFR"}, Row: 0, Column: 3},

					Label{Text: "過給圧出力:", Row: 1, Column: 0},
					ComboBox{AssignTo: &a.cbBoostOut, Model: []string{"psi (Virtual Dyno既定)", "kPa"}, Row: 1, Column: 1},
					Label{Text: "", Row: 1, Column: 2},
					Label{Text: "", Row: 1, Column: 3},

					CheckBox{AssignTo: &a.chkWOT, Text: "WOT自動トリム TPS≥", Row: 2, Column: 0},
					LineEdit{AssignTo: &a.wotEdit, Text: "90", MinSize: Size{Width: 50, Height: 22}, Row: 2, Column: 1},
					Label{Text: "% の最長区間のみ出力", Row: 2, Column: 2, ColumnSpan: 2},

					Label{Text: "手動範囲 開始(s):", Row: 3, Column: 0},
					LineEdit{AssignTo: &a.startEdit, MinSize: Size{Width: 80, Height: 22}, Row: 3, Column: 1},
					Label{Text: "終了(s):", Row: 3, Column: 2},
					LineEdit{AssignTo: &a.endEdit, MinSize: Size{Width: 80, Height: 22}, Row: 3, Column: 3},
				},
			},
			GroupBox{
				Title:  "追加出力列（VDは無視、解析用に残したい列。左で選択→追加。手動範囲入力時はWOTトリムより手動が優先）",
				Layout: VBox{},
				Children: []Widget{
					Composite{Layout: HBox{},
						Children: []Widget{
							Label{Text: "絞り込み:"},
							LineEdit{AssignTo: &a.filterEdit, OnTextChanged: a.onFilterChanged, StretchFactor: 1},
							PushButton{Text: "追加 →", OnClicked: a.onAddExtra},
						},
					},
					Composite{Layout: HBox{},
						Children: []Widget{
							ListBox{AssignTo: &a.colList, StretchFactor: 1, MinSize: Size{Width: 400, Height: 120}},
							GroupBox{
								Title:  "追加済み",
								Layout: VBox{},
								Children: []Widget{
									TextEdit{AssignTo: &a.extrasEdit, MinSize: Size{Width: 260, Height: 90}},
									PushButton{Text: "クリア", OnClicked: func() { a.extrasEdit.SetText("") }},
								},
							},
						},
					},
				},
			},
			Composite{Layout: HBox{},
				Children: []Widget{
					PushButton{Text: "プリセット読み込み", OnClicked: a.onLoadPreset},
					PushButton{Text: "プリセット保存", OnClicked: a.onSavePreset},
					HSpacer{Size: 220},
					PushButton{Text: "変換して保存", OnClicked: a.onConvert},
				},
			},
			TextEdit{AssignTo: &a.logEdit, ReadOnly: true, VScroll: true, MinSize: Size{Width: 980, Height: 120}, StretchFactor: 1},
		},
	}
	if err := mw.Create(); err != nil {
		fmt.Fprintln(os.Stderr, "GUI作成失敗:", err)
		os.Exit(1)
	}
	a.chkWOT.SetCheckState(walk.CheckChecked)
	a.log("ログファイル(PCLinkで Logging→Export→CSV で書き出したもの)をここにドラッグ、または[参照...]へ。")
	a.mw.Run()
}

// ---- helpers ----

func (a *app) log(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	a.logEdit.AppendText(line + "\r\n")
}

func (a *app) onBrowse() {
	dlg := walk.FileDialog{
		Title:  "PCLink CSV エクスポートを選択",
		Filter: "CSV ファイル (*.csv)|*.csv|すべてのファイル (*.*)|*.*",
	}
	if dir := a.inEdit.Text(); dir != "" {
		dlg.InitialDirPath = filepath.Dir(dir)
	}
	ok, err := dlg.ShowOpen(a.mw)
	if err != nil {
		a.log("ファイルダイアログエラー: %v", err)
		return
	}
	if ok {
		a.loadFile(dlg.FilePath)
	}
}

func (a *app) loadFile(path string) {
	tbl, err := pclinkcsv.Parse(path)
	if err != nil {
		a.log("解析失敗: %v", err)
		walk.MsgBox(a.mw, "解析エラー", err.Error(), walk.MsgBoxIconError)
		return
	}
	a.tbl = tbl
	a.inEdit.SetText(path)

	a.labels = make([]string, len(tbl.Headers))
	for i := range tbl.Headers {
		a.labels[i] = tbl.ColumnLabel(i)
	}
	for _, cb := range []*walk.ComboBox{a.cbTime, a.cbRPM, a.cbSpeed, a.cbTPS, a.cbLambda, a.cbBoost, a.cbGear} {
		_ = cb.SetModel(a.labels)
		cb.SetCurrentIndex(-1)
	}
	a.applyFilter("")
	_ = a.colList.SetModel(a.allItems())

	g := tbl.GuessColumns()
	setIf := func(cb *walk.ComboBox, i int) {
		if i >= 0 && i < len(a.labels) {
			cb.SetCurrentIndex(i)
		}
	}
	setIf(a.cbTime, g.Time)
	setIf(a.cbRPM, g.RPM)
	setIf(a.cbSpeed, g.Speed)
	setIf(a.cbTPS, g.TPS)
	setIf(a.cbLambda, g.Lambda)
	setIf(a.cbBoost, g.Boost)
	setIf(a.cbGear, g.Gear)

	if g.Time >= 0 && g.Time < len(tbl.Headers) {
		a.log("解析完了: %s — %d列 × %d行 (時刻 %s〜%s s)", filepath.Base(path), len(tbl.Headers), len(tbl.Rows),
			tbl.Rows[0][g.Time], tbl.Rows[len(tbl.Rows)-1][g.Time])
	} else {
		a.log("解析完了: %s — %d列 × %d行", filepath.Base(path), len(tbl.Headers), len(tbl.Rows))
	}
	a.log("自動認識: Time=%s RPM=%s 車速=%s TPS=%s λ=%s 過給=%s ギア=%s",
		gotDesc(tbl, g.Time), gotDesc(tbl, g.RPM), gotDesc(tbl, g.Speed), gotDesc(tbl, g.TPS),
		gotDesc(tbl, g.Lambda), gotDesc(tbl, g.Boost), gotDesc(tbl, g.Gear))
	if g.Time < 0 || g.RPM < 0 || g.Speed < 0 {
		a.log("⚠ 必須列が自動認識できません。上のドロップダウンで選択してください。")
	}
}

func gotDesc(tbl *pclinkcsv.Table, i int) string {
	if i < 0 {
		return "未検出"
	}
	return tbl.Header(i)
}

// ---- extra columns ----

func (a *app) allItems() []string { return a.labels }

func (a *app) onFilterChanged() { a.applyFilter(a.filterEdit.Text()) }

func (a *app) applyFilter(f string) {
	if a.colList == nil {
		return
	}
	f = strings.ToLower(strings.TrimSpace(f))
	var items []string
	for _, l := range a.labels {
		if f == "" || strings.Contains(strings.ToLower(l), f) {
			items = append(items, l)
		}
	}
	if items == nil {
		items = []string{}
	}
	_ = a.colList.SetModel(items)
}

func (a *app) onAddExtra() {
	i := a.colList.CurrentIndex()
	if i < 0 {
		return
	}
	label, _ := a.colList.Model().([]string)
	if label == nil || i >= len(label) {
		return
	}
	cur := strings.TrimSpace(a.extrasEdit.Text())
	if cur == "" {
		a.extrasEdit.SetText(label[i])
	} else {
		a.extrasEdit.SetText(cur + "; " + label[i])
	}
}

// parseExtras resolves the extras text ("MAP; 水温; 42") into column indices.
func (a *app) parseExtras() ([]int, []string, error) {
	text := a.extrasEdit.Text()
	var idxs []int
	var hdrs []string
	seen := map[int]bool{}
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool { return r == ';' || r == ',' || r == '\n' }) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		// strip "12: " label prefix if present
		if c := strings.Index(tok, ": "); c > 0 {
			tok = tok[c+2:]
		}
		idx := -1
		// label like "MAP [kPa]" → name part
		name := tok
		if b := strings.LastIndex(name, " ["); b > 0 && strings.HasSuffix(name, "]") {
			name = name[:b]
		}
		if n, err := strconv.Atoi(name); err == nil && n >= 0 && n < len(a.tbl.Headers) {
			idx = n
		} else {
			lt := strings.ToLower(name)
			for i, h := range a.tbl.Headers {
				if strings.ToLower(h) == lt {
					idx = i
					break
				}
			}
			if idx < 0 {
				for i, h := range a.tbl.Headers {
					if strings.Contains(strings.ToLower(h), lt) {
						idx = i
						break
					}
				}
			}
		}
		if idx < 0 {
			return nil, nil, fmt.Errorf("追加列「%s」が見つかりません", tok)
		}
		if !seen[idx] {
			seen[idx] = true
			idxs = append(idxs, idx)
			hdrs = append(hdrs, a.tbl.Header(idx))
		}
	}
	return idxs, hdrs, nil
}

// ---- conversion ----

func (a *app) collect() (convert.Mapping, convert.Options, []string, error) {
	if a.tbl == nil {
		return convert.Mapping{}, convert.Options{}, nil, fmt.Errorf("先にログファイルを読み込んでください")
	}
	m := convert.NewMapping()
	m.Time = a.cbTime.CurrentIndex()
	m.RPM = a.cbRPM.CurrentIndex()
	m.Speed = a.cbSpeed.CurrentIndex()
	m.TPS = a.cbTPS.CurrentIndex()
	m.Lambda = a.cbLambda.CurrentIndex()
	m.Boost = a.cbBoost.CurrentIndex()
	m.Gear = a.cbGear.CurrentIndex()

	o := convert.DefaultOptions()
	switch a.cbSpeedOut.CurrentIndex() {
	case 1:
		o.SpeedOut = "kph"
	}
	switch a.cbLambdaOut.CurrentIndex() {
	case 1:
		o.LambdaOut = "lambda"
	case 2:
		o.LambdaOut = "afr"
	}
	switch a.cbBoostOut.CurrentIndex() {
	case 1:
		o.BoostOut = "kpa"
	}
	if a.chkWOT.CheckState() == walk.CheckChecked {
		thr, err := strconv.ParseFloat(strings.TrimSpace(a.wotEdit.Text()), 64)
		if err != nil || thr <= 0 {
			return m, o, nil, fmt.Errorf("WOTトリム閾値(%s)が数値ではありません", a.wotEdit.Text())
		}
		o.WOTThreshold = thr
	} else {
		o.WOTThreshold = 0
	}
	if s := strings.TrimSpace(a.startEdit.Text()); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return m, o, nil, fmt.Errorf("手動開始時刻「%s」が数値ではありません", s)
		}
		o.HasStart, o.Start = true, v
	}
	if s := strings.TrimSpace(a.endEdit.Text()); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return m, o, nil, fmt.Errorf("手動終了時刻「%s」が数値ではありません", s)
		}
		o.HasEnd, o.End = true, v
	}

	extraIdx, extraHdrs, err := a.parseExtras()
	if err != nil {
		return m, o, nil, err
	}
	m.Extra = extraIdx
	return m, o, extraHdrs, nil
}

func (a *app) onConvert() {
	m, o, extraHdrs, err := a.collect()
	if err != nil {
		a.log("設定エラー: %v", err)
		walk.MsgBox(a.mw, "設定エラー", err.Error(), walk.MsgBoxIconWarning)
		return
	}
	res, err := convert.Run(a.tbl, m, o, extraHdrs)
	if err != nil {
		a.log("変換エラー: %v", err)
		walk.MsgBox(a.mw, "変換エラー", err.Error(), walk.MsgBoxIconWarning)
		return
	}

	def := strings.TrimSuffix(filepath.Base(a.inEdit.Text()), ".csv") + "_vd.csv"
	dlg := walk.FileDialog{
		Title:      "出力CSVを保存",
		FilePath:   filepath.Join(filepath.Dir(a.inEdit.Text()), def),
		Filter:     "CSV ファイル (*.csv)|*.csv",
	}
	ok, err := dlg.ShowSave(a.mw)
	if err != nil {
		a.log("ファイルダイアログエラー: %v", err)
		return
	}
	if !ok {
		return
	}
	if err := convert.WriteCSV(dlg.FilePath, res); err != nil {
		a.log("出力エラー: %v", err)
		walk.MsgBox(a.mw, "出力エラー", err.Error(), walk.MsgBoxIconError)
		return
	}
	a.log("%s", res.Stats.Describe())
	a.log("出力: %s", dlg.FilePath)
	a.log("Virtual Dyno: File→Load で開き、認識されない場合は Options→Columns&Profiles の Custom で列名を指定してください。")
}

// ---- presets ----

func (a *app) onSavePreset() {
	m, o, _, err := a.collect()
	if err != nil {
		a.log("設定エラー: %v", err)
		return
	}
	dlg := walk.FileDialog{Title: "プリセット保存", Filter: "JSON (*.json)|*.json", FilePath: "preset.json"}
	ok, err := dlg.ShowSave(a.mw)
	if err != nil || !ok {
		return
	}
	if err := convert.SavePreset(dlg.FilePath, convert.Preset{Name: strings.TrimSuffix(dlg.FilePath, filepath.Ext(dlg.FilePath)), Mapping: m, Options: o}); err != nil {
		a.log("プリセット保存失敗: %v", err)
		return
	}
	a.log("プリセット保存: %s", dlg.FilePath)
}

func (a *app) onLoadPreset() {
	dlg := walk.FileDialog{Title: "プリセット読み込み", Filter: "JSON (*.json)|*.json"}
	ok, err := dlg.ShowOpen(a.mw)
	if err != nil || !ok {
		return
	}
	p, err := convert.LoadPreset(dlg.FilePath)
	if err != nil {
		a.log("プリセット読み込み失敗: %v", err)
		return
	}
	if a.tbl == nil {
		a.log("プリセットを読み込みました。ログを開くとマッピングが適用されます（列番号はログごとに同じとは限らない点に注意）。")
		return
	}
	setIf := func(cb *walk.ComboBox, i int) {
		if i >= 0 && i < len(a.labels) {
			cb.SetCurrentIndex(i)
		}
	}
	setIf(a.cbTime, p.Mapping.Time)
	setIf(a.cbRPM, p.Mapping.RPM)
	setIf(a.cbSpeed, p.Mapping.Speed)
	setIf(a.cbTPS, p.Mapping.TPS)
	setIf(a.cbLambda, p.Mapping.Lambda)
	setIf(a.cbBoost, p.Mapping.Boost)
	setIf(a.cbGear, p.Mapping.Gear)

	labels := make([]string, 0, len(p.Mapping.Extra))
	for _, i := range p.Mapping.Extra {
		if i >= 0 && i < len(a.labels) {
			labels = append(labels, a.labels[i])
		}
	}
	a.extrasEdit.SetText(strings.Join(labels, "; "))

	switch strings.ToLower(p.Options.SpeedOut) {
	case "kph":
		a.cbSpeedOut.SetCurrentIndex(1)
	default:
		a.cbSpeedOut.SetCurrentIndex(0)
	}
	switch strings.ToLower(p.Options.LambdaOut) {
	case "lambda":
		a.cbLambdaOut.SetCurrentIndex(1)
	case "afr":
		a.cbLambdaOut.SetCurrentIndex(2)
	default:
		a.cbLambdaOut.SetCurrentIndex(0)
	}
	switch strings.ToLower(p.Options.BoostOut) {
	case "kpa":
		a.cbBoostOut.SetCurrentIndex(1)
	default:
		a.cbBoostOut.SetCurrentIndex(0)
	}
	if p.Options.WOTThreshold > 0 {
		a.chkWOT.SetCheckState(walk.CheckChecked)
	} else {
		a.chkWOT.SetCheckState(walk.CheckUnchecked)
	}
	a.wotEdit.SetText(fmt.Sprintf("%g", p.Options.WOTThreshold))
	if p.Options.HasStart {
		a.startEdit.SetText(fmt.Sprintf("%g", p.Options.Start))
	}
	if p.Options.HasEnd {
		a.endEdit.SetText(fmt.Sprintf("%g", p.Options.End))
	}
	a.log("プリセット読み込み: %s", filepath.Base(dlg.FilePath))
}
