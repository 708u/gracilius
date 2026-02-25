# gracilius TUI Layout Proposals

## Background

gracilius は Claude Code が生成したコードをレビューする
read-only TUI ビューアである。現在の実装は
`[FileTree | Editor]` の2ペインレイアウトだが、
将来的な機能拡張を見据えてレイアウトを再設計する。

### 将来の機能要件

1. Diff viewer
   - openDiff: Claude Code からの差分表示 (MCP blocking)
   - git diff: ローカルの未コミット変更やブランチ間比較
2. GitHub PR viewer (PR diff, description, comments)
3. Local comment (MCP 経由の行レベルアノテーション)
4. File search (fuzzy finder)
5. Multiple tabs (ファイル・diff の切り替え)
6. Syntax highlighting (chroma v2 で実装済み)

### 確定した設計判断

- Diff は side-by-side (Old | New) と unified の両方対応
- Side-by-side diff 時、デフォルトは sidebar 非表示の全幅
- Ctrl+b 等で sidebar の表示/非表示を toggle 可能
- Read-only: 書き込み不要
- openDiff (MCP) は blocking。到着時に即座に
  diff view を表示しフォーカスする。
  accept/reject 後は diff view を閉じて元の view に戻る
- MCP の役割は local review 状態の読み取り・変更のみ。
  PR データの取得等は MCP の範囲外

### 設計上の制約

- MCP: Claude Code が diff を push する受動的ワークフロー
  (openDiff) と、gracilius が能動的にデータを取得する
  ワークフロー (git diff, GitHub PR) の両方が存在
- Terminal size: 最小 80x24、一般的 120x40+
- Keyboard-first, mouse supported
- bubbletea + lipgloss (Go)

### 調査から得た知見

@docs/research-file-managers.md
@docs/research-code-review-tools.md
@docs/ux-review.md

ファイルマネージャ調査 (yazi, ranger, lf, nnn, broot):

- Miller columns は directory traversal 向き、
  code review には tree + content の2ペインが適切
- Ratio-based sizing (lf の `1:3` 等) は柔軟性が高い
- broot の search-driven tree は大規模コードベースに有効
- read-only では file operation UI が不要で
  content 表示領域を最大化できる

Code review ツール調査 (lazygit, tig, gh-dash, gitui):

- lazygit の stacked panel + main content は情報密度が高い
- gh-dash の tab + sidebar toggle は bubbletea で実証済み
- context-aware key hints (bottom bar) は学習コスト低減に有効
- blocking diff の accept/reject は2ボタンで表現
- delta の side-by-side diff: 行番号 + 色分け + word-level
  highlighting が読みやすさに貢献

---

## Proposal A: Unified Adaptive Layout

2ペインを基本とし、Content pane の中身が
context に応じて切り替わる設計。
sidebar は常に toggle 可能で、
view の種類に応じて中身が決まる:

- File view -> sidebar = FileTree
- Diff view -> sidebar = Changed files list

### Files View (default)

```txt
 [main.go] [handler.go] [PR #42]
+--FileTree---+--Viewer------------------------------+
| src/        |  10 | func main() {                  |
|   main.go   |  11 |     srv := NewServer()          |
| * handler.go|  12 |     srv.Run()                   |
|   utils.go  |  13 | }                               |
|   auth/     |  14 |                                  |
|     login.go|  15 | func NewServer() *Server {       |
+-------------+--------------------------------------+
 [Tree] j/k:Move Tab:Switch /:Search Esc:Quit
 src/main.go  Connected:18765
```

### Diff View: Side-by-Side (default, sidebar hidden)

```txt
 [main.go] [*diff: handler.go*] [PR #42]
+--Old (before)------------------+--New (after)-----------------+
|  10 func Handle(r *Request) {  |  10 func Handle(r *Request) {|
|  11     return process(r)      |  11     if r == nil {         |
|                                |  12         return ErrNil     |
|                                |  13     }                     |
|  12     // ...                 |  14     resp := process(r)    |
+--------------------------------+-------------------------------+
 [a]ccept [r]eject [c]omment  n/N:File  u:Unified  Ctrl+b:Sidebar
 handler.go (+3, -1)  Diff 1/3
```

### Diff View: Side-by-Side (sidebar toggled on)

```txt
 [main.go] [*diff: handler.go*]
+--Changes---+--Old (before)------+--New (after)-------+
| > handler  |  10 func Handle() {|  10 func Handle()  |
|   utils    |  11   return proc()|  11   if r == nil { |
|   main     |                    |  12     return Err  |
|            |  12   // ...       |  13   }              |
|            |                    |  14   resp := proc()|
+------------+--------------------+---------------------+
 [a]ccept [r]eject  Ctrl+b:Hide sidebar  u:Unified
```

