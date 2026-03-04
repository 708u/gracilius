# Bubbletea Usage Analysis

graciliusコードベースにおけるbubbletea/lipgloss/bubbles
の全使用箇所を網羅的にまとめたドキュメント。

## go.mod バージョン

```txt
github.com/charmbracelet/bubbles    v1.0.0
github.com/charmbracelet/bubbletea  v1.3.10
github.com/charmbracelet/lipgloss   v1.1.0
github.com/charmbracelet/x/ansi     v0.11.6
```

## 1. bubbletea (tea) 使用箇所

### 1.1 import

| ファイル | import形式 |
| --- | --- |
| `cmd/gra/main.go:15` | `tea "github.com/charmbracelet/bubbletea"` |
| `internal/tui/update.go:13` | `tea "github.com/charmbracelet/bubbletea"` |
| `internal/tui/watch.go:7` | `tea "github.com/charmbracelet/bubbletea"` |
| `internal/tui/openfile.go:12` | `tea "github.com/charmbracelet/bubbletea"` |

### 1.2 tea.Model インターフェース実装

`Model` 構造体が `tea.Model` を実装。

#### Init()

- `internal/tui/update.go:29`

```go
func (m *Model) Init() tea.Cmd {
    return tea.Batch(m.watchFile(), m.watchDir())
}
```

#### Update()

- `internal/tui/update.go:93`

```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
```

#### View()

- `internal/tui/view.go:32`

```go
func (m *Model) View() string {
```

### 1.3 tea.NewProgram / Program オプション

- `cmd/gra/main.go:116-120`

```go
p := tea.NewProgram(m,
    tea.WithAltScreen(),
    tea.WithMouseCellMotion(),
    tea.WithContext(ctx),
)
```

### 1.4 Program.Send()

コールバック経由でTUIにメッセージを送信。

- `cmd/gra/main.go:125`
  `p.Send(tui.OpenDiffMsg{...})`
- `cmd/gra/main.go:133`
  `p.Send(tui.CloseDiffMsg{})`
- `cmd/gra/main.go:138`
  `p.Send(tui.IdeConnectedMsg{})`

### 1.5 Program.Run()

- `cmd/gra/main.go:141`

```go
if _, err := p.Run(); err != nil &&
    !errors.Is(err, tea.ErrProgramKilled) {
```

### 1.6 tea.Cmd / tea.Batch

- `internal/tui/update.go:30`
  `tea.Batch(m.watchFile(), m.watchDir())`
- `internal/tui/update.go:290`
  `var cmd tea.Cmd` (複数箇所で使用)

### 1.7 tea.Quit

- `internal/tui/update.go:293`
  `return m, tea.Quit`

### 1.8 tea.ErrProgramKilled

- `cmd/gra/main.go:141`
  `errors.Is(err, tea.ErrProgramKilled)`

### 1.9 tea.Tick

- `internal/tui/update.go:296-298`

```go
return m, tea.Tick(quitTimeout,
    func(time.Time) tea.Msg {
        return quitTimeoutMsg{}
    })
```

- `internal/tui/update.go:520-522`

```go
return m, tea.Tick(statusClearTimeout,
    func(time.Time) tea.Msg {
        return statusClearMsg{}
    })
```

### 1.10 tea.Msg 型

#### tea.WindowSizeMsg

- `internal/tui/update.go:98` (型スイッチ)
- `internal/tui/update.go:159`
  `msg.Width`, `msg.Height` を使用

#### tea.MouseMsg

- `internal/tui/update.go:98` (型スイッチ)
- `internal/tui/update.go:169`
  `msg.Button`, `msg.Action`, `msg.X`, `msg.Y` を使用

#### tea.KeyMsg

- `internal/tui/update.go:98` (型スイッチ)
- `internal/tui/update.go:289`
  `msg.Type` を使用
- `internal/tui/update.go:326`
  `msg.Type == tea.KeyEnter` の比較
- `internal/tui/openfile.go:204-206`
  `msg.Type` でキー種別を判定

#### カスタム Msg 型

- `internal/tui/model.go:30-47`
  - `OpenDiffMsg` (exported)
  - `CloseDiffMsg` (exported)
  - `IdeConnectedMsg` (exported)
  - `fileChangedMsg` (unexported)
  - `treeChangedMsg` (unexported)
- `internal/tui/update.go:23-26`
  - `quitTimeoutMsg` (unexported)
  - `statusClearMsg` (unexported)

### 1.11 Mouse 定数

- `tea.MouseButtonLeft`
  - `internal/tui/update.go:171,188,201,239`
