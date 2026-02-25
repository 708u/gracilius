# Layout Proposals: Alternatives (検討記録)

Proposal A (Unified Adaptive Layout) を採用。
以下は検討過程で比較した代替案の記録。

---

## Proposal B: Mode-Switched Full Layout

Top-level mode で画面全体のレイアウトが最適化される設計。
mode ごとに pane 構成が異なる。

### Mode: Browse (default)

```txt
 [Browse] [Review] [PR]
+--FileTree---+--Viewer------------------------------+
| src/        |  10 | func main() {                  |
|   main.go   |  11 |     srv := NewServer()          |
| * handler.go|  12 |     srv.Run()                   |
|   utils.go  |  13 | }                               |
|   auth/     |  14 |                                  |
|     login.go|  15 | func NewServer() *Server {       |
+-------------+--------------------------------------+
 1:Browse 2:Review 3:PR  j/k:Move /:Search Esc:Quit
```

### Mode: Review (Side-by-Side, full width)

```txt
 [Browse] [*Review*] [PR]
 [handler.go] [utils.go] [main.go]    <- file tabs
+--Old (before)------------------+--New (after)-----------------+
|  10 func Handle(r *Request) {  |  10 func Handle(r *Request) {|
|  11     return process(r)      |  11     if r == nil {         |
|                                |  12         return ErrNil     |
|                                |  13     }                     |
|  12     // ...                 |  14     resp := process(r)    |
+--------------------------------+-------------------------------+
 [a]ccept [r]eject [c]omment  n/N:File  u:Unified  Ctrl+b:Sidebar
 handler.go (+3, -1)  Review: 1/3 files
```

### Mode: Review (Unified)

```txt
 [Browse] [*Review*] [PR]
 [handler.go] [utils.go] [main.go]
+--Diff: handler.go--------------------------------------+
|  10   func Handle(r *Request) {                        |
|  11 - return process(r)                                |
|  11 + if r == nil {                                    |
|  12 +     return ErrNil                                |
|  13 + }                                                |
|  14   resp := process(r)                               |
+---------------------------------------------------------+
 [a]ccept [r]eject [c]omment  n/N:File  s:SbS  Ctrl+b:Sidebar
```

### Mode: Review (sidebar toggled on)

```txt
 [Browse] [*Review*] [PR]
+--Changed----+--Old (before)--------+--New (after)-------+
| > handler.go|  10 func Handle() {  |  10 func Handle()  |
|     +3, -1  |  11   return proc()  |  11   if r == nil { |
|   utils.go  |                      |  12     return Err  |
|     +1, -0  |  12   // ...         |  13   }              |
|   main.go   |                      |  14   resp := proc()|
|     accepted|                      |                     |
+-------------+----------------------+---------------------+
 [a]ccept [r]eject  Ctrl+b:Hide  Review: 1/3
```

### Mode: PR

```txt
 [Browse] [Review] [*PR*]
+---PR #42: Fix auth handling---------------------------+
| Author: @alice  Branch: fix/auth -> main              |
| Status: Open  CI: passing  Reviews: 1/2 approved     |
+-------------------------------------------------------+
|                                                       |
| ## Description                                        |
| Fix null pointer in auth handler when request body    |
| is empty.                                             |
|                                                       |
+--Changed Files----------------------------------------+
|  handler.go (+3, -1)   utils.go (+1, -0)             |
+-------------------------------------------------------+
+--Conversation-----------------------------------------+
|  @reviewer: LGTM but add nil check test              |
|    @alice: Added in abc1234                           |
|  @ci: All checks passed                              |
+-------------------------------------------------------+
 Enter:Open diff  c:Comment  /:Search
```

### Comment Overlay (in Review mode)

```txt
+--Old (before)------------------+--New (after)---------+
|  10 func Handle(r *Request) {  |  10 func Handle() {  |
|  11     return process(r)      |  11     if r == nil { |
+----- Comment (line 11) --------------------------------+
| @you: Consider error wrapping here                     |
+--- [c]lose [r]eply -------------------Esc:Close -------+
|                                |  12     return Err    |
+--------------------------------+-----------------------+
```

### Search Overlay (any mode, Ctrl+p)

```txt
+--Search File-----------------------------------------+
| > hand                                       [3/42]  |
|                                                      |
| src/handler.go                                       |
| src/handler_test.go                                  |
| pkg/error_handler.go                                 |
+------------------------------------------------------+
 Enter:Open  Esc:Cancel
```

### Proposal B の構造

