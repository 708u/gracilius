# Bubbletea v2 新機能調査

## 概要

Charm v2 (Bubbletea v2, Lipgloss v2, Bubbles v2) は
2026年2月23日にリリースされた。
高度に最適化されたレンダリング、宣言的API、
高忠実度の入力処理を提供する。

モジュールパスは vanity domain に変更:

```txt
charm.land/bubbletea/v2
charm.land/lipgloss/v2
charm.land/bubbles/v2
```

## Bubbletea v2 新機能

### 宣言的View構造体

v1では `View()` が `string` を返していたが、
v2では `tea.View` 構造体を返す。
ターミナル機能の制御を宣言的に行える。

```go
func (m model) View() tea.View {
    v := tea.NewView()
    v.SetContent("Hello, world!")
    v.AltScreen = true
    v.MouseMode = tea.MouseModeCellMotion
    v.ReportFocus = true
    v.WindowTitle = "My App"
    v.Cursor.Shape = tea.CursorBlock
    v.Cursor.Blink = true
    return v
}
```

View構造体の主なフィールド:

| フィールド | 型 | 用途 |
| --- | --- | --- |
| AltScreen | bool | 代替画面バッファ |
| MouseMode | MouseMode | マウスモード |
| ReportFocus | bool | フォーカス報告 |
| WindowTitle | string | タイトル設定 |
| Cursor | Cursor | カーソル制御 |
| ForegroundColor | color.Color | 前景色 |
| BackgroundColor | color.Color | 背景色 |
| ProgressBar | float64 | プログレスバー |
| KeyboardEnhancements | int | キーボード拡張 |
| OnMouse | func | マウスイベントフィルタ |
| DisableBracketedPasteMode | bool | ペースト制御 |

graciliusへの影響:
`View()` の戻り値を `string` から `tea.View` に
変更する必要がある。AltScreenやMouseModeの設定を
Program Optionから View構造体のフィールドに移行する。

### Cursed Renderer (新レンダリングエンジン)

ncursesアルゴリズムに基づいて一から構築された
新しいレンダラー。
速度、効率、正確性が大幅に最適化されている。

主な特徴:

- Mode 2026 (Synchronized Updates) をデフォルトで有効化
  - ターミナルの更新をアトミックに行い、
    ティアリングやカーソルのちらつきを軽減
- Mode 2027 (Unicode Support) を自動有効化
  - ワイド文字や絵文字を正しく処理
  - アプリケーションのレイアウト崩れを防止

graciliusへの影響:
レンダリング最適化は自動的に享受できる。
ワイド文字の表示処理 (`display.go` の
`displayWidth`, `isWideRune` 等) がターミナル側で
サポートされる場合、一部のロジックを簡略化できる
可能性がある。

### キーボード入力の強化

Progressive Keyboard Enhancements により、
従来マッピングできなかったキーの組み合わせを検出可能。

主な変更:

- `tea.KeyMsg` はインタフェースに変更
  - `tea.KeyPressMsg`: キー押下
  - `tea.KeyReleaseMsg`: キーリリース
- フィールド変更:
  - `Type` -> `Code` (rune型)
  - `Runes` -> `Text` (string型)
  - `Alt` -> `Mod` (修飾キー)
- スペースバーは `" "` ではなく `"space"` を返す
- 新フィールド: `ShiftedCode`, `BaseCode`, `IsRepeat`
- `Keystroke()` メソッド追加

新たにマッピング可能なキーの例:

- `shift+enter`
- `ctrl+m` (Enterと区別可能に)
- `super+space`
- キーリリースイベント

graciliusへの影響:
`update.go` 内のすべての `tea.KeyMsg` の
type switchを `tea.KeyPressMsg` に変更する必要がある。
キーリリースイベントの活用で、より精密な
インタラクションが実現可能。

### マウスイベントの改善

`tea.MouseMsg` がインタフェースに変更され、
イベント種別ごとに専用の型が導入された。

```go
// v1
case tea.MouseMsg:
    switch msg.Type {
    case tea.MouseLeft:
    case tea.MouseRelease:
    case tea.MouseWheelUp:
    case tea.MouseMotion:
    }

// v2
case tea.MouseClickMsg:
    // クリック
case tea.MouseReleaseMsg:
    // リリース
case tea.MouseWheelMsg:
    // ホイール
case tea.MouseMotionMsg:
    // モーション
```

座標の取得方法も変更:

```go
// v1
x, y := msg.X, msg.Y

// v2
mouse := msg.Mouse()
x, y := mouse.X, mouse.Y
```

