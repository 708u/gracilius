# Layout Proposals 実装計画

## Context

gracilius は現在 FileTree | Editor の2ペインレイアウト。
`docs/layout-proposals.md` の Proposal A (Unified Adaptive Layout)
に基づき、タブシステム、サイドバートグル、diff renderer、
git diff、コメント、検索、PR diff の8フェーズを実装する。

現在の実装: ~1,760行 (internal/tui/ 内11ファイル)。
リファクタリング済み:

- diff.go 削除済み (未使用の diff/preview コード除去)
- preview 関連フィールド削除済み
  (previewLines, previewFilePath, previewOldLines,
  FilePreviewMsg, ClearPreviewMsg)
- highlight.go 追加 (chroma v2 によるシンタックスハイライト)
- keys.go 追加 (keyMap を独立ファイルに抽出)
- display.go 簡素化 (truncateString 等は
  lipgloss/ansi に置換済み)
- Model メソッドは全てポインタレシーバに統一済み
- openDiff callback は登録済みだがログ出力のみ
  (TUI へのメッセージ送信は未実装)

Model は単一構造体に全状態を保持、タブなし、
accept/reject 未実装、オーバーレイシステムなし。

## PR分割方針

各PRは400行以下の差分を目標とし、レビュー可能な単位に分割。
依存関係のないPRはエージェントチームで並列作業する。

## 進捗

- [ ] PR 1-1: Tab データモデル
- [ ] PR 1-2: Tab 操作とキーバインド
- [ ] PR 1-3: Tab Bar レンダリング
- [ ] PR 2-1: サイドバートグルと Changed Files List
- [ ] PR 3-1: Diff データモデル
- [ ] PR 3-2: Word-level Diff
- [ ] PR 3-3: Side-by-Side Renderer
- [ ] PR 3-4: Diff View 統合と openDiff accept/reject
- [ ] PR 4-1: Unified Renderer と Mode Toggle
- [ ] PR 5-1: Git パッケージ
- [ ] PR 5-2: Git Diff TUI 統合
- [ ] PR 6-0: Overlay System 基盤
- [ ] PR 6-1: Comment データモデル
- [ ] PR 6-2: Comment UI
- [ ] PR 6-3: MCP Comment 通知
- [ ] PR 7-1: Fuzzy Search Overlay
- [ ] PR 8-1: GitHub API クライアント
- [ ] PR 8-2: PR Diff 統合
- [ ] PR 8-3: PR Comment 統合

## Phase 1: Tab System

### PR 1-1: Tab データモデル

新規ファイル `internal/tui/tab.go` を作成。

```txt
変更ファイル:
  NEW  internal/tui/tab.go        (~130行)
  MOD  internal/tui/model.go      (per-tab フィールド抽出)
```

内容:

- `tabKind` 型 (`fileTab`, `diffTab`)
- `tab` 構造体 (filePath, lines, highlightedLines,
  cursor*, anchor*, selecting,
  scrollOffset, comments 等を Model から移動)
- `newFileTab()`, `newDiffTab()` コンストラクタ
- Model に `tabs []tab`, `activeTab int` 追加
- `activeTabState() *tab` アクセサ
- per-tab フィールドを Model から削除

### PR 1-2: Tab 操作とキーバインド

```txt
変更ファイル:
  MOD  internal/tui/update.go     (tab 切り替え/close)
  MOD  internal/tui/fileio.go     (loadFile を tab 対象に変更)
  MOD  internal/tui/notify.go     (activeTabState 経由に変更)
  MOD  internal/tui/keys.go       (tab 関連キーバインド追加)
  MOD  internal/tui/model.go      (OpenDiffMsg 型追加)
  MOD  cmd/gra/main.go            (callback で p.Send)
```

内容:

- `loadFile` を `loadFileIntoTab(t *tab, ...)` に変更
  - `resetEditorState` のロジックも tab 単位に移行
- update.go: 全ハンドラを `m.activeTabState()` 経由に移行
- FileTree の Enter/click でファイルタブ作成
  (同一ファイルは既存タブに切り替え)