```txt
+--Mode Bar (Browse | Review | PR)-----+
|                                      |
+--Mode-specific layout---------------+
|  Browse: [FileTree | Viewer]         |
|  Review: [Old | New] or [Unified]    |
|          + toggleable sidebar        |
|          + file tabs within mode     |
|  PR:     [Full-width info sections]  |
+--------------------------------------+
|  Context-aware Key Hints + Status    |
+--------------------------------------+
```

### Proposal B の特徴

- 3つの top-level mode: Browse, Review, PR
- 各 mode のレイアウトが目的に完全最適化
- Review mode 内に file tabs (diff 対象の切り替え)
- Review mode: side-by-side (default) / unified toggle
- Sidebar (changed files) は Review mode で toggle 可能
- PR mode は全幅で情報表示 (description, files, conversation)
- Mode 間は number keys (1-3) で即座に切り替え

### Proposal B の利点

- Mode ごとのレイアウトが最適化され、情報密度が高い
- Review mode は side-by-side diff に全幅を使える
- PR mode は複雑な情報 (description, checks, conversation)
  を1画面に整理できる
- Claude Code が diff を push した時、
  自動で Review mode に切り替わるとスムーズ

### Proposal B の欠点

- Mode 切り替え時にコンテキストが分断される
  (Browse で見ていたファイルと Review の diff が離れる)
- 実装の複雑度が高い (mode ごとに layout logic が別)
- mode + file tabs の二重ナビゲーションが認知コスト増

### 不採用理由

File view と Diff view の2つだけで全ユースケースを
カバーできるため、mode ごとに layout を分ける
実装コストに見合わない。

---

## Proposal C: Stacked Navigation + Dynamic Content

lazygit の stacked panel を採用し、左の navigation zone が
context を常に提示する設計。

### Default State (Files focused)

```txt
+--Nav--------+--Content-----------------------------+
| [*Files*]   |  10 | func main() {                  |
|  src/       |  11 |     srv := NewServer()          |
|    main.go  |  12 |     srv.Run()                   |
|  * handler  |  13 | }                               |
|    utils.go |  14 |                                  |
|-------------|  15 | func NewServer() *Server {       |
| [Diffs] (3) |  16 |     return &Server{}              |
|  handler.go |                                       |
|  utils.go   |                                       |
|-------------|                                       |
| [Comments]  |                                       |
|  (none)     |                                       |
+-------------+--------------------------------------+
 j/k:Move Tab:Section /:Search  Connected:18765
```

### Diff Focus: Side-by-Side (sidebar hidden)

```txt
+--Old (before)------------------+--New (after)-----------------+
|  10 func Handle(r *Request) {  |  10 func Handle(r *Request) {|
|  11     return process(r)      |  11     if r == nil {         |
|                                |  12         return ErrNil     |
|                                |  13     }                     |
|  12     // ...                 |  14     resp := process(r)    |
|  13 }                          |  15 }                         |
+--------------------------------+-------------------------------+
 [a]ccept [r]eject  n/N:File  u:Unified  Ctrl+b:Nav panel
 handler.go (+3, -1)  Diff 1/3
```

### Diff Focus: Side-by-Side (Nav panel visible)

```txt
+--Nav--------+--Old (before)--------+--New (after)-------+
| [Files]     |  10 func Handle() {  |  10 func Handle()  |
|  (collapsed)|  11   return proc()  |  11   if r == nil { |
|-------------|                      |  12     return Err  |
| [*Diffs*] 3 |  12   // ...         |  13   }              |
| > handler.go|                      |  14   resp := proc()|
|   utils.go  |                      |                     |
|   main.go   |                      |                     |
|-------------|                      |                     |
| [Comments]  |                      |                     |
|  handler:11 |                      |                     |
+-------------+----------------------+---------------------+
 [a]ccept [r]eject  Ctrl+b:Hide Nav  u:Unified
```

### Diff Focus: Unified (Nav panel visible)

```txt
+--Nav--------+--Diff: handler.go--------------------+
| [Files]     |  10   func Handle(r *Request) {      |
|  (collapsed)|  11 - return process(r)              |
|-------------|  11 + if r == nil {                   |
| [*Diffs*] 3 |  12 +     return ErrNil              |
| > handler.go|  13 + }                               |
|   utils.go  |  14   resp := process(r)              |
|   main.go   |  15   return resp                     |
|-------------|                                       |
| [Comments]  |                                       |
|  handler:11 |                                       |
+-------------+--------------------------------------+
 [a]ccept [r]eject  s:SbS  Ctrl+b:Hide Nav
```

