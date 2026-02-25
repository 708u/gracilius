# Proposal A UX Review

Proposal A (Unified Adaptive Layout) に対する
frontend design / UX 観点からのレビュー。

## 評価の前提

gracilius の主要ワークフローは3つ:

1. Passive review:
   Claude Code が diff を push ->
   ユーザーが review -> accept/reject
2. Active browse:
   ユーザーがファイルツリーでコードを探索
3. PR review:
   PR の diff を順番にレビュー

---

## 良い点

### Diff 中心の統一モデル

File tab と Diff tab の2種類に絞った判断は、
認知負荷を下げる最も重要な設計判断。
ユーザーのメンタルモデルが「ファイルを見る」か
「差分を見る」の2つで済む。

### Sidebar toggle

Diff を読む時は全幅、ファイルを切り替えたい時だけ
sidebar を出す。操作の頻度に対してレイアウトが
最適化されている。

### Context-aware key hints

Bottom bar の key hints は discoverability を確保しつつ
画面を圧迫しない良いパターン。

---

## 問題点と改善提案

### 1. Comment overlay が diff を隠す

コメントを書く時、そのコメントの対象コードが
overlay に隠れてしまう。
対象行を見ながらコメントを書きたい。

現在の設計:

```txt
|  11     return process(r)      |  11     if r == nil { |
+----- Comment (line 11) --------------------------------+
| @you: Consider error wrapping here                     |
+--- [c]lose [r]eply [d]elete --------------Esc:Close ---+
|                                |  12     return Err    |
```

line 11 が overlay の上に見えるが、
周辺コンテキストは隠れる。

改善案: overlay を diff の下部 (bottom area) に表示

```txt
+--Old (before)------------------+--New (after)----------+
|  10 func Handle(r *Request) {  |  10 func Handle() {   |
|  11     return process(r)      |  11     if r == nil {  |
|                                |  12       return Err   |
|  12     // ...                 |  13     }              |
+--------------------------------+------------------------+
+--Comment (line 11)-------------------------------------+
| @you: Consider error wrapping here                     |
| @you: Also add context parameter                       |
+-- [r]eply [d]elete  Esc:Close -------------------------+
 [a]ccept [r]eject  n/N:File
```

diff コードが見えたままコメントを確認できる。
Content area を上下に split する形。
高さは動的に (コメント数に応じて、最大50%) 調整。

重要度: 高

### 2. SbS diff が 80 列で実用的か

80 列 terminal で全幅 SbS の場合:

- 行番号 gutter: 各側 4 文字
- 区切り: 1-3 文字
- コード表示領域: 各側 約 34 文字

34 文字では Go のコードは頻繁に折り返しが発生する。

改善案:

```txt
Terminal 幅に応じたデフォルト:
  < 100列: Unified
  >= 100列: Side-by-Side
```

ユーザーは u/s で手動切り替え可能。
デフォルトのみ幅依存にする。

重要度: 高

### 3. Tab bar のスケーラビリティ

Claude Code が 10 ファイルの diff を push すると
tab bar が溢れる。80 列で tab ラベルを表示すると
6-7 tab が限界。

改善案:

```txt
Tab bar overflow 戦略:
- 表示しきれない tab は省略記号で示す
- Active tab は常に表示
- 左右矢印で tab bar をスクロール

 < [handler.go] [*utils.go*] [auth.go] [main.go] >
   ^ scroll indicator

または tab 数が多い場合は番号表示に切り替え:
 [1:handler] [2:*utils*] [3:auth] ... (8 tabs)
```

重要度: 中

### 4. Sidebar のコンテキスト切り替えが暗黙的

Tab を切り替えると sidebar の中身が
FileTree から Changed files に暗黙的に変わる。
ユーザーが sidebar を開いた時、
「前は FileTree だったのに違う内容が出た」と
混乱する可能性がある。

改善案:

Sidebar のヘッダーにラベルを表示して
現在の内容を明示する。

```txt
+--Files--+            +--Changes (3)--+
| src/    |     or     | > handler.go  |
|  main.go|            |   utils.go    |
+---------+            +   main.go     +
                       +---------------+
```

重要度: 中

### 5. Empty state / 初期状態の設計がない

gracilius を起動して Claude Code がまだ接続していない時、
または接続していても diff が来ていない時の画面が未定義。

改善案:

```txt
+--FileTree---+--Welcome-----------------------------+
| src/        |                                      |
|   main.go   |  gracilius                           |
|   handler.go|                                      |
|   utils.go  |  Waiting for Claude Code...          |
|             |  Port: 18765                          |
|             |                                      |
|             |  Select a file to browse,             |
|             |  or start Claude Code with:           |
|             |  CLAUDE_CODE_SSE_PORT=18765 claude    |
|             |                                      |
+-------------+--------------------------------------+
 j/k:Move  Enter:Open  /:Search  ?:Help
```

接続後:

```txt
+--FileTree---+--Connected---------------------------+
| src/        |                                      |
|   main.go   |  Connected to Claude Code            |
|   handler.go|                                      |
|   utils.go  |  Diffs will appear here as Claude    |
|             |  makes changes.                      |
|             |                                      |
+-------------+--------------------------------------+
```

重要度: 中

### 6. Hunk 間ナビゲーションがない

`n/N` でファイル間を移動できるが、
1ファイル内の hunk 間ジャンプが定義されていない。
長い diff ではスクロールだけだと効率が悪い。

改善案:

```txt
| Key       | Context | Action          |
| `}` / `{` | Diff    | Next/prev hunk |
```

`]` / `[` は comment thread navigation に使っているので、
`}` / `{` で hunk 移動。
vim の paragraph motion と同じキー。

重要度: 中

### 7. Review 進捗の可視化が弱い

Status bar に `Diff 1/3` とあるが、
どのファイルが accepted/rejected/pending かの
全体像が分かりにくい。

改善案: Sidebar (Changed files) にステータスを表示

```txt
+--Changes (3)---------+
| > handler.go  [+3-1] |
|   utils.go    [+1]   |
|   main.go     [done] |
+-----------------------+
```

`[done]` = accepted, `[rej]` = rejected,
未レビューはステータスなし。

重要度: 中

---

## 総合評価

Proposal A の骨格は堅実。Diff 中心の設計、
tab + sidebar toggle の構造、overlay の活用は
ワークフローに対して適切に設計されている。

上記の改善点は骨格の変更ではなく、
ワークフローの細部を詰める内容。

### 優先的に対処すべき2点

1. Comment を bottom area に
   (対象コードの可視性確保)
2. Terminal 幅に応じた SbS/Unified デフォルト切り替え
