# Bubbletea v2 Breaking Changes

Bubbletea v2, Lipgloss v2, Bubbles v2 の breaking changes
をまとめたドキュメント。

## 目次

- [1. Import Path の変更](#1-import-path-の変更)
- [2. Bubbletea v2 Breaking Changes](#2-bubbletea-v2-breaking-changes)
  - [2.1 View() の戻り値変更](#21-view-の戻り値変更)
  - [2.2 KeyMsg の変更](#22-keymsg-の変更)
  - [2.3 MouseMsg の変更](#23-mousemsg-の変更)
  - [2.4 Paste 処理の変更](#24-paste-処理の変更)
  - [2.5 削除された Program Options](#25-削除された-program-options)
  - [2.6 削除された Commands](#26-削除された-commands)
  - [2.7 削除された Program Methods](#27-削除された-program-methods)
  - [2.8 リネームされた API](#28-リネームされた-api)
- [3. Lipgloss v2 Breaking Changes](#3-lipgloss-v2-breaking-changes)
  - [3.1 Color 型の変更](#31-color-型の変更)
  - [3.2 Color Downsampling の手動化](#32-color-downsampling-の手動化)
  - [3.3 背景色検出の手動化](#33-背景色検出の手動化)
  - [3.4 AdaptiveColor の変更](#34-adaptivecolor-の変更)
  - [3.5 Renderer の廃止](#35-renderer-の廃止)
- [4. Bubbles v2 Breaking Changes](#4-bubbles-v2-breaking-changes)
  - [4.1 グローバルパターン](#41-グローバルパターン)
  - [4.2 コンポーネント別の変更](#42-コンポーネント別の変更)

## 1. Import Path の変更

全ライブラリの import path が vanity domain に変更された。

```go
// Before (v1)
import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/charmbracelet/bubbles/viewport"
)

// After (v2)
import (
    tea "charm.land/bubbletea/v2"
    "charm.land/lipgloss/v2"
    "charm.land/bubbles/v2/viewport"
)
```

## 2. Bubbletea v2 Breaking Changes

### 2.1 View() の戻り値変更

`View()` の戻り値が `string` から `tea.View` に変更された。
AltScreen やマウスモード等の設定は View の
フィールドで宣言的に行う。

```go
// Before (v1)
func (m model) View() string {
    return "Hello, world!"
}

// After (v2)
func (m model) View() tea.View {
    return tea.NewView("Hello, world!")
}
```

View フィールドで端末機能を宣言的に設定する例:

```go
func (m model) View() tea.View {
    var v tea.View
    v.SetContent("Hello, world!")
    v.AltScreen = true
    v.MouseMode = tea.MouseModeCellMotion
    v.ReportFocus = true
    return v
}
```

`tea.View` の主要フィールド:

| フィールド | 型 | 説明 |
| --- | --- | --- |
| Content | string | 描画内容 |
| AltScreen | bool | 代替スクリーン |
| MouseMode | MouseMode | マウスモード |
| ReportFocus | bool | フォーカスイベント |
| DisableBracketedPasteMode | bool | ペースト制御 |
| WindowTitle | string | ウィンドウタイトル |
| Cursor | *Cursor | カーソル設定 |
| ForegroundColor | color.Color | 前景色 |
| BackgroundColor | color.Color | 背景色 |
| ProgressBar | *ProgressBar | プログレスバー |
| KeyboardEnhancements | KeyboardEnhancement | キーボード拡張 |

### 2.2 KeyMsg の変更

#### 型名の変更

`tea.KeyMsg` はインターフェースになり、
具体的なメッセージ型として
`tea.KeyPressMsg` と `tea.KeyReleaseMsg` に分割された。

```go
// Before (v1)
case tea.KeyMsg:
    switch msg.String() {
    case "q":
        return m, tea.Quit
    }

// After (v2)
case tea.KeyPressMsg:
    switch msg.String() {
    case "q":
        return m, tea.Quit
    }
```

press と release の両方を処理する場合:

```go
case tea.KeyMsg:
    switch key := msg.(type) {
    case tea.KeyPressMsg:
        // key press
    case tea.KeyReleaseMsg:
        // key release
    }
```

#### フィールド名の変更

| v1 | v2 | 説明 |
| --- | --- | --- |
| `msg.Type` | `msg.Code` | rune 型に変更 |
| `msg.Runes` | `msg.Text` | `[]rune` から `string` に |
| `msg.Alt` | `msg.Mod` | `msg.Mod.Contains(tea.ModAlt)` |

#### スペースキーの文字列表現

```go
// Before (v1)
case " ":

// After (v2)
case "space":
```

`key.Code` は `' '`、`key.Text` は `" "` のままだが、
`String()` は `"space"` を返す。

#### Ctrl キーのマッチング

```go
// Before (v1)
case tea.KeyCtrlC:

// After (v2): 文字列マッチング
case tea.KeyPressMsg:
    switch msg.String() {
    case "ctrl+c":
    }

// After (v2): フィールドマッチング
case tea.KeyPressMsg:
    if msg.Code == 'c' && msg.Mod == tea.ModCtrl {
    }
```

#### 新しいフィールド

| フィールド | 型 | 説明 |
| --- | --- | --- |
| `ShiftedCode` | rune | Shift 時のコード |
| `BaseCode` | rune | US PC-101 レイアウト基準 |
| `IsRepeat` | bool | オートリピート検出 |
| `Keystroke()` | string | 修飾キー常時含む |

### 2.3 MouseMsg の変更

#### インターフェース化と型分割

`tea.MouseMsg` はインターフェースになり、
具体型に分割された。

```go
// Before (v1)
case tea.MouseMsg:
    if msg.Action == tea.MouseActionPress &&
       msg.Button == tea.MouseButtonLeft {
        // left click
    }

// After (v2)
case tea.MouseClickMsg:
    if msg.Button == tea.MouseLeft {
        // left click
    }
case tea.MouseReleaseMsg:
    // release
case tea.MouseWheelMsg:
    // scroll
case tea.MouseMotionMsg:
    // movement
```

共通フィールドへのアクセス:

```go
case tea.MouseMsg:
    mouse := msg.Mouse()
    x, y := mouse.X, mouse.Y
```

#### ボタン定数のリネーム

| v1 | v2 |
| --- | --- |
| `tea.MouseButtonLeft` | `tea.MouseLeft` |
| `tea.MouseButtonRight` | `tea.MouseRight` |
| `tea.MouseButtonMiddle` | `tea.MouseMiddle` |
| `tea.MouseButtonWheelUp` | `tea.MouseWheelUp` |
| `tea.MouseButtonWheelDown` | `tea.MouseWheelDown` |
| `tea.MouseButtonWheelLeft` | `tea.MouseWheelLeft` |
| `tea.MouseButtonWheelRight` | `tea.MouseWheelRight` |

#### マウスモードの設定方法

```go
// Before (v1)
p := tea.NewProgram(model{},
    tea.WithMouseCellMotion())

// After (v2)
func (m model) View() tea.View {
    v := tea.NewView("...")
    v.MouseMode = tea.MouseModeCellMotion
    return v
}
```

### 2.4 Paste 処理の変更

`KeyMsg.Paste` フラグは削除され、専用メッセージに分離された。

```go
// Before (v1)
case tea.KeyMsg:
    if msg.Paste {
        m.text += string(msg.Runes)
    }

// After (v2)
case tea.PasteMsg:
    m.text += msg.Content
case tea.PasteStartMsg:
    // paste started
case tea.PasteEndMsg:
    // paste ended
```

### 2.5 削除された Program Options

View フィールドに移行したため削除された。

| 削除された Option | 代替 |
| --- | --- |
| `tea.WithAltScreen()` | `view.AltScreen = true` |
| `tea.WithMouseCellMotion()` | `view.MouseMode = tea.MouseModeCellMotion` |
| `tea.WithMouseAllMotion()` | `view.MouseMode = tea.MouseModeAllMotion` |
| `tea.WithReportFocus()` | `view.ReportFocus = true` |
| `tea.WithoutBracketedPaste()` | `view.DisableBracketedPasteMode = true` |
| `tea.WithInputTTY()` | 削除 (v2 で自動処理) |
| `tea.WithANSICompressor()` | 削除 (レンダラが自動最適化) |

### 2.6 削除された Commands

View フィールドに移行したため削除された。

| 削除された Command | 代替 |
| --- | --- |
| `tea.EnterAltScreen` | `view.AltScreen = true` |
| `tea.ExitAltScreen` | `view.AltScreen = false` |
| `tea.EnableMouseCellMotion` | `view.MouseMode = ...CellMotion` |
| `tea.EnableMouseAllMotion` | `view.MouseMode = ...AllMotion` |
| `tea.DisableMouse` | `view.MouseMode = ...None` |
| `tea.HideCursor` | `view.Cursor = nil` |
| `tea.ShowCursor` | `view.Cursor = &tea.Cursor{...}` |
| `tea.EnableBracketedPaste` | `view.DisableBracketedPasteMode = false` |
| `tea.DisableBracketedPaste` | `view.DisableBracketedPasteMode = true` |
| `tea.EnableReportFocus` | `view.ReportFocus = true` |
| `tea.DisableReportFocus` | `view.ReportFocus = false` |
| `tea.SetWindowTitle("...")` | `view.WindowTitle = "..."` |

### 2.7 削除された Program Methods

| 削除された Method | 代替 |
| --- | --- |
| `p.Start()` | `p.Run()` |
| `p.StartReturningModel()` | `p.Run()` |
| `p.EnterAltScreen()` | `view.AltScreen = true` |
| `p.ExitAltScreen()` | `view.AltScreen = false` |
| `p.EnableMouseCellMotion()` | View フィールド |
| `p.DisableMouseCellMotion()` | View フィールド |
| `p.EnableMouseAllMotion()` | View フィールド |
| `p.DisableMouseAllMotion()` | View フィールド |
| `p.SetWindowTitle(...)` | View フィールド |

### 2.8 リネームされた API

| v1 | v2 | 備考 |
| --- | --- | --- |
| `tea.Sequentially(...)` | `tea.Sequence(...)` | v1 で非推奨だった |
| `tea.WindowSize()` | `tea.RequestWindowSize` | Msg を直接返す |

## 3. Lipgloss v2 Breaking Changes

### 3.1 Color 型の変更

`lipgloss.TerminalColor` インターフェースと
文字列ベースの `Color` 型が、
Go 標準の `color.Color` インターフェースに変更された。

```go
// Before (v1)
style := lipgloss.NewStyle().
    Foreground(lipgloss.Color("212")).
    Background(lipgloss.Color("#7D56F4"))

// After (v2)
style := lipgloss.NewStyle().
    Foreground(lipgloss.Color("#7D56F4")).
    Background(lipgloss.Color("212"))
```

16 進数と整数フォーマットでの色定義は廃止された。
`lipgloss.Color()` は `color.Color` を返す。

### 3.2 Color Downsampling の手動化

v1 では自動的に色のダウンサンプリングが行われたが、
v2 では Bubbletea 使用時は自動、
スタンドアロンでは手動で行う必要がある。

```go
// Before (v1): fmt.Println で自動ダウンサンプリング
fmt.Println(style.Render("Hello"))

// After (v2): lipgloss の出力関数を使用
lipgloss.Println(style.Render("Hello"))
lipgloss.Printf("%s\n", style.Render("Hello"))
```

Bubbletea v2 と併用する場合は変更不要
(ダウンサンプリングは Bubbletea v2 に組み込まれている)。

### 3.3 背景色検出の手動化

v1 では stdin/stdout を自動的に参照していたが、
v2 では明示的に指定する必要がある。

```go
// Before (v1): 自動検出
lipgloss.HasDarkBackground()

// After (v2): 明示的な I/O 指定
lipgloss.HasDarkBackground(in, out)
```

Bubbletea v2 と併用する場合は
`tea.RequestBackgroundColor` を使用し、
`tea.BackgroundColorMsg` で受け取る。

### 3.4 AdaptiveColor の変更

`AdaptiveColor` は `isDark bool` パラメータを
明示的に受け取る方式に変更された。

```go
// Before (v1): 自動で明暗を判定
color := lipgloss.AdaptiveColor{
    Light: "235",
    Dark: "252",
}

// After (v2): LightDark ヘルパーで明示的に選択
ld := lipgloss.LightDark(isDark)
color := ld(
    lipgloss.Color("#000000"),  // light
    lipgloss.Color("#FFFFFF"),  // dark
)
```

互換パッケージ `charm.land/lipgloss/v2/compat` で
v1 互換の `AdaptiveColor`, `CompleteColor`,
`CompleteAdaptiveColor` が利用可能(非推奨)。

### 3.5 Renderer の廃止

v1 の `lipgloss.Renderer` は廃止された。
Lipgloss v2 はスタイルが純粋(副作用なし)になったため、
Renderer を経由する必要がなくなった。

```go
// Before (v1)
r := lipgloss.DefaultRenderer()
style := r.NewStyle().Foreground(lipgloss.Color("212"))

// After (v2)
style := lipgloss.NewStyle().
    Foreground(lipgloss.Color("212"))
```

`DefaultStylesWithRenderer()` のような関数も削除された。
`DefaultStyles()` を直接使用する。

## 4. Bubbles v2 Breaking Changes

### 4.1 グローバルパターン

全コンポーネントに共通する変更パターン。

#### tea.KeyMsg から tea.KeyPressMsg への変更

```go
// Before (v1)
case tea.KeyMsg:

// After (v2)
case tea.KeyPressMsg:
```

#### Width/Height のフィールドからメソッドへの変更

多くのコンポーネントで Width/Height が
getter/setter メソッドに変更された。

```go
// Before (v1)
m.Width = 40
m.Height = 20
fmt.Println(m.Width, m.Height)

// After (v2)
m.SetWidth(40)
m.SetHeight(20)
fmt.Println(m.Width(), m.Height())
```

影響を受けるコンポーネント:
filepicker, help, progress, table, textinput, viewport

#### DefaultKeyMap の変数から関数への変更

グローバル可変の `DefaultKeyMap` 変数が
関数に変更された。

```go
// Before (v1)
km := textinput.DefaultKeyMap

// After (v2)
km := textinput.DefaultKeyMap()
```

影響を受けるコンポーネント:
paginator, textarea, textinput

#### DefaultStyles への isDark パラメータ追加

Lipgloss v2 で `AdaptiveColor` が廃止されたため、
`DefaultStyles` に `isDark bool` パラメータが必要になった。

```go
// Before (v1)
styles := help.DefaultStyles()

// After (v2)
styles := help.DefaultStyles(isDark)
```

#### NewModel のエイリアス削除

非推奨の `NewModel` エイリアスが削除された。
`New()` を直接使用する。

```go
// Before (v1)
h := help.NewModel()

// After (v2)
h := help.New()
```

影響を受けるコンポーネント:
help, list, paginator, spinner, textinput

#### 内部パッケージ化

`runeutil` と `memoization` パッケージは
内部パッケージ化され、外部から import できなくなった。

### 4.2 コンポーネント別の変更

#### Cursor

| v1 | v2 | 説明 |
| --- | --- | --- |
| `model.Blink` | `model.IsBlinked()` | 状態取得 |
| `model.BlinkCmd()` | `model.Blink()` | コマンド実行 |

#### Filepicker

| v1 | v2 |
| --- | --- |
| `DefaultStylesWithRenderer(r)` | `DefaultStyles()` |
| `model.Height = 10` | `model.SetHeight(10)` |
| `model.Height` (読取) | `model.Height()` |

#### Help

| v1 | v2 |
| --- | --- |
| `NewModel()` | `New()` |
| `model.Width = 80` | `model.SetWidth(80)` |
| `model.Width` (読取) | `model.Width()` |
| `DefaultStyles()` | `DefaultStyles(isDark)` |

新規追加:

- `DefaultDarkStyles() Styles`
- `DefaultLightStyles() Styles`

#### List

| v1 | v2 |
| --- | --- |
| `NewModel(...)` | `New(...)` |
| `DefaultStyles()` | `DefaultStyles(isDark)` |
| `NewDefaultItemStyles()` | `NewDefaultItemStyles(isDark)` |
| `styles.FilterPrompt` | `styles.Filter.Focused.Prompt` |
| `styles.FilterCursor` | `styles.Filter.Cursor` |

#### Paginator

| v1 | v2 |
| --- | --- |
| `NewModel(...)` | `New(...)` |
| `DefaultKeyMap` (変数) | `DefaultKeyMap()` (関数) |
| `model.UsePgUpPgDownKeys` | 削除 (KeyMap で直接設定) |
| `model.UseLeftRightKeys` | 削除 (KeyMap で直接設定) |
| `model.UseUpDownKeys` | 削除 (KeyMap で直接設定) |
| `model.UseHLKeys` | 削除 (KeyMap で直接設定) |
| `model.UseJKKeys` | 削除 (KeyMap で直接設定) |

#### Progress

Width の変更:

```go
// Before (v1)
p.Width = 40

// After (v2)
p.SetWidth(40)
p.Width() // getter
```

色の型変更 (`string` から `color.Color` へ):

```go
// Before (v1)
p.FullColor = "#FF0000"

// After (v2)
p.FullColor = lipgloss.Color("#FF0000")
```

グラデーション/ブレンドオプション:

| v1 | v2 |
| --- | --- |
| `WithGradient(a, b string)` | `WithColors(colors ...color.Color)` |
| `WithDefaultGradient()` | `WithDefaultBlend()` |
| `WithScaledGradient(a, b)` | `WithColors(...) + WithScaled(true)` |
| `WithDefaultScaledGradient()` | `WithDefaultBlend() + WithScaled(true)` |
| `WithSolidFill(string)` | `WithColors(color)` (単色) |
| `WithColorProfile(...)` | 削除 (自動処理) |

`Update()` の戻り値が `(tea.Model, tea.Cmd)` から
`(Model, tea.Cmd)` に変更された。

#### Spinner

| v1 | v2 |
| --- | --- |
| `NewModel()` | `New()` |
| `spinner.Tick()` (パッケージ関数) | `model.Tick()` (メソッド) |

#### Stopwatch

```go
// Before (v1)
sw := stopwatch.NewWithInterval(500 * time.Millisecond)

// After (v2)
sw := stopwatch.New(
    stopwatch.WithInterval(500 * time.Millisecond),
)
```

#### Table

| v1 | v2 |
| --- | --- |
| `model.viewport.Width` | `model.Width()` / `model.SetWidth(w)` |
| `model.viewport.Height` | `model.Height()` / `model.SetHeight(h)` |

#### Textarea

KeyMap:

```go
// Before (v1)
km := textarea.DefaultKeyMap

// After (v2)
km := textarea.DefaultKeyMap()
```

Styles 構造の変更:

| v1 | v2 |
| --- | --- |
| `textarea.Style` (型) | `textarea.StyleState` (型) |
| `model.FocusedStyle` | `model.Styles.Focused` |
| `model.BlurredStyle` | `model.Styles.Blurred` |
| `DefaultStyles()` (2値返却) | `DefaultStyles(isDark) Styles` |

Cursor の変更:

| v1 | v2 |
| --- | --- |
| `ta.Cursor` (cursor.Model) | `ta.Cursor()` (*tea.Cursor) |
| `ta.SetCursor(col)` | `ta.SetCursorColumn(col)` |

新規追加: `Column()`, `ScrollYOffset()`,
`ScrollPosition()`, `MoveToBeginning()`, `MoveToEnd()`

#### Textinput

KeyMap:

```go
// Before (v1)
km := textinput.DefaultKeyMap

// After (v2)
km := textinput.DefaultKeyMap()
```

Width:

```go
// Before (v1)
ti.Width = 40

// After (v2)
ti.SetWidth(40)
```

Styles 構造の変更:

| v1 | v2 |
| --- | --- |
| `textinput.Style` (型) | `textinput.StyleState` (型) |
| `model.FocusedStyle` | `model.Styles.Focused` |
| `model.BlurredStyle` | `model.Styles.Blurred` |
| `DefaultStyles()` (2値返却) | `DefaultStyles(isDark) Styles` |

#### Timer

breaking changes なし。

#### Viewport

Width/Height/YOffset が getter/setter に変更された。

```go
// Before (v1)
vp.Width = 80
vp.Height = 24

// After (v2)
vp.SetWidth(80)
vp.SetHeight(24)
vp.Width()  // getter
vp.Height() // getter
```

## 参考リンク

- [Bubbletea v2 UPGRADE_GUIDE](https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md)
- [Bubbletea v2.0.0 Release](https://github.com/charmbracelet/bubbletea/releases/tag/v2.0.0)
- [Bubbles v2 UPGRADE_GUIDE](https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md)
- [Lipgloss v2 Discussion](https://github.com/charmbracelet/lipgloss/discussions/506)
- [Lipgloss v2.0.0 Release](https://github.com/charmbracelet/lipgloss/releases/tag/v2.0.0)
- [Bubbletea v2 Discussion](https://github.com/charmbracelet/bubbletea/discussions/1374)