- `OpenDiffMsg` (filePath, contents) を新規定義
  - openDiff callback から `p.Send(OpenDiffMsg{...})`
  - 受信時に DiffTab を作成
- `CloseDiffMsg` を新規定義
  - close_tab/closeAllDiffTabs callback から送信
  - 受信時に DiffTab を close
- キーバインド: `gt`/`gT` (タブ切替)、
  `q` (タブ close、最後のタブなら quit)
- `pendingKey` フィールド追加 (g+t/T の2キーシーケンス)

### PR 1-3: Tab Bar レンダリング

```txt
変更ファイル:
  MOD  internal/tui/view.go       (tab bar、per-tab 参照)
```

内容:

- `renderTabBar(width int) string` 追加
  - アクティブタブ: `[*name*]`、非アクティブ: `[name]`
  - 幅超過時は省略表示
- ヘッダーを tab bar に置換
- `renderEditor` 等の per-tab データ参照を移行
- フッターのキーヒントを context-aware に変更
- contentHeight 調整 (tab bar 行分)

## Phase 2: Sidebar Toggle

### PR 2-1: サイドバートグルと Changed Files List

```txt
変更ファイル:
  MOD  internal/tui/model.go      (sidebarVisible 追加)
  MOD  internal/tui/tab.go        (changedFileEntry 追加)
  MOD  internal/tui/update.go     (Ctrl+b ハンドリング)
  MOD  internal/tui/view.go       (トグルレイアウト、
                                   renderChangedFiles 追加)
```

内容:

- Model に `sidebarVisible bool` (default: true)
- `Ctrl+b` でトグル
- `sidebarVisible == false` 時は全幅をコンテンツに
- `changedFileEntry` 構造体 (path, name, additions, deletions)
- `renderChangedFiles(width, height int) []string`
- View() でアクティブタブの kind に応じて
  renderTree / renderChangedFiles を切り替え

## Phase 3: Side-by-Side Diff Renderer

### PR 3-1: Diff データモデル

```txt
変更ファイル:
  NEW  internal/tui/diffmodel.go       (~150行)
  NEW  internal/tui/diffmodel_test.go  (~100行)
```

内容:

- `diffRow` 構造体 (oldLineNum, newLineNum,
  oldText, newText, rowType)
- `diffRowType` (unchanged, modified, added, deleted)
- `diffHunk` (startIdx, endIdx)
- `diffState` (rows, hunks, stats)
- `buildDiffState(oldLines, newLines []string) *diffState`
  - `sergi/go-diff` の出力を side-by-side 対に変換
  - 連続する remove/add をペアリングして modified に
  - hunk 検出
- テスト: 行番号アライメント、hunk 検出、空 diff

### PR 3-2: Word-level Diff

```txt
変更ファイル:
  NEW  internal/tui/worddiff.go       (~80行)
  NEW  internal/tui/worddiff_test.go  (~60行)
```

内容:

- `wordSpan` 構造体 (text, op)
- `computeWordDiff(old, new string) ([]wordSpan, []wordSpan)`
  - 単語境界でトークン化
  - 単語単位の LCS で差分計算
- テスト: 変数名変更、インデント変更、空行

### PR 3-3: Side-by-Side Renderer

```txt
変更ファイル:
  NEW  internal/tui/diffrender.go       (~200行)
  NEW  internal/tui/diffrender_test.go  (~100行)
  MOD  internal/tui/model.go            (diffMode 等追加)
```

内容:

- Model に `diffMode`, `diffData *diffState`,
  `diffScrollOff int` 追加
- `renderSideBySide(state, width, height, scrollOff) []string`
  - Old | New の2カラム: `(width - 3) / 2` ずつ
  - 行番号 gutter + コード
  - modified 行: word-level diff でハイライト
  - added: 緑背景、deleted: 赤背景
  - filler 行 (対応なし側は空)