- `tea.MouseButtonWheelUp`
  - `internal/tui/update.go:275`
- `tea.MouseButtonWheelDown`
  - `internal/tui/update.go:280`
- `tea.MouseActionPress`
  - `internal/tui/update.go:171,188,201,242`
- `tea.MouseActionMotion`
  - `internal/tui/update.go:192,252`
- `tea.MouseActionRelease`
  - `internal/tui/update.go:196,260,268`

### 1.12 Key 定数

- `tea.KeyEnter`
  - `internal/tui/update.go:326,359`
- `tea.KeyUp`
  - `internal/tui/openfile.go:206`
- `tea.KeyDown`
  - `internal/tui/openfile.go:206`

### 1.13 tea.Cmd を返す関数

#### watchFile()

- `internal/tui/watch.go:12`

```go
func (m *Model) watchFile() tea.Cmd {
    return func() tea.Msg { ... }
}
```

#### watchDir()

- `internal/tui/watch.go:44`

```go
func (m *Model) watchDir() tea.Cmd {
    return func() tea.Msg { ... }
}
```

## 2. lipgloss 使用箇所

### 2.1 import

| ファイル | import形式 |
| --- | --- |
| `internal/tui/view.go:8` | `"github.com/charmbracelet/lipgloss"` |
| `internal/tui/highlight.go:10` | `"github.com/charmbracelet/lipgloss"` |
| `internal/tui/welcome.go:6` | `"github.com/charmbracelet/lipgloss"` |
| `internal/tui/display.go:6` | `"github.com/charmbracelet/lipgloss"` |
| `internal/tui/openfile.go:13` | `"github.com/charmbracelet/lipgloss"` |

### 2.2 lipgloss.NewStyle()

| ファイル:行 | 用途 |
| --- | --- |
| `view.go:19` | `styleComment` |
| `view.go:20` | `styleInput` |
| `view.go:21` | `styleBodyWhite` |
| `view.go:22-24` | `styleFooter` (BorderTop, BorderStyle) |
| `view.go:28` | `styleTreeCursor()` (Background) |
| `view.go:103` | `styleActive` (Foreground) |
| `view.go:105` | `styleInactive` (Foreground) |
| `view.go:107` | `styleBorder` (Foreground) |
| `view.go:234-235` | 動的スタイル (Background, isActiveFile) |
| `welcome.go:57-67` | 5つのスタイル (Primary/Secondary/Section/Leaf/Trunk) |
| `display.go:11` | `padRight` 内で `lipgloss.NewStyle().Width()` |
| `openfile.go:33,34` | delegate内の `matchStyle`, `selBgStyle` |
| `openfile.go:58` | 空スタイル `lipgloss.Style{}` |
| `openfile.go:98-102` | overlay delegate styles |
| `openfile.go:117` | textinput PromptStyle |
| `openfile.go:333-339` | overlay box スタイル (Border, Padding, Width, Height) |

### 2.3 lipgloss.Color()

全てのスタイル定義で使用。テーマ設定値から色を生成。

代表的な箇所:

- `view.go:19-28` (styleComment, styleInput, styleBodyWhite, styleTreeCursor)
- `view.go:104,106,108` (tab bar styles)
- `view.go:235` (active file background)
- `welcome.go:58-67` (welcome screen styles)
- `openfile.go:60,62,99,101,117,335` (open-file overlay)

### 2.4 lipgloss.Border / BorderTop / BorderStyle

- `view.go:12-14` separatorBorder (カスタムBorder)

```go
var separatorBorder = lipgloss.Border{Top: "\u2500"}
```

- `view.go:23-24` styleFooter

```go
styleFooter = lipgloss.NewStyle().
    BorderTop(true).
    BorderStyle(separatorBorder)
```

- `openfile.go:334` `lipgloss.RoundedBorder()`

### 2.5 lipgloss.Width() / .Render()

- `display.go:11` `lipgloss.NewStyle().Width(width).Render(s)`
- `view.go:76-77` `styleFooter.Width(m.width).Render(footer)`
- `openfile.go:333-339` overlay box `.Width()`, `.Height()`,
  `.Render()`
- `welcome.go:104` `lipgloss.Width(l)` (文字列幅計算)

### 2.6 lipgloss.JoinHorizontal / JoinVertical

- `view.go:65-70`

```go
content := lipgloss.JoinHorizontal(
    lipgloss.Top, ...)
```

- `view.go:81-87`

```go
base := lipgloss.JoinVertical(
    lipgloss.Left, ...)
```

### 2.7 lipgloss.HasDarkBackground()

- `highlight.go:63`

