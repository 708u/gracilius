#!/bin/bash

# CLAUDECODE環境変数のunset
unset CLAUDECODE

# ~/.claude.jsonへのtrust設定追加
WORKTREE_PATH="$(pwd)"
jq --arg path "$WORKTREE_PATH" '
  .projects //= {} |
  .projects[$path] = ((.projects[$path] // {}) + {
    "hasTrustDialogAccepted": true,
    "hasTrustDialogHooksAccepted": true,
    "projectOnboardingSeenCount": 1,
    "hasClaudeMdExternalIncludesApproved": true,
    "hasClaudeMdExternalIncludesWarningShown": true,
    "hasCompletedProjectOnboarding": true
  })
' ~/.claude.json > ~/.claude.json.tmp && mv ~/.claude.json.tmp ~/.claude.json

claude \
  --permission-mode acceptEdits \
"以下の計画に従って Bubbletea v2 移行 Phase 1 (破壊的変更対応) を実装してください。
ビルド通過と既存動作の維持が目標です。新機能の活用は含みません。

編集対象ファイル:
@go.mod
@cmd/gra/main.go
@internal/tui/update.go
@internal/tui/view.go
@internal/tui/openfile.go
@internal/tui/highlight.go
@internal/tui/model.go
@internal/tui/keys.go
@internal/tui/tab.go
@internal/tui/watch.go
@internal/tui/welcome.go
@internal/tui/display.go
@internal/tui/openfile_test.go

参考ドキュメント (docs/ 配下に調査レポートあり):
@docs/bubbletea-v2-breaking-changes.md
@docs/bubbletea-v2-new-features.md
@docs/bubbletea-usage-analysis.md

## 実施順序

依存関係に基づき以下の順序で実施する。

### Step 1: go.mod 更新

go.mod の require を書き換え。