- ANSI カラー定数 (addBg, removeBg, wordAdd, wordRem)
- `ansi.Truncate`, `padRight` を再利用
- テスト: カラム幅計算、truncation

### PR 3-4: Diff View 統合と openDiff accept/reject

```txt
変更ファイル:
  MOD  internal/tui/update.go    (diff キーバインド)
  MOD  internal/tui/view.go      (renderEditor 内 diff dispatch)
  MOD  internal/tui/model.go     (DiffResponseMsg 追加)
  MOD  internal/protocol/handler.go (blocking 対応)
  MOD  internal/server/server.go    (accept/reject callback)
  MOD  cmd/gra/main.go              (callback wiring)
```

内容:

- `renderEditor` 内で diffTab の場合 diff renderer に dispatch
- `OpenDiffMsg` 受信時に `buildDiffState` 実行
- terminal width < 100 時は unified をデフォルトに
- キーバインド: `}` / `{` (hunk ナビゲーション)
- `a` (accept) / `r` (reject) キー (openDiff のみ)
- openDiff を blocking に変更:
  handler.go で response を保留、accept/reject 時に送信
- accept callback, reject callback を server に追加
- フッターに diff stats 表示 (+N, -M)

## Phase 4: Unified Diff Renderer

### PR 4-1: Unified Renderer と Mode Toggle

```txt
変更ファイル:
  MOD  internal/tui/diffrender.go      (unified renderer 追加)
  MOD  internal/tui/diffrender_test.go (テスト追加)
  MOD  internal/tui/update.go          (u/s トグル)
```

内容:

- `renderUnified(state, width, height, scrollOff) []string`
  - 単一カラム、old/new 両行番号表示
  - +/- プレフィックス、modified は old (-) + new (+) の2行
  - word-level diff ハイライト
- `u` キー: side-by-side -> unified
- `s` キー: unified -> side-by-side
- `diffState` は共通 (モード切替時に再計算不要)

## Phase 5: Local Git Diff

### PR 5-1: Git パッケージ

```txt
変更ファイル:
  NEW  internal/git/diff.go       (~150行)
  NEW  internal/git/diff_test.go  (~100行)
```

内容:

- `git` CLI を exec で実行 (go-git は使わない)
- `DiffWorkingTree(dir) ([]FileDiff, error)`
- `DiffStaged(dir) ([]FileDiff, error)`
- `DiffBranch(dir, base, head) ([]FileDiff, error)`
- `ParseUnifiedDiff(output) []FileDiff`
  - `FileDiff` (OldPath, NewPath, OldContent, NewContent)
- テスト: unified diff パース

### PR 5-2: Git Diff TUI 統合

```txt
変更ファイル:
  MOD  internal/tui/update.go  (git diff トリガー)
  MOD  internal/tui/tab.go     (changedFiles population)
  MOD  internal/tui/model.go   (git diff メッセージ型)
```

内容:

- キーバインド `gd` で unstaged diff を実行
- 結果を DiffTab として開く
- Changed files list にファイル一覧を表示
- n/N で次/前のファイルに移動
- accept/reject なし (git diff ソースなので)

## Phase 6: Comment System

### PR 6-0: Overlay System 基盤

```txt
変更ファイル:
  NEW  internal/tui/overlay.go   (~100行)
  MOD  internal/tui/model.go     (overlayMode 追加)
  MOD  internal/tui/view.go      (overlay レンダリング)
  MOD  internal/tui/update.go    (overlay 入力キャプチャ)
```

内容:

- `overlayMode` 型 (none, comment, search, prInfo, help)
- `renderOverlay(content []string, width, height int) string`
  - View() 最後に呼び出し、矩形領域を上書き
- overlay アクティブ時はキー入力を overlay が専有
- `Esc` で overlay close

### PR 6-1: Comment データモデル

```txt
変更ファイル:
  NEW  internal/tui/comment.go       (~100行)
  MOD  internal/tui/tab.go           (comments フィールド置換)
```

内容:

- `Comment` 構造体 (ID, FilePath, Line, Text,
  Author, ThreadID, Source)