### Diff View: Unified (sidebar hidden)

```txt
 [main.go] [*diff: handler.go*] [PR #42]
+--Diff: handler.go--------------------------------------+
|  10   func Handle(r *Request) {                        |
|  11 - return process(r)                                |
|  11 + if r == nil {                                    |
|  12 +     return ErrNil                                |
|  13 + }                                                |
|  14   resp := process(r)                               |
|  15   return resp                                      |
+---------------------------------------------------------+
 [a]ccept [r]eject [c]omment  n/N:File  s:SbS  Ctrl+b:Sidebar
 handler.go (+3, -1)  Diff 1/3
```

### Comment Overlay

```txt
+--Old (before)------------------+--New (after)---------+
|  10 func Handle(r *Request) {  |  10 func Handle() {  |
|  11     return process(r)      |  11     if r == nil { |
+----- Comment (line 11) --------------------------------+
| @you: Consider error wrapping here                     |
| @you: Also add context parameter                       |
+--- [c]lose [r]eply [d]elete --------------Esc:Close ---+
|                                |  12     return Err    |
+--------------------------------+-----------------------+
```

### Search Overlay (Ctrl+p)

```txt
+--Old (before)------------------+--New (after)---------+
|                                                        |
|  +--Search File----------------------------------+     |
|  | > hand                                [3/42]  |     |
|  |                                               |     |
|  | src/handler.go                                |     |
|  | src/handler_test.go                           |     |
|  | pkg/error_handler.go                          |     |
|  +-----------------------------------------------+     |
|                                                        |
+--------------------------------------------------------+
```

### Diff View: openDiff (MCP, accept/reject あり)

openDiff は Claude Code からの blocking tool call。
到着時に即座に diff view を表示し、
accept/reject 後は元の view に戻る。

```txt
 [main.go] [*diff: handler.go*]
+--Old (before)------------------+--New (after)-----------------+
|  10 func Handle(r *Request) {  |  10 func Handle(r *Request) {|
|  11     return process(r)      |  11     if r == nil {         |
|                                |  12         return ErrNil     |
|                                |  13     }                     |
|  12     // ...                 |  14     resp := process(r)    |
+--------------------------------+-------------------------------+
 [a]ccept [r]eject [c]omment  u:Unified  Ctrl+b:Sidebar
 handler.go (+3, -1)  Diff 1/1
```

### Diff View: git diff (local, 複数ファイル)

ユーザーが能動的に実行する。
未コミット変更やブランチ間比較で複数ファイルの
diff を表示。accept/reject なし。
選択した diff を Claude Code に共有可能。

```txt
 [main.go] [*diff: working changes*]
+--Changes---+--Old (before)------+--New (after)-------+
| > handler  |  10 func Handle() {|  10 func Handle()  |
|   utils    |  11   return proc()|  11   if r == nil { |
|   main     |                    |  12     return Err  |
|            |  12   // ...       |  13   }              |
|            |                    |  14   resp := proc()|
+------------+--------------------+---------------------+
 [c]omment  n/N:File  u:Unified  Ctrl+b:Hide sidebar
 handler.go (+3, -1)  Diff 1/3
```

### Diff View: PR diff (GitHub API)

ユーザーが TUI 内から能動的に PR を開く。
gracilius が GitHub API からデータを取得。
diff renderer は共通、accept/reject なし。
PR 固有の情報は diff header に付帯メタデータとして表示。

```txt
 [main.go] [*diff: handler.go (PR #42)*]
 PR #42: Fix auth handling  @alice  Open  CI:pass
+--Old (before)------------------+--New (after)-----------------+
|  10 func Handle(r *Request) {  |  10 func Handle(r *Request) {|
|  11     return process(r)      |  11     if r == nil {         |
|                                |  12         return ErrNil     |
|                                |  13     }                     |
|  12     // ...                 |  14     resp := process(r)    |
+--------------------------------+-------------------------------+
 [c]omment  n/N:File  u:Unified  Ctrl+b:Sidebar  i:PR info
 handler.go (+3, -1)  Diff 1/3  Reviews: 1/2
```

PR 情報の詳細表示が必要な場合は `i` で info overlay:

```txt
+--PR #42 Info-------------------------------------------+
| Fix auth handling                                      |
| Author: @alice  Branch: fix/auth -> main               |
| Status: Open  CI: passing  Reviews: 1/2 approved       |
|                                                        |
| Fix null pointer in auth handler when request          |
| body is empty.                                         |
|                                                        |
| Changed Files:                                         |
|   handler.go (+3, -1)                                  |
|   utils.go (+1, -0)                                    |
+------Esc:Close-----------------------------------------+
```

### Proposal A の構造

