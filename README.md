# pclink2vd — PCLink G4X ログ → Virtual Dyno CSV 変換

LINK G4X (PCLink) のログCSVエクスポートを、Virtual Dyno が読み込めるCSVに変換する
Windows用単一exeツールです（実行環境のインストール不要・GUI付き）。

## 使い方（GUI）

1. PCLink の Logging → Export → CSV でログを書き出す（タイムスタンプは File Time / Section Time どちらも可）
2. `pclink2vd.exe` をダブルクリック
3. CSVファイルをウィンドウにドラッグ&ドロップ（または「参照...」）
4. 列が自動認識されるので、マッピングを確認
   - 例: Time=セクション / RPM=エンジン回転数 / 車速=駆動輪速 / TPS=TPS（メイン） /
     ラムダ=ラムダ 1 / 過給圧=MGP / ギア=Gear
5. 単位を選択（既定は Virtual Dyno 向け: 車速 mph・過給圧 psi・空燃比 λ）
6. 区間トリム:
   - **WOT自動トリム**: TPS≥閾値(既定90%)の最長区間のみ出力
   - **手動範囲**: 開始/終了(s)を入力するとWOTトリムより優先
   - どちらも不要ならチェックを外すと全ログ出力
   - ※フルスロットル区間のないログだとWOTトリムはエラーになります
7. 「変換して保存」→ CSV保存。Virtual Dyno の File→Load で開く

出力CSVのヘッダ: `Time,MPH,RPM,TPS,Lambda,Boost,Gear` (+追加列は元の名前)

設定がお気に入りにったら「プリセット保存」でJSONに保存でき、次回「プリセット読み込み」で
復元できます（CLIとも共用）。

## CLI（一括変換）

```
pclink2vd.exe -in ログ.csv -out 出力.csv [-preset preset.json]
              [-wot 90 | -full | -start 秒 -end 秒]
              [-speed mph|kph] [-lambda auto|lambda|afr] [-boost psi|kpa]
              [-list]
```

※GUIアプリとしてビルドしているため、コンソールからの実行時に出力表示・終了コードが
不安定なことがあります（結果ファイルは正しく出力されます）。

## Virtual Dyno 側の注意

- 既定ヘッダ (Time/MPH/RPM/TPS/Lambda/Boost) は Virtual Dyno が認識する名前です。
  認識しない場合は Options → Columns & Profiles の Custom で列を割り付けてください。
- 空燃比は PCLink の「ラムダ 1」列が実際には AFR 値になっていることを自動判定し、
  λ = AFR / 14.7 に変換して出力します（出力モード切替可）。

## ビルド（開発用）

Go 1.27+ が必要（go.mod 参照）。

```
.\build.ps1     # → dist\pclink2vd.exe
```

マニフェスト(テーマ+HighDPI)は `winres/winres.json` から go-winres で .syso を生成し
（`cmd/pclink2vd/` に作成）、`-H windowsgui -s -w` で単一exeにリンクします。

## リポジトリ構成

```
cmd/pclink2vd/    メインプログラム（GUI / CLI のエントリポイント）
internal/convert/ 変換・トリム・CSV出力ロジック
internal/pclinkcsv/ PCLink CSVのエンコーディング検出・列自動認識
winres/           exeに埋め込むマニフェスト定義
dist/             ビルド出力（gitignore）
```