ボタン定数の名称変更:

- `MouseButtonLeft` -> `MouseLeft`
- `MouseButtonRight` -> `MouseRight`
- `MouseButtonMiddle` -> `MouseMiddle`

マウスモードの設定は宣言的に:

```go
// v1
tea.WithMouseCellMotion()

// v2
view.MouseMode = tea.MouseModeCellMotion
```

graciliusへの影響:
`update.go` のマウスイベント処理を全面的に書き換える
必要がある。graciliusはマウスインタラクションを多用
(クリック、ドラッグ、ペインリサイズ、スクロール)
しているため、影響範囲が広い。
ただし、イベント種別ごとに型が分かれることで、
各ハンドラの責務が明確になり可読性が向上する。

### ペーストイベント

v1では `msg.Paste` フラグで判定していたが、
v2では専用のメッセージ型が導入された。

- `tea.PasteMsg`: ペースト内容 (`Content` フィールド)
- `tea.PasteStartMsg`: ペースト開始
- `tea.PasteEndMsg`: ペースト終了

graciliusへの影響:
現時点でペースト処理を実装していない場合、
将来の機能拡張時にこのAPIを活用できる。

### カーソル制御

View構造体の `Cursor` フィールドで
カーソルを直接制御可能。

```go
view.Cursor = tea.Cursor{
    Position: tea.CursorPosition{X: 10, Y: 5},
    Shape:    tea.CursorBlock,
    Color:    lipgloss.Color("#FF0000"),
    Blink:    true,
}
```

graciliusへの影響:
エディタペインでのカーソル表示に活用可能。
現在のカーソル表示ロジックを簡略化できる可能性。

### クリップボードサポート

OSC52による ネイティブクリップボード操作。
SSH経由でもコピー&ペースト可能。

```go
tea.SetClipboard("copied text")
tea.ReadClipboard()
tea.SetPrimaryClipboard("primary selection")
```

graciliusへの影響:
選択範囲のコピー機能を実装する際に活用できる。

### ターミナルクエリ

新しいコマンドでターミナルの情報を問い合わせ可能。

- カーソル位置の取得
- ターミナルバージョンの検出
- モードレポート
- terminfo/termcap ケイパビリティ確認

graciliusへの影響:
ターミナルの能力に応じた表示最適化が可能。

### テスト改善

- `tea.WithWindowSize(w, h)`: 初期ウィンドウサイズ設定
  - ターミナルのモックなしでテスト可能
- `tea.OpenTTY()`: 直接TTYアクセス
- `tea.WithColorProfile(p)`: カラープロファイル指定

graciliusへの影響:
TUIのテストが容易になる。現在テストが書きにくい
UIロジックに対してユニットテストを追加可能。

### その他の変更

- `p.Start()` / `p.StartReturningModel()`
  -> `p.Run()` に統合
- `tea.Sequentially()` -> `tea.Sequence()` にリネーム
- `tea.WindowSize()` -> `tea.RequestWindowSize`
- プログラムメソッド (`EnterAltScreen()`,
  `SetWindowTitle()` 等) は削除、View構造体で設定
- `EnvMsg`: SSH接続時のクライアント側環境変数取得
- rawエスケープシーケンスサポート

## Lipgloss v2 新機能

### 純粋なスタイリングライブラリ化

v1ではLipglossが独自にstdin/stdoutを参照していたが、
v2ではBubbletea がI/Oを管理し、Lipglossに指示を出す。
これによりロックアップの原因となっていた
I/O競合が解消された。

graciliusへの影響:
Bubbleteaとの統合がよりシームレスになる。
カラープロファイルの衝突が発生しなくなる。

### カラーシステムの変更

- 色の型が `lipgloss.TerminalColor` から
  `color.Color` (`image/color`) に変更
- `LightDark()`: ライト/ダークテーマの色選択
- `Complete()`: ターミナル能力に応じた色定義
- 手動カラーダウンサンプリング
  - `lipgloss.Println`, `lipgloss.Printf`,
    `lipgloss.Fprint` を使用

graciliusへの影響:
色定義の方法を変更する必要がある。
`lipgloss.Color("#XXXXXX")` の使用方法自体は
変わらないが、AdaptiveColor等の使い方が変わる。

### レイヤーとキャンバス

コンポジション用の新機能:

- レイヤー: スタイル付きコンテンツの重ね合わせ
- キャンバス: レイヤーの配置

graciliusへの影響:
ペインレイアウトの実装に活用できる可能性がある。