```txt
+--Tab Bar-----------------------------------------+
|                                                  |
+--Sidebar (toggle)--+--Content Pane--------------+
|  FileTree          |  File Viewer               |
|  or                |  or Side-by-Side Diff      |
|  Changed Files     |  or Unified Diff           |
+--------------------+----------------------------+
|  Context-aware Key Hints + Status               |
+-------------------------------------------------+
```

View は2つ:

- File view -> File Viewer (syntax highlighted)
  - sidebar = FileTree
- Diff view -> Side-by-Side or Unified diff
  - sidebar = Changed files list (1件でも複数でも)
  - diff renderer は全ソースで共通

Diff view のデータソースは3つ:

```txt
ソース             トリガー              accept/reject
──────────────────────────────────────────────────────
openDiff (MCP)     Claude Code が push   あり (blocking)
git diff (local)   ユーザーが実行       なし
PR diff (GitHub)   ユーザーが実行       なし
```

accept/reject の有無だけがソースごとに異なる。
PR diff の場合は diff header に PR メタデータを表示。

### Proposal A の特徴

- 全ての view が同じ骨格 (sidebar + content) を共有
- Sidebar は Ctrl+b で toggle、記憶される
- Sidebar の中身は view の種類で決まる
  (File view = FileTree, Diff view = Changed files list)
- Tab bar でファイル/diff を切り替え
- Diff は side-by-side (デフォルト) と
  unified を `u`/`s` で切り替え
- Diff renderer は全ソース (openDiff, git diff, PR)
  で共通。accept/reject の有無のみ異なる
- Overlay: comment, search, help, PR info
- Bottom bar: context-aware key hints

### Proposal A の利点

- 構造がシンプルで一貫性がある
- Sidebar toggle により side-by-side diff の横幅を確保
- 現在の実装からの evolution path が明確
- Tab 追加で機能拡張が容易
- Sidebar の切り替えルールが「File view か Diff view か」
  の1条件のみでシンプル
- Diff renderer を1つ実装すれば
  openDiff / git diff / PR diff 全てに対応

### Proposal A の欠点

- 多数の tab が開くと tab bar が溢れる
- PR の情報量が sidebar + overlay では
  不足する可能性 (info overlay で対応)

---

## Recommendation

### Primary: Proposal A (Unified Adaptive Layout)

Proposal A を推奨する。

#### 中核設計: 2つの View と統一 Diff Renderer

gracilius の本質は diff viewer である。
ファイルブラウジングは diff viewer への導線であり、
PR viewer は diff のデータソース違いに過ぎない。

View は2つ:

```txt
+-- File view --+
| sidebar:      |  FileTree
| content:      |  File Viewer (syntax highlighted)
+---------------+

+-- Diff view --+
| sidebar:      |  Changed files list
| content:      |  Side-by-Side or Unified diff
| source:       |  openDiff / git diff / PR diff
| accept/reject:|  openDiff のみ
+---------------+
```

Diff view のデータソースは3つだが、
diff renderer は1つの実装で全てに対応:

```txt
ソース             トリガー              accept/reject
──────────────────────────────────────────────────────
openDiff (MCP)     Claude Code が push   あり (blocking)
git diff (local)   ユーザーが実行       なし
PR diff (GitHub)   ユーザーが実行       なし
```

ソースの違いは以下のみで吸収:

- openDiff: accept/reject キーが有効。
  1件ずつ blocking で到着し、応答後に
  diff view を閉じて元の view に戻る
- git diff: ユーザーが TUI 内で実行。
  複数ファイルの Changed files list を表示。
  選択した diff を Claude Code に共有可能
- PR diff: ユーザーが TUI 内で PR を開く。
  gracilius が GitHub API からデータ取得。
  diff header に PR メタデータを表示。
  `i` で PR info overlay

この統一により:

- Diff renderer の実装が1つで済む
- Comment system が全 diff で再利用可能
- Sidebar (Changed files list) が全ソースで共通
- Sidebar の切り替えルールが
  「File view か Diff view か」の1条件のみ

#### MCP の役割

MCP の責務は local review 状態の読み取り・変更のみ:

- Claude Code -> gracilius: openDiff (diff を push)
- gracilius -> Claude Code: accept/reject, comment 共有

PR データの取得は MCP の範囲外。
gracilius が GitHub API クライアントを持ち、
ユーザー操作で能動的にデータを取得する。

#### 推奨理由

1. **Diff 中心設計との整合**

   View が File と Diff の2つだけになり、
   layout logic がシンプルになる。
   PR viewer 専用のレイアウトが不要。

2. **Side-by-side diff との相性**

   Sidebar toggle で全幅 side-by-side を確保。
   全ての diff ソースで同じ toggle 操作。