```go
if lipgloss.HasDarkBackground() {
    activeTheme = darkTheme
} else {
    activeTheme = lightTheme
}
```

### 2.8 lipgloss.StyleRunes()

- `openfile.go:63`

```go
pathStr = lipgloss.StyleRunes(
    pathStr, fi.matchedRunes, ms, us)
```

### 2.9 lipgloss.Style メソッド一覧

使用されているメソッド:

- `.Foreground()` - テキスト前景色
- `.Background()` - テキスト背景色
- `.Bold()` - 太字
- `.Width()` - 幅指定
- `.Height()` - 高さ指定
- `.Render()` - レンダリング
- `.BorderTop()` - 上辺ボーダー
- `.BorderStyle()` - ボーダースタイル
- `.Border()` - ボーダー全体
- `.BorderForeground()` - ボーダー前景色
- `.Padding()` - パディング

## 3. bubbles 使用箇所

### 3.1 import

| ファイル | パッケージ |
| --- | --- |
| `internal/tui/keys.go:4` | `bubbles/help` |
| `internal/tui/keys.go:5` | `bubbles/key` |
| `internal/tui/model.go:7` | `bubbles/help` |
| `internal/tui/update.go:12` | `bubbles/key` |
| `internal/tui/tab.go:6` | `bubbles/textarea` |
| `internal/tui/openfile.go:10` | `bubbles/list` |
| `internal/tui/openfile.go:11` | `bubbles/textinput` |
| `internal/tui/openfile_test.go:8` | `bubbles/list` |

### 3.2 bubbles/key

#### key.Binding

- `internal/tui/keys.go:9-31`
  `keyMap` 構造体の全21フィールド

#### key.NewBinding / key.WithKeys / key.WithHelp

- `internal/tui/keys.go:35-123`
  全21キーバインド定義

#### key.Matches()

- `internal/tui/update.go:291`
  `key.Matches(msg, m.keys.Quit)`
- `internal/tui/update.go:303,307,341,356`
  各キーバインドとのマッチ
- `internal/tui/update.go:372-557`
  switchブロック内の全キーマッチ

#### key.Binding.SetEnabled()

- `internal/tui/keys.go:148-156`
  コンテキストに応じたキー有効/無効切替

#### key.Binding.Help().Key

- `internal/tui/update.go:326,334`
  ヘルプテキスト取得 (`m.keys.CommentSubmit.Help().Key`)

### 3.3 bubbles/help

#### help.Model

- `internal/tui/model.go:84`
  `help help.Model` (Model構造体フィールド)

#### help.New()

- `internal/tui/model.go:207`
  `help: help.New()`

#### help.Model.View()

- `internal/tui/view.go:167`
  `m.help.View(m.contextKeyMap())`

#### help.Model.Width

- `internal/tui/view.go:166`
  `m.help.Width = m.width`

#### help.KeyMap インターフェース

- `internal/tui/keys.go:127-141`
  `ShortHelp()` / `FullHelp()` メソッド実装

### 3.4 bubbles/textarea

#### textarea.Model

- `internal/tui/tab.go:40`
  `commentInput textarea.Model` (tab構造体フィールド)

#### textarea.New()

- `internal/tui/tab.go:47`

```go
ta := textarea.New()
ta.Placeholder = "Enter comment..."
ta.CharLimit = 2000
ta.SetHeight(3)
ta.ShowLineNumbers = false
ta.Prompt = ""
```

#### textarea.Model メソッド

- `.Update()` - `update.go:329`
- `.Value()` - `update.go:309,325`
- `.Reset()` - `update.go:305,503`
- `.Blur()` - `update.go:306,322`
- `.Focus()` - `update.go:509`
- `.SetWidth()` - `update.go:500-501`
- `.SetHeight()` - `update.go:327,331`
- `.Height()` - `update.go:326,331`
- `.SetValue()` - `update.go:507`
- `.View()` - `view.go:336`

### 3.5 bubbles/list

#### list.Model

- `internal/tui/openfile.go:90`
  `list list.Model` (openFileOverlay構造体フィールド)

#### list.New()

- `internal/tui/openfile.go:105`

```go
l := list.New(nil, delegate, 0, 0)
```

#### list.Model メソッド