- `CommentStore` (ファイル別、行別のインデックス)
- 既存の `map[int]string` を `CommentStore` に置換

### PR 6-2: Comment UI

```txt
変更ファイル:
  MOD  internal/tui/overlay.go   (comment overlay)
  MOD  internal/tui/view.go      (diff 行に [N] indicator)
  MOD  internal/tui/update.go    (c キー、]/[ ナビゲーション)
```

内容:

- diff 行に `[N]` (コメント数) インジケーター
- `c` キーで comment overlay 開く
- bottom area に現在行のコメント表示
- `]`/`[` で次/前のコメントスレッドに移動

### PR 6-3: MCP Comment 通知

```txt
変更ファイル:
  MOD  internal/tui/model.go        (MCPServer IF 拡張)
  MOD  internal/tui/notify.go       (NotifyComment 追加)
  MOD  internal/protocol/handler.go (comment 通知)
  MOD  internal/server/server.go    (comment 送信)
```

内容:

- `MCPServer` に `NotifyComment(filePath, line, text)` 追加
- 現在の `[Comment]` prefix hack を正式メソッドに置換
- server 経由で Claude Code にコメント送信

## Phase 7: Search

### PR 7-1: Fuzzy Search Overlay

```txt
変更ファイル:
  NEW  internal/tui/search.go       (~120行)
  MOD  internal/tui/overlay.go      (search overlay 統合)
  MOD  internal/tui/update.go       (Ctrl+p、入力処理)
```

内容:

- 簡易 fuzzy matcher (外部ライブラリ不要)
- `Ctrl+p` で search overlay 開く
- filter-as-you-type でファイル一覧を絞り込み
- `Enter` でファイルタブ or diff タブを開く
- `Esc` で close

## Phase 8: PR Diff Source

### PR 8-1: GitHub API クライアント

```txt
変更ファイル:
  NEW  internal/github/client.go      (~120行)
  NEW  internal/github/client_test.go (~60行)
```

内容:

- `google/go-github` を使用
- 認証: `gh auth token` 出力 or `GITHUB_TOKEN` 環境変数
- `GetPullRequest(owner, repo, number) (*PRData, error)`
- `GetPullRequestDiff(owner, repo, number) ([]FileDiff, error)`
  - `internal/git.FileDiff` と共通型を使用

### PR 8-2: PR Diff 統合

```txt
変更ファイル:
  MOD  internal/tui/update.go  (PR open トリガー)
  MOD  internal/tui/tab.go     (PR メタデータフィールド)
  MOD  internal/tui/view.go    (PR header 表示)
```

内容:

- TUI 内から PR を開くコマンド
- DiffTab に PR メタデータ (title, author, status, CI)
- diff header に PR 情報1行表示
- `i` キーで PR info overlay

### PR 8-3: PR Comment 統合

```txt
変更ファイル:
  MOD  internal/github/client.go (GetPRComments 追加)
  MOD  internal/tui/comment.go   (PR comment source 統合)
  MOD  internal/tui/view.go      (PR + local comment 表示)
```

内容:

- GitHub PR comment の取得
- Comment の `Source` フィールドで local/pr を区別
- 統合表示 (local comment と PR comment を並列)

## エージェントチーム並列化計画

3エージェントで並列作業する。
依存関係のないPRを異なるエージェントに割り当てる。

### Sprint 1: 基盤 (Phase 1-2)

```txt
Agent A          Agent B            Agent C
PR 1-1 (tab)    PR 3-1 (diffmodel)  PR 3-2 (worddiff)
PR 1-2 (tab op)
PR 1-3 (tab UI)
PR 2-1 (sidebar)
```

- PR 3-1, 3-2 は新規ファイルのみで Phase 1 と競合しない
- Agent A が Phase 1-2 の sequential な作業を担当

### Sprint 2: Diff Renderer (Phase 3-4)