3. **段階的な実装が可能**

   現在の2ペイン -> sidebar toggle -> diff view と
   段階的に拡張できる。git diff 対応で
   Changed files list の基盤を作り、
   PR 対応はメタデータ表示の追加のみ。

4. **Read-only の利点の最大化**

   Editor pane が不要なため content 領域が広い。
   Diff viewer に全幅を使える。

5. **小さい terminal での動作**

   80x24 でも sidebar off で
   side-by-side diff が機能。

### 実装パス

#### Phase 1: Tab System

- Tab bar を header に追加
- Tab 型: FileTab, DiffTab
- Tab 切り替え: `gt`/`gT` or Ctrl+1,2,3...
- Tab close: `q` or `Ctrl+w`

#### Phase 2: Sidebar Toggle + Changed Files List

- Ctrl+b で sidebar の表示/非表示
- Sidebar の中身は view で決まる:
  - File view: FileTree
  - Diff view: Changed files list
- Changed files list は diff ソースに依存しない
  共通コンポーネント (1件でも複数でも表示)

#### Phase 3: Diff Renderer (Side-by-Side)

- 全ソース共通の diff renderer
- Old | New の2カラム表示
- 行番号同期スクロール
- Word-level diff highlighting
- `u` で unified に切り替え
- openDiff (MCP) でまず動作確認

#### Phase 4: Diff Renderer (Unified)

- `s` で side-by-side に切り替え
- 現在の preview diff 表示をベースに拡張

#### Phase 5: Local Git Diff

- TUI 内から git diff を実行
  (未コミット変更、ブランチ間比較)
- 複数ファイルの Changed files list 表示
- Diff renderer を再利用
- 選択した diff を Claude Code に共有 (MCP)

#### Phase 6: Comment System

- Line-level comment indicator `[N]`
- Comment を bottom area に表示
  (diff コードの可視性を確保)
- Thread navigation (`]`/`[`)
- MCP 経由で Claude Code に送信

#### Phase 7: Search

- Ctrl+p で fuzzy search overlay
- Filter-as-you-type
- Enter で file/diff tab を開く

#### Phase 8: PR Diff Source

- GitHub API クライアントの実装
- TUI 内から PR を開くユーザー操作
- Diff header に PR メタデータを表示
  (title, author, status, CI, reviews)
- `i` で PR info overlay
  (description, conversation)
- PR comment と local comment の統合表示

### Key Bindings (proposed)

| Key | Context | Action |
| --- | --- | --- |
| `j/k` | All | Cursor up/down |
| `h/l` | FileTree | Navigate tree |
| `Tab` | All | Switch focus (sidebar/content) |
| `gt/gT` | All | Next/prev tab |
| `Ctrl+b` | All | Toggle sidebar |
| `Enter` | FileTree | Open file tab |
| `Enter` | Changed files | Open diff in content |
| `a` | Diff (openDiff) | Accept diff |
| `r` | Diff (openDiff) | Reject diff |
| `c` | Diff/File | New comment |
| `u` | Diff (SbS) | Switch to unified |
| `s` | Diff (Unified) | Switch to side-by-side |
| `n/N` | Diff | Next/prev changed file |
| `}/\{` | Diff | Next/prev hunk |
| `]/[` | All | Next/prev comment thread |
| `Ctrl+p` | All | Fuzzy search overlay |
| `i` | Diff (PR) | PR info overlay |
| `?` | All | Help overlay |
| `q` | All | Close tab or quit |
| `Esc` | Overlay | Close overlay |

### Bottom Bar Design

Context-aware key hints (lazygit / gitui pattern):

```txt
 [a]ccept [r]eject [c]omment  n/N:File  u:Unified
```

表示する hint は現在の focus と context で決まる:

- FileTree focus: `Enter:Open j/k:Move /:Filter`
- File viewer: `c:Comment v:Select /:Search`
- Diff (SbS, openDiff):
  `a:Accept r:Reject c:Comment u:Unified`
- Diff (SbS, git diff/PR):
  `c:Comment n/N:File u:Unified`
- Diff (Unified, openDiff):
  `a:Accept r:Reject c:Comment s:SbS`
- Diff (Unified, git diff/PR):
  `c:Comment n/N:File s:SbS`
- Diff (PR source): 上記に加え `i:PR info`
- Comment overlay: `r:Reply d:Delete Esc:Close`

### Status Bar Design

Bottom bar の右側に status 情報:

```txt
 [key hints]                   handler.go (+3,-1) 1/3
```

表示項目:

- 現在のファイル名
- Diff stats (追加/削除行数)
- Changed files progress (N/M files)
- Connection status (port, connected/disconnected)
- PR source の場合: PR number と review status