\`\`\`go
// 削除
github.com/charmbracelet/bubbles v1.0.0
github.com/charmbracelet/bubbletea v1.3.10
github.com/charmbracelet/lipgloss v1.1.0

// 追加
charm.land/bubbles/v2 <latest>
charm.land/bubbletea/v2 <latest>
charm.land/lipgloss/v2 <latest>
\`\`\`

github.com/charmbracelet/x/ansi は path 変更なし。
indirect 依存は go mod tidy で解決。

### Step 2: import path のみ変更 (API変更なし)

以下6ファイルは import path 書き換えのみ。

| ファイル | 変更 |
| --- | --- |
| internal/tui/display.go:6 | lipgloss |
| internal/tui/welcome.go:6 | lipgloss |
| internal/tui/keys.go:4-5 | bubbles/help, bubbles/key |
| internal/tui/watch.go:7 | bubbletea |
| internal/tui/tab.go:6 | bubbles/textarea |
| internal/tui/openfile_test.go:8 | bubbles/list |

import path パターン:

\`\`\`txt
github.com/charmbracelet/bubbletea  -> charm.land/bubbletea/v2
github.com/charmbracelet/lipgloss   -> charm.land/lipgloss/v2
github.com/charmbracelet/bubbles/X  -> charm.land/bubbles/v2/X
\`\`\`

### Step 3: model.go

internal/tui/model.go:7

import path 変更のみ (bubbles/help)。
help.New(), help.Model は v2 で存続。

### Step 4: view.go -- View() 戻り値型変更

internal/tui/view.go

#### 4a. import 追加

\`\`\`go
tea \"charm.land/bubbletea/v2\"  // 追加 (View戻り値型)
\"charm.land/lipgloss/v2\"       // path変更
\"github.com/charmbracelet/x/ansi\"  // 変更なし
\`\`\`

#### 4b. View() シグネチャ変更

行32: func (m *Model) View() string ->
func (m *Model) View() tea.View

全ての return 箇所を tea.View に変更。
AltScreen / MouseMode を View フィールドで設定。

\`\`\`go
func (m *Model) View() tea.View {
    if m.err != nil {
        var v tea.View
        v.AltScreen = true
        v.MouseMode = tea.MouseModeCellMotion
        v.SetContent(fmt.Sprintf(
            \"Error: %v\n\nPress Ctrl+C to quit.\",
            m.err))
        return v
    }

    if m.width == 0 || m.height == 0 {
        var v tea.View
        v.AltScreen = true
        v.MouseMode = tea.MouseModeCellMotion
        return v
    }

    // ... 既存レイアウト構築ロジック (変更なし) ...

    var v tea.View
    v.AltScreen = true
    v.MouseMode = tea.MouseModeCellMotion
    if m.openFile.active {
        v.SetContent(m.openFile.overlay(
            base, m.width, m.height))
    } else {
        v.SetContent(base)
    }
    return v
}
\`\`\`

#### 4c. help.Width setter

行166: m.help.Width = m.width ->
m.help.SetWidth(m.width)

### Step 5: cmd/gra/main.go

#### 5a. import 変更

行15: bubbletea path 変更

#### 5b. Program Options 削除

行116-120:

\`\`\`go
// Before
p := tea.NewProgram(m,
    tea.WithAltScreen(),
    tea.WithMouseCellMotion(),
    tea.WithContext(ctx),
)

// After
p := tea.NewProgram(m,
    tea.WithContext(ctx),
)
\`\`\`

tea.WithAltScreen / tea.WithMouseCellMotion は
v2 で削除済み。View フィールドに移行 (Step 4b)。
tea.WithContext, p.Send, p.Run,
tea.ErrProgramKilled は v2 で存続。

### Step 6: update.go -- KeyMsg/MouseMsg 型分割

internal/tui/update.go -- 最大の変更量。

#### 6a. import 変更

行12-13: bubbletea, bubbles/key path 変更

#### 6b. openFile active 時の型フィルタ

行97-98:

\`\`\`go
// Before
case tea.KeyMsg, tea.MouseMsg, tea.WindowSizeMsg,

// After
case tea.KeyPressMsg, tea.MouseClickMsg,
    tea.MouseReleaseMsg, tea.MouseWheelMsg,
    tea.MouseMotionMsg, tea.WindowSizeMsg,
\`\`\`

#### 6c. MouseMsg 型分割 (行169-288)

元の case tea.MouseMsg: (119行) を
4つの case に分割する。

型マッピング:

| v1 | v2 |
| --- | --- |
| MouseButtonLeft + MouseActionPress | tea.MouseClickMsg + msg.Button == tea.MouseLeft |
| MouseActionMotion | tea.MouseMotionMsg |
| MouseActionRelease | tea.MouseReleaseMsg |
| MouseButtonWheelUp/Down | tea.MouseWheelMsg + msg.Button |

定数リネーム:

| v1 | v2 |
| --- | --- |
| tea.MouseButtonLeft | tea.MouseLeft |
| tea.MouseButtonWheelUp | tea.MouseWheelUp |
| tea.MouseButtonWheelDown | tea.MouseWheelDown |

削除されるもの:
tea.MouseActionPress, tea.MouseActionMotion,
tea.MouseActionRelease -- 型分割により不要。

座標アクセス: 具体型で msg.X, msg.Y 直接アクセス可。

分割後の構造:

\`\`\`go
case tea.MouseClickMsg:
    // openFile active時のクリック (行170-182)
    // ボーダーリサイズ開始 (行188-190)
    // ツリーペインクリック (行201-207)
    // エディタペインクリック (行239-249)

case tea.MouseMotionMsg:
    // リサイズ中ドラッグ (行192-194)
    // エディタペインドラッグ選択 (行252-259)

case tea.MouseReleaseMsg:
    // リサイズ終了 (行196-198)
    // エディタペインリリース (行260-274)

case tea.MouseWheelMsg:
    // スクロールアップ (行275-279)
    // スクロールダウン (行280-285)
\`\`\`

各 case の先頭で openFile.active チェックと
hasTab / len(t.lines) ガード、lo 計算を
必要に応じて行う。

#### 6d. KeyMsg -> KeyPressMsg

行289: case tea.KeyMsg: -> case tea.KeyPressMsg:

#### 6e. msg.Type -> msg.String()

行326:

\`\`\`go
// Before
if msg.Type == tea.KeyEnter && ...
// After
if msg.String() == \"enter\" && ...
\`\`\`

行359:

\`\`\`go
// Before
case msg.Type == tea.KeyEnter:
// After
case msg.String() == \"enter\":
\`\`\`

key.Matches(msg, ...) は v2 の tea.KeyPressMsg で
正常動作するため、他の箇所は変更不要。

### Step 7: openfile.go

internal/tui/openfile.go

#### 7a. import 変更

行10-14: bubbles/list, bubbles/textinput,
bubbletea, lipgloss の path 変更。
x/ansi は変更なし。

#### 7b. Cursor API 変更

行164:

\`\`\`go
// Before
return s.input.Cursor.BlinkCmd()
// After
return s.input.Cursor().Blink()
\`\`\`

#### 7c. KeyMsg -> KeyPressMsg + msg.Type -> msg.String()

行204-206:

\`\`\`go
// Before
case tea.KeyMsg:
    switch msg.Type {
    case tea.KeyUp, tea.KeyDown:

// After
case tea.KeyPressMsg:
    switch msg.String() {
    case \"up\", \"down\":
\`\`\`

#### 7d. textinput.Width setter

行327:

\`\`\`go
// Before
s.input.Width = g.innerW
// After
s.input.SetWidth(g.innerW)
\`\`\`

### Step 8: highlight.go

internal/tui/highlight.go

#### 8a. import 変更

行10: lipgloss path 変更。
\"os\" を追加。

#### 8b. HasDarkBackground シグネチャ変更

行62-68:

\`\`\`go
// Before
if lipgloss.HasDarkBackground() {
// After
if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
\`\`\`

v2 では I/O を明示的に渡す必要がある。
init() 時点では Bubbletea 未起動のため、
直接 stdin/stdout を渡す方式が適切。

### Step 9: go mod tidy + ビルド確認

\`\`\`bash
go mod tidy
go build -o gra ./cmd/gra/
\`\`\`

ビルドエラーが出た場合、以下を確認:

- ta.ShowLineNumbers -> ta.SetShowLineNumbers(false) ?
- ta.Prompt -> ta.SetPrompt(\"\") ?
- help.Width -> help.SetWidth() ?
- MouseWheelMsg.Button -> 方向フィールド ?

エラーが出たら公式のupgrade guideを参照して修正:
- https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md
- https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md

## 検証

1. go build -o gra ./cmd/gra/ -- コンパイル通過
2. go test ./... -- 全テスト通過

完了したら:
1. /simplify を呼び出してコードを整理
2. /commit-push-update-pr を呼び出してPRを作成・更新
3. /export-session を呼び出してセッション内容を保存
4. .twig-claude-prompt-feat-bubbletea-v2-upgrade.sh を .completed-twig-claude-prompt.sh にリネーム"