### Comment Inline Expansion

```txt
+--Nav--------+--Content-----------------------------+
| [Files]     |  10 | func main() {                  |
|  (collapsed)|  11 |     srv := NewServer()     [2] |
|-------------|  ---- Comment @you ------------------  |
| [Diffs] (3) |  | NewServer should accept opts     |  |
|  handler.go |  | > Consider context parameter     |  |
|-------------|  ------------------------------------  |
| [*Comments*]|  12 |     srv.Run()                   |
| > main:11   |  13 | }                               |
|   handler:25|                                       |
+-------------+--------------------------------------+
 c:New  ]/[:Jump  r:Reply  d:Delete
```

### PR Info (p key toggles content)

```txt
+--Nav--------+--PR #42-----------------------------+
| [Files]     | Fix auth handling                    |
|  (collapsed)| @alice  fix/auth -> main             |
|-------------|                                      |
| [Diffs] (3) | Status: Open  CI: passing            |
|  handler.go | Reviews: 1 approved, 1 pending       |
|  utils.go   |                                      |
|-------------|  ## Description                      |
| [Comments]  | Fix null pointer in auth handler...  |
|  (2 threads)|                                      |
+-------------+--------------------------------------+
 Enter:View diff  c:Comment  Esc:Back
```

### Proposal C の Search Overlay (Ctrl+p)

```txt
+--Nav--------+--Content-----------------------------+
|             |                                      |
|  +--Search File----------------------------------+ |
|  | > hand                                [3/42]  | |
|  |                                               | |
|  | src/handler.go                                | |
|  | src/handler_test.go                           | |
|  | pkg/error_handler.go                          | |
|  +-----------------------------------------------+ |
|             |                                      |
+-------------+--------------------------------------+
```

### Proposal C の構造

```txt
+--Nav Panel (toggle with Ctrl+b)--+--Content--------+
| [Files]  - file tree / collapsed |                  |
| ------                           |  File Viewer     |
| [Diffs]  - changed files / count |  or SbS Diff     |
| ------                           |  or Unified Diff |
| [Comments] - thread list / count |  or PR Info      |
+----------------------------------+-----------------+
| Key Hints + Status                                 |
+----------------------------------------------------+
```

Nav Panel の各 section は expand/collapse:

- Focus された section が expand
- 他の section は1行サマリー (collapsed)
- Tab key で section 間を移動

### Proposal C の特徴

- 左 Nav に Files, Diffs, Comments の3 section
- 各 section は expand/collapse で情報密度を制御
- Content pane は Nav の選択で動的に変化
- Nav panel 全体を Ctrl+b で toggle (diff 時に全幅確保)
- Side-by-side / unified は `u`/`s` で toggle
- PR info は `p` key で content を切り替え

### Proposal C の利点

- 全ての情報 (files, diffs, comments) が Nav で一覧可能
- lazygit で実証済みの pattern で馴染みがある
- Sidebar toggle で side-by-side diff の横幅確保
- Section の focus/collapse で自然な情報の濃淡
- read-only なので Nav が narrow でも十分

### Proposal C の欠点

- 3 section の expand/collapse + focus 管理が複雑
- Terminal が小さいと section が潰れる (24行で3 section)
- lazygit は write 操作向けで、
  read-only では section の存在意義が薄まる可能性
- PR 情報は Nav の section では表示しきれない

### 不採用理由

Files / Diffs / Comments の3セクションは
read-only ビューアには過剰。
sidebar の中身を view 種別で切り替えるだけで済む。

---

## Comparison

| Aspect | A: Unified Adaptive | B: Mode-Switched | C: Stacked Nav |
| --- | --- | --- | --- |
| Layout base | sidebar + content | mode tabs | stacked panels |
| Impl cost | Low | High | Medium |
| Distance from current | Minimal | Large | Medium |
| SbS diff width | Full (sidebar off) | Full (mode dedicated) | Full (Nav off) |
| Context preservation | High (tabs) | Low (mode switch) | High (sections) |
| Small terminal (80x24) | Good | Good | Poor |
| PR diff 統合 | Diff view に統合 | 専用 mode | Content に統合 |
| Information density | Medium | High per mode | High overall |
| Learning curve | Low | Medium | Medium |
| Navigation model | tabs + sidebar | modes + tabs | sections + content |
| Inspiration | gh-dash sidebar | gitui tabs | lazygit panels |