- `.SetShowTitle(false)` - `openfile.go:106`
- `.SetShowStatusBar(false)` - `openfile.go:107`
- `.SetShowHelp(false)` - `openfile.go:108`
- `.SetShowPagination(false)` - `openfile.go:109`
- `.SetFilteringEnabled(false)` - `openfile.go:110`
- `.SetShowFilter(false)` - `openfile.go:111`
- `.DisableQuitKeybindings()` - `openfile.go:112`
- `.SetItems()` - `openfile.go:172,184,197`
- `.Items()` - `openfile.go:297`
- `.Update()` - `openfile.go:208`
- `.SelectedItem()` - `openfile.go:312`
- `.Select()` - `openfile.go:301`
- `.SetSize()` - `openfile.go:326`
- `.Width()` - `openfile.go:69`
- `.Index()` - `openfile.go:52`
- `.View()` - `openfile.go:331`
- `.Paginator.GetSliceBounds()` - `openfile.go:298`

#### list.Item インターフェース

- `internal/tui/openfile.go:19-27`
  `fileItem` が実装 (`Title()`, `Description()`,
  `FilterValue()`)

#### list.ItemDelegate インターフェース

- `internal/tui/openfile.go:30-38`
  `openFileDelegate` が実装 (`Height()`, `Spacing()`,
  `Update()`, `Render()`)

### 3.6 bubbles/textinput

#### textinput.Model

- `internal/tui/openfile.go:89`
  `input textinput.Model` (openFileOverlay構造体フィールド)

#### textinput.New()

- `internal/tui/openfile.go:114`

```go
ti := textinput.New()
ti.Placeholder = "Open file..."
ti.Prompt = "..."
ti.PromptStyle = lipgloss.NewStyle()
```

#### textinput.Model メソッド

- `.Reset()` - `openfile.go:160`
- `.Focus()` - `openfile.go:161`
- `.Value()` - `openfile.go:177,211,214`
- `.SetValue()` - (applyFilterから間接利用)
- `.Update()` - `openfile.go:213,223`
- `.View()` - `openfile.go:329`
- `.Width` (フィールド) - `openfile.go:327`
- `.Cursor.BlinkCmd()` - `openfile.go:164`

## 4. charmbracelet/x/ansi 使用箇所

### 4.1 import

| ファイル | import形式 |
| --- | --- |
| `internal/tui/view.go:9` | `"github.com/charmbracelet/x/ansi"` |
| `internal/tui/openfile.go:14` | `"github.com/charmbracelet/x/ansi"` |

### 4.2 使用API

- `ansi.Truncate()` - `view.go:141,143,227`
  文字列をディスプレイ幅で切り詰め
- `ansi.StringWidth()` - `openfile.go:72,77,367,394,395`
  文字列のディスプレイ幅計算
- `ansi.TruncateLeft()` - `openfile.go:402`
  左側からの切り詰め

## 5. bubbletea関連で使用していない主要API

以下は現在のコードベースで使用されていない主要API:

- `tea.Sequence`
- `tea.Println` / `tea.Printf`
- `tea.QuitMsg` (tea.Quit経由で間接的に使用)
- `tea.WithInputTTY`
- `tea.WithOutput`
- `tea.EnterAltScreen` / `tea.ExitAltScreen` (Cmd版)

## 6. ファイル別サマリ

| ファイル | bubbletea | lipgloss | bubbles |
| --- | --- | --- | --- |
| `cmd/gra/main.go` | tea.NewProgram, Send, Run, WithAltScreen, WithMouseCellMotion, WithContext, ErrProgramKilled | - | - |
| `internal/tui/model.go` | Msg型定義(3 exported + 2 unexported) | - | help.Model, help.New |
| `internal/tui/update.go` | Init, Update, tea.Cmd, tea.Batch, tea.Quit, tea.Tick, tea.Msg, tea.WindowSizeMsg, tea.MouseMsg, tea.KeyMsg, Mouse/Key定数 | - | key.Matches, key.Binding |
| `internal/tui/view.go` | - | NewStyle, Color, Border, JoinHorizontal, JoinVertical, Width, Render | - |
| `internal/tui/watch.go` | tea.Cmd, tea.Msg | - | - |
| `internal/tui/openfile.go` | tea.Cmd, tea.Msg, tea.KeyMsg, Key定数 | NewStyle, Color, Style, StyleRunes, RoundedBorder | list.Model, list.Item, list.ItemDelegate, textinput.Model |
| `internal/tui/keys.go` | - | - | key.Binding, key.NewBinding, key.WithKeys, key.WithHelp, key.Matches, key.SetEnabled, help.KeyMap |
| `internal/tui/tab.go` | - | - | textarea.Model, textarea.New |
| `internal/tui/welcome.go` | - | NewStyle, Color, Width, Render | - |
| `internal/tui/display.go` | - | NewStyle, Width, Render | - |
| `internal/tui/highlight.go` | - | HasDarkBackground | - |
| `internal/tui/openfile_test.go` | - | - | list.Item |