```txt
Agent A          Agent B            Agent C
PR 3-3 (SbS)   PR 5-1 (git pkg)   PR 6-0 (overlay)
PR 3-4 (integ)                     PR 6-1 (comment model)
PR 4-1 (unified)
```

- PR 5-1, 6-0, 6-1 は新規パッケージ/ファイルで並列可
- Agent A が diff renderer の serial path を担当

### Sprint 3: Feature 統合 (Phase 5-7)

```txt
Agent A          Agent B            Agent C
PR 5-2 (git UI) PR 6-2 (comment UI) PR 7-1 (search)
                 PR 6-3 (MCP comment) PR 8-1 (github pkg)
```

### Sprint 4: PR 統合 (Phase 8)

```txt
Agent A          Agent B            Agent C
PR 8-2 (PR diff) PR 8-3 (PR comment)
```

## 依存関係グラフ

```txt
1-1 -> 1-2 -> 1-3 -> 2-1 -> 3-4 -> 4-1 -> 5-2
                                          -> 6-2

3-1 -> 3-3 -> 3-4
3-2 -> 3-3

5-1 -> 5-2
6-0 -> 6-2 -> 6-3
6-0 -> 7-1
6-1 -> 6-2
8-1 -> 8-2 -> 8-3
```

Critical path: 1-1 -> 1-2 -> 1-3 -> 2-1 -> 3-4 -> 5-2

## 検証方法

各PRで以下を確認:

1. `go test ./...` が全て pass
2. `go build -o gra ./cmd/gra/` が成功
3. `go run ./cmd/gra/` で TUI が起動し基本操作が動作
4. Phase 3-4 以降: Claude Code と接続し
   openDiff の送受信が正常動作
   (`CLAUDE_CODE_SSE_PORT=18765 claude`)
5. 各フェーズの新機能が proposals の ASCII mockup と
   一致することを目視確認

## 重要ファイル一覧

```txt
既存ファイル (変更対象):
internal/tui/model.go       全フェーズで変更 (状態追加)
internal/tui/update.go      全フェーズで変更 (キーバインド)
internal/tui/view.go        Phase 1-4, 6-8 で変更
internal/tui/keys.go        Phase 1-2 以降 (キー定義追加)
internal/tui/fileio.go      Phase 1 (tab 対応)
internal/tui/notify.go      Phase 1 (activeTabState 経由)
internal/tui/highlight.go   変更なし (参照のみ)
internal/tui/display.go     変更なし (padRight 再利用)
internal/protocol/handler.go  Phase 1-2, 3-4, 6 で変更
internal/server/server.go     Phase 1-2, 3-4, 6 で変更
cmd/gra/main.go              Phase 1-2, 3-4 で変更

新規ファイル:
internal/tui/tab.go         NEW (Phase 1)
internal/tui/diffmodel.go   NEW (Phase 3)
internal/tui/diffrender.go  NEW (Phase 3-4)
internal/tui/worddiff.go    NEW (Phase 3)
internal/tui/overlay.go     NEW (Phase 6)
internal/tui/comment.go     NEW (Phase 6)
internal/tui/search.go      NEW (Phase 7)
internal/git/diff.go        NEW (Phase 5)
internal/github/client.go   NEW (Phase 8)
```

## 設計判断

- **Tab 状態分離**: per-tab フィールドを `tab` 構造体に抽出、
  `activeTabState()` アクセサで既存コードへの影響を最小化
- **Git 統合**: `go-git` ではなく `git` CLI を exec
  (依存最小化、パフォーマンス、標準 diff 出力)
- **GitHub API**: `google/go-github` + `gh auth token` 認証
- **Diff renderer**: `sergi/go-diff` (既存依存) の出力を
  `buildDiffState` で side-by-side 対に変換。
  diff.go は削除済みのため Phase 3 で新規構築。
  renderer は `*diffState` を受け取る純関数
- **Overlay**: View() の最後で矩形上書き。
  Phase 6, 7, 8 で共有
- **openDiff blocking**: handler.go で response channel を保持、
  accept/reject 時に結果を送信