### テーブルの改善

テーブルコンポーネントの多数のバグ修正と
レンダリング改善。

### compatパッケージ

`compat` パッケージで後方互換性を提供。
段階的な移行が可能。

graciliusへの影響:
移行を段階的に進められる。

### パフォーマンス最適化

- `maxRuneWidth` の最適化 (2026年1月)
- `getFirstRuneAsString` のアロケーション削除
  (2025年10月)

## Bubbles v2 新機能

### Viewport (大幅強化)

#### ソフトラップ

```go
vp.SoftWrap = true
```

長い行を自動的に折り返す。

#### 左ガター

```go
vp.LeftGutterFunc = func(
    info viewport.GutterContext,
) string {
    if info.Soft {
        return "     | "
    }
    if info.Index >= info.TotalLines {
        return "   ~ | "
    }
    return fmt.Sprintf(
        "%4d | ", info.Index+1,
    )
}
```

行番号などのガター表示をカスタマイズ可能。
`GutterContext` は以下の情報を提供:

- `Index`: 行インデックス
- `Soft`: ソフトラップ行かどうか
- `TotalLines`: 総行数

#### ハイライト

```go
vp.SetHighlights(
    regexp.MustCompile("pattern").
        FindAllStringIndex(
            vp.GetContent(), -1,
        ),
)
vp.HighlightNext()
vp.HighlightPrevious()
vp.ClearHighlights()
```

テキスト内のパターンハイライトと
ハイライト間のナビゲーション。

#### 水平スクロール

矢印キーやマウスホイールによる水平スクロール対応。

#### コンテンツ管理

```go
vp.SetContentLines([]string{...})
content := vp.GetContent()
```

行単位でのコンテンツ設定と取得。

#### その他

```go
vp.FillHeight = true  // 空行で高さを埋める
vp.StyleLineFunc = func(i int) lipgloss.Style {
    // 行ごとのスタイリング
}
```

graciliusへの影響:
エディタペインの表示に大幅な改善が見込める:

- ソフトラップでコードの水平スクロールを回避
- LeftGutterFuncで行番号表示を簡略化
  (現在 `renderLineWith*` で実装している処理)
- ハイライト機能で検索結果表示を実装可能
- 水平スクロールでワイドなdiffの表示改善
- StyleLineFuncでdiff行の色分けを簡潔に実装可能

### スタイリングシステムの変更

全コンポーネントで `isDark bool` パラメータが必要に。

```go
// v1
styles := component.DefaultStyles()

// v2
styles := component.DefaultStyles(isDark)
```

graciliusへの影響:
ダークテーマ前提であれば `true` を渡せばよい。

### Width/Heightアクセサ

公開フィールドからgetter/setterメソッドに変更。

```go
// v1
m.Width = 40

// v2
m.SetWidth(40)
w := m.Width()
```

### DefaultKeyMapの関数化

```go
// v1
km := textinput.DefaultKeyMap

// v2
km := textinput.DefaultKeyMap()
```

### Progressコンポーネント

```go
// v1
progress.New(
    progress.WithGradient("#5A56E0", "#EE6FF8"),
)

// v2
progress.New(
    progress.WithColors(
        lipgloss.Color("#5A56E0"),
        lipgloss.Color("#EE6FF8"),
    ),
)
```

- `WithScaled(bool)` オプション追加
- `WithColorFunc()` でセルごとの動的カラーリング

### Textarea / Textinput

- スタイルがネスト構造に再編
  (`Styles.Focused` / `Styles.Blurred`)
- 新メソッド: `MoveToBeginning()`, `MoveToEnd()`
- `ScrollPosition()`, `ScrollYOffset()` ゲッター追加
- PageUp/PageDownキーバインド追加 (Textarea)

### Listコンポーネント

- フィルタースタイルが `Styles.Filter` に再編
- `DefaultStyles(isDark)` でテーマ対応

### runeutil パッケージの削除

`charm.land/bubbles/v2` には `runeutil` が含まれない。

## 参考リンク

- [Bubble Tea v2: What's New (Discussion)](https://github.com/charmbracelet/bubbletea/discussions/1374)
- [Bubble Tea v2.0.0 Release](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0)
- [Bubble Tea Upgrade Guide](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md)
- [Lip Gloss v2: What's New (Discussion)](https://github.com/charmbracelet/lipgloss/discussions/506)
- [Bubbles v2 Upgrade Guide](https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md)
- [Charm v2 Blog Post](https://charm.land/blog/v2/)
