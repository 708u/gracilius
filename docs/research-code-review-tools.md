# コードレビュー TUI ツール: レイアウトパターン調査

## 概要表

| ツール | 言語 | フレームワーク | レイアウト | Diff 形式 | ファイルナビ | コメント | パネル |
| --- | --- | --- | --- | --- | --- | --- | --- |
| lazygit | Go | gocui | 複数パネル固定 | Unified | サイドパネルリスト | N/A | 5固定 + main |
| tig | C | ncurses | Pager ビュー | Unified | ビュー切替 | N/A | 単一 + split |
| gh-dash | Go | bubbletea | リスト + sidebar | delta 経由 | テーブル行 | PR コメント | セクション + sidebar |
| gitui | Rust | ratatui | タブ + パネル | Unified | パネル内ツリー | N/A | タブ + サブパネル |
| delta | Rust | Pager | Pager 出力 | Unified/SbS | N/A (pager) | N/A | N/A |
| diff-so-fancy | Perl | Pager | Pager 出力 | Unified | N/A (pager) | N/A | N/A |

略称: SbS = Side-by-Side, N/A = 該当なし

## 詳細分析

### 1. lazygit

#### lazygit のレイアウト構造

gocui で構築された固定マルチパネルレイアウト。
画面は2つの主要エリアに分割:

```txt
+----------+---------------------------+
| Status   |                           |
+----------+                           |
| Files    |       Main Panel          |
+----------+     (Diff / Content)      |
| Branches |                           |
+----------+                           |
| Commits  |                           |
+----------+                           |
| Stash    |                           |
+----------+---------------------------+
| Options / Status Bar / Information   |
+--------------------------------------+
```

左サイドバー: 5つの stacked パネル
(Status, Files, Branches, Commits, Stash)。
各パネルはタブで複数の "context" を保持可能
(例: Branches パネルに Local Branches, Remotes,
Tags のタブ)。

右メインエリア: コンテキスト依存のコンテンツ
(diff, commit 詳細, ファイル内容) を表示する
単一の大きなパネル。
ステージングビュー用のセカンダリ split をサポート。

主要な設計判断:

- パネルジャンプショートカット ([1]-[5]) で直接アクセス
- パネル内のタブ切替 ([ と ] キー)
- フォーカス中のパネルは色付きボーダーでハイライト
- Main パネルのタイトルがコンテキストに応じて変化
- 画面モード切替 (+/_ キー) でパネルの拡大/縮小
- サイドパネル幅はターミナル幅の約 30%

#### lazygit の Diff 表示

- Unified diff 形式のみ
- カスタム pager による構文認識カラーリング
  (delta, diff-so-fancy 設定可能)
- 行レベルのステージング: space で個別行をステージ
- Hunk レベルの操作: a で hunk 全体を選択
- v キーでビジュアル範囲選択
- 古いコミットからのカスタムパッチ構築
- 2コミット比較モード (shift+w でマーク、diff 表示)

#### lazygit のファイルナビゲーション

- Files パネル内のファイルツリービュー
- フラットリストまたはツリー表示 (切替可能)
- ファイル名横にステータスインジケータ (M, A, D, R 等)
- `/` キーでフィルタ
- ファイル上で Enter すると main パネルに diff を表示

#### lazygit のコンテキスト切替

- 数字キー (1-5) でパネルに直接ジャンプ
- Tab/shift-tab でパネル間を循環
- Enter でサブビューにドリルダウン
  (例: commit > files)
- Escape で前のコンテキストに戻る
- commit > files > diff のパンくずリスト的ナビ

#### lazygit のキーボードフロー

- hjkl または矢印キーでナビゲーション
- Space で主要アクション (stage/unstage)
- Enter でドリルダウン
- Escape で戻る
- ? でヘルプオーバーレイ
- コンテキスト依存のキーバインドが bottom bar に表示

### 2. tig

#### tig のレイアウト構造

pager スタイルの single-view-at-a-time アプローチで、
任意の split view を持つ。
異なるビュータイプを持つモーダルビューアとして機能。

```txt
+-----------------------------------------+
|              Main View                  |
|  (commit log / diff / blame / etc.)     |
|                                         |
|                                         |
|                                         |
+-----------------------------------------+
| Status line                             |
+-----------------------------------------+
```

コミットにドリルダウンするとビューが分割:

```txt
+-----------------------------------------+
| Commit List (上半分)                     |
|  > selected commit                      |
+-----------------------------------------+
| Diff View (下半分)                       |
|  選択コミットの diff を表示              |
+-----------------------------------------+
```

ビュータイプ:

- main: グラフ付きコミットログ
- diff: 完全な diff 出力
- log: フルメッセージ付き詳細ログ
- blame: 行ごとの blame
- refs: ブランチ/タグリファレンスブラウザ
- status: ワーキングツリーのステータス
- stage: ステージングエリア
- stash: stash リスト

#### tig の Diff 表示

- Unified 形式、pager スタイルのスクロール
- 追加 (緑) と削除 (赤) のカラーコーディング
- 設定による行番号表示
- Hunk ヘッダーをハイライト
- .tigrc でカスタマイズ可能 (色, 表示オプション)

#### tig のファイルナビゲーション

- status ビュー: 変更ファイルのリスト
- diff ビュー: 全ファイルを順次スクロール
- ファイルツリーなし。diff 順にファイルが表示
- j/k または矢印キーでスクロール

#### tig のコンテキスト切替

- 各ビュータイプが独立
- Enter で詳細ビューを開く (画面分割)
- q で現在のビューを閉じ、前のビューに戻る
- ビュースタック: push/pop でビューを管理
- 特定ビューへの単一キーショートカット
  (m=main, d=diff, l=log, b=blame, r=refs, s=status)

#### tig のキーボードフロー

- Vim スタイルナビゲーション (j/k/h/l)
- Enter でドリルダウン
- q で戻る
- `/` でビュー内検索
- n/N で次/前の検索マッチ
- stage ビューで u で個別 hunk をステージ

### 3. gh-dash

#### gh-dash のレイアウト構造

設定可能なセクションと詳細サイドバーを持つ
ダッシュボードスタイルのレイアウト。
bubbletea で構築 (gracilius と同じフレームワーク)。

```txt
+-------------------------------------------+
| [PRs] [Issues] [Repos] [Notifications]    |
+-------------------------------------------+
| Section: My PRs                           |
| +---------------------------------------+ |
| | #123 Fix auth bug  OPEN  2h ago  ...  | |
| | #124 Add feature   DRAFT 1d ago  ...  | |
| | > #125 Update docs OPEN  3d ago  ...  | |
| +---------------------------------------+ |
| Section: Review Requested                 |
| +---------------------------------------+ |
| | #200 Refactor API  OPEN  5h ago  ...  | |
| +---------------------------------------+ |
+-------------------------------------------+
| Footer: keybinding hints                  |
+-------------------------------------------+
```

PR を選択して開く (Enter) とサイドバーが表示:

```txt
+---------------------------+---------------+
| PR List                   | PR Detail     |
|                           | Sidebar       |
| #123 Fix auth bug  OPEN   | Description  |
| > #124 Add feature DRAFT  | Checks       |
|                           | Reviews      |
|                           | Files        |
+---------------------------+---------------+
```

コンポーネント階層:

- Tabs (上部): PRs, Issues, Repos, Notifications
- Sections (タブごとに設定可能): クエリベースのグループ
- Rows: 個別の PR/issue エントリ (列付き)
- Sidebar: 選択項目の詳細ビュー
- Footer: コンテキスト依存のキーバインドヒント

#### gh-dash の Diff 表示

- diff レンダリングは delta に委譲
- 完全な diff 表示は外部 pager で開く
- 変更インジケータ付きの PR ファイルリスト
- TUI 内にインライン diff なし

#### gh-dash のファイルナビゲーション

- 列付きのテーブルベースリスト:
  title, status, author, updated, CI status, reviews
- YAML 設定でカスタマイズ可能な列
- クエリでグループ化されたセクション
  (例: authored, review-requested, assigned)
- セクション内の垂直スクロール

#### コメント / アノテーション UI

- markdown としてレンダリングされた
  PR description (glamour 経由)
- テーブル列にレビューステータスインジケータ
- PR ごとのコメント数表示
- カスタムアクションでコメント追加可能
- サイドバーに完全な PR 詳細を表示

#### ステータス情報

- PR ごとの CI チェックステータスアイコン
- レビュー状態インジケータ (approved, changes requested)
- PR 状態バッジ (open, closed, merged, draft)
- 相対タイムスタンプ
- アサイニーとラベルの表示

#### パネル管理

- タブベースのトップレベルナビゲーション (1-4 キー)
- Enter でサイドバーの表示切替、Escape で閉じる
- サイドバー幅は設定可能
- セクションが独立してスクロール

#### gh-dash のキーボードフロー

- j/k で上下ナビゲーション
- Tab でセクション切替
- Enter でサイドバーを開く / ドリルダウン
- Escape でサイドバーを閉じる
- 設定可能なキーによるカスタムアクション
- `/` で検索/フィルタ
- r でリフレッシュ
- c で checkout, d で diff, b でブラウザで開く

### 4. gitui

#### gitui のレイアウト構造

タブベースのレイアウトで、各タブが独自の
パネル配置を持つ。ratatui (crossterm) で構築。

```txt
[Status] [Log] [Files] [Stashing] [Stash List]
+------------------+---------------------+
| Unstaged Changes |                     |
| +- src/          |    Diff View        |
| |  main.rs [M]   |                     |
| +- tests/        |  - old line         |
|    test.rs [M]   |  + new line         |
+------------------+                     |
| Staged Changes   |                     |
| (empty)          |                     |
+------------------+---------------------+
| [key hints based on context]           |
+----------------------------------------+
```

タブの説明:

- Status: unstaged/staged の変更 + diff (主要タブ)
- Log: 詳細展開付きコミット履歴
- Files: HEAD でのリポジトリファイルブラウズ、blame
- Stashing: stash の作成
- Stash List: 既存 stash の管理

各タブが独自の内部パネルレイアウトを持つ。
Status タブは3パネルレイアウト:
unstaged (左上), staged (左下), diff (右)。
Log タブはコミットリスト (左) とコミット詳細 (右)。

#### gitui の Diff 表示

- Unified 形式
- カラーコーディング: 追加 (緑), 削除 (赤)
- セパレータとしての hunk ヘッダー
- diff パネル内でスクロール可能
- diff ビューから個別 hunk や行をステージ
- パス情報付きファイルヘッダー

#### gitui のファイルナビゲーション

- 展開/折り畳み付きツリースタイルのファイルリスト
- ステータスアイコン (M=modified, A=added, D=deleted)
- unstaged と staged で別々のツリー
- Files タブ: 完全なリポジトリツリーブラウザ

#### gitui のコンテキスト切替

- 数字キー (1-5) または Tab でトップレベルタブ切替
- Enter/右矢印で詳細展開
- Escape/左矢印で戻る
- タブ内のサブパネル間でフォーカス循環
- bottom にコンテキスト対応コマンドバー

#### gitui のキーボードフロー

- デフォルトは矢印キー (vim キー設定可能)
- Enter で確認/展開
- Space で stage/unstage
- Tab でサブパネル間を循環
- ? または F1 でヘルプポップアップ
- Bottom bar に現在のコンテキストで利用可能なキーを表示

### 5. delta と diff-so-fancy

完全な TUI ではなく pager ツールだが、
diff の表示パターンは gracilius に関連する。

#### delta

Diff 表示機能:

- 言語認識シンタックスハイライト (bat テーマ経由)
- Levenshtein 編集推論による word-level diff ハイライト
- 行折り返し付き side-by-side ビューモード
- 行番号 (設定可能: 左, 右, または両方)
- ファイルヘッダーデコレーション (ボックス, 下線 等)
- Hunk ヘッダーデコレーション
- n/N キーでファイル間ナビゲーション (--navigate)
- コミットハッシュとファイルパスのハイパーリンク
- マージコンフリクト表示の強化
- シンタックスハイライト付き git blame 表示
- 設定可能なカラーテーマ

Side-by-side レイアウト:

```txt
  3  |  old code here           |  3  |  new code here
  4  |  unchanged line          |  4  |  unchanged line
  5  |  deleted line            |     |
     |                          |  5  |  added line
  6  |  modified old            |  6  |  modified new
```

主要なデザインパターン:

- デフォルトで +/- マーカーを削除しクリーンな見た目
- 追加/削除行に背景色
- 変更行内のインライン word-level ハイライト
- 設定可能なデコレーション (box, line, none)
- Unicode 罫線文字によるファイルセパレータ
- ダーク/ライトテーマの自動検出

#### diff-so-fancy

Diff 表示機能:

- 可読性のために +/- 行プレフィックスを削除
- 簡略化された hunk ヘッダー (人間が読みやすい形式)
- Unicode ルーラーセパレータ付きファイルヘッダー
- 空行のマーキング (最初の列を色付け)
- 変更部分の word-level ハイライト
- ミニマルでクリーンな美観

主要なデザインパターン:

- マシンの構文解析性より人間の可読性を重視
- 視覚的ノイズ削減のため先頭シンボルを除去
- ファイル区切りに Unicode ルーラー
- 設定可能なルーラー幅
- ミニマルなカラーパレット: 緑/赤の背景

### 6. VS Code / JetBrains のコードレビューパターン

#### パネルレイアウト (概念)

IDE のコードレビュー UI は一般的にマルチパネル
アプローチを使用:

```txt
+----------+---------------------------+
| File     | Diff View                 |
| Tree     | (side-by-side or unified) |
|          |                           |
| src/     | - old line                |
|  foo.ts  | + new line                |
|  bar.ts  |                           |
|          | [Comment thread]          |
|          | > "This needs refactoring"|
|          | > Reply: "Will fix"       |
|          |                           |
+----------+---------------------------+
| PR Description / Conversation        |
+--------------------------------------+
```

主要パターン:

- 変更件数バッジ付きの左側ファイルツリー
- unified と side-by-side の両方をサポートする
  メイン diff エリア
- diff 行間にインラインコメントスレッドが表示
- クリックした行にインラインでコメント入力が表示
- 折り畳み可能なコメントスレッド
- レビュー進捗追跡用のファイルごとの "Viewed" チェック
- ファイルごとの追加/削除を示す変更概要
- 変更間のナビゲーション (JetBrains では F7/Shift+F7)

#### コメントスレッディング

- コメントは特定行に紐付けてインライン表示
- スレッド構造: 元のコメント + 返信
- resolved/unresolved スレッドの視覚的インジケータ
- 任意の行で新規スレッドを開始可能
- ファイルツリーエントリにコメントバッジ

### 7. GitHub PR レビューパターン

#### Conversation ビュー

```txt
+------------------------------------------+
| PR Title                        [Merge]  |
| Author  |  Branch  |  Status            |
+------------------------------------------+
| [Conversation] [Commits] [Checks] [Files]|
+------------------------------------------+
| PR Description                           |
| ...                                      |
+------------------------------------------+
| Review Comment (inline context shown)    |
| @user: "This looks wrong"               |
|   > @author: "Fixed in abc1234"          |
|   [Resolve conversation]                 |
+------------------------------------------+
| Commit: "Fix auth bug"                   |
+------------------------------------------+
| Review: @reviewer APPROVED               |
+------------------------------------------+
```

#### Files Changed ビュー

```txt
+------------------------------------------+
| [Conversation] [Commits] [Checks] [Files]|
+------------------------------------------+
| File filter  |  N files changed          |
+------+-------+---------------------------+
| File | src/auth.ts              [Viewed]  |
| Tree | src/utils.ts             [Viewed]  |
|      | > tests/auth.test.ts              |
+------+------------------------------------+
| Diff: tests/auth.test.ts                 |
|  10   | describe('auth', () => {          |
|  11 + |   it('should validate', () => {   |
|  12 + |     expect(validate()).toBe(true); |
|       | [+] Add comment                    |
|  13   |   });                             |
+------+------------------------------------+
```

主要パターン:

- タブベースナビゲーション:
  Conversation, Commits, Checks, Files changed
- Files changed ビューでのファイルツリーサイドバー
  (折り畳み可能)
- ホバー時のインラインコメント挿入
  (gutter の + ボタン)
- 折り畳み可能なコメントスレッド
- レビュー進捗追跡用のファイルごとの "Viewed" toggle
- diff スクロール中のファイルヘッダー固定表示
- レビュー送信フロー:
  comment, approve, request changes
- 送信前のペンディングレビューへのバッチコメント
- ファイル変更インジケータ:
  追加 (緑ドット), 削除 (赤ドット), リネーム (黄ドット)

## コードレビュー TUI の主要デザインパターン

### パターン 1: 2ゾーンレイアウト (ナビゲーション + コンテンツ)

最も一般的で効果的なパターン。
狭いナビゲーションゾーン (ファイルリスト,
コミットリスト, PR リスト) と
広いコンテンツゾーン (diff, description, 詳細ビュー)
の組み合わせ。

比率は一般的に 25-35% ナビゲーション : 65-75% コンテンツ。

使用: lazygit, gitui, gh-dash (sidebar), GitHub

### パターン 2: Stacked パネルナビゲーション

ナビゲーションゾーン内に複数の stacked パネル。
各パネルが異なる側面を表示
(files, branches, commits)。
tab や数字キーでパネル間のフォーカス移動。

使用: lazygit (5パネル), gitui (unstaged/staged)

### パターン 3: タブベースのビュー切替

トップレベルのタブで完全に異なるビューや
ワークフローを切り替え。
各タブが独自の内部レイアウトを持つ。

使用: gitui (5タブ), gh-dash (4タブ), GitHub (4タブ)

### パターン 4: ドリルダウン + Escape-Back

Enter/右矢印で詳細にドリルダウン。
Escape/左矢印で戻る。
ユーザーが自然にナビゲートできるビュースタックを作成。

使用: 調査した全ツール

### パターン 5: コンテキスト対応キーヒント

bottom bar またはオーバーレイで
現在のコンテキストで利用可能なキーバインドを表示。
暗記の負担を軽減し、インラインドキュメントとして機能。

使用: lazygit (bottom bar), gitui (bottom bar),
gh-dash (footer)

### パターン 6: Sidebar Toggle

必要に応じて表示 (Enter) され、
消える (Escape) 詳細サイドバー。
リストのコンテキストを保持しながら詳細を表示。

使用: gh-dash, GitHub (PR レビューのファイルツリー)

## ターミナルでの Diff 表示のベストプラクティス

### カラースキーム

1. 追加: 緑背景または緑前景
2. 削除: 赤背景または赤前景
3. コンテキスト行: デフォルトのターミナル色
4. Hunk ヘッダー: シアンまたは青、暗め
5. ファイルヘッダー: 太字、装飾セパレータ付き
6. Word-level 変更: 行の追加/削除色の中で
   より明るくまたは下線付き

### 行番号表示

- old と new の行番号を別々の gutter に表示
- コンテンツの妨げにならないよう行番号を暗めに
- クリーンな左端のため数字を右揃え
- 追加のみ/削除のみの行では反対側の行番号を省略

### Hunk 表示

- hunk 間に明確な視覚的セパレータ
- 利用可能な場合、hunk ヘッダーに
  関数/クラスのコンテキストを表示
- 長い diff では個別 hunk の折り畳み/展開
- hunk 行範囲の表示 (@@ -10,5 +10,7 @@)

### 長い Diff の処理

- ファイルレベルナビゲーション (ファイル間ジャンプ)
- Hunk レベルナビゲーション (hunk 間ジャンプ)
- 非常に長い diff のための遅延読み込み
- 進捗を示すファイル数インジケータ (3/15 files)
- hunk 間の未変更領域の折り畳み

### Diff でのシンタックスハイライト

- diff コンテンツに言語認識ハイライトを適用
- 追加/削除行でもハイライトを維持
- 追加/削除には背景色、構文には前景色を使用
- delta がこのパターンを効果的に実証

## TUI でのインラインコメント UI の推奨事項

### コメント配置オプション

#### オプション A: Diff 行間にインライン

```txt
  10  |  let x = compute();
  11  |  let y = transform(x);
  ---- Comment by @reviewer -------------------
  | map() で簡略化できるのでは。               |
  | > @author: 確かに、リファクタします。      |
  ---------------------------------------------
  12  |  return y;
```

利点: コメントが関連箇所の正確な位置に表示。
欠点: diff のフローが中断、スクロールが複雑化。

#### オプション B: コメント用サイドパネル

```txt
  10  |  let x = compute();   | @reviewer:
  11  |  let y = transform(x);| "map()で簡略化"
  12  |  return y;             | > "リファクタします"
```

利点: diff のフロー保持、両方が同時に見える。
欠点: 横幅が必要、コードが切れる可能性。

#### オプション C: 選択時のオーバーレイ/ポップアップ

```txt
  10  |  let x = compute();
  11  |  let y = transform(x);  <-- [2 comments]
  12  |  return y;

  [Enter to view comments on line 11]
```

利点: クリーンな diff ビュー、スペースのオーバーヘッドなし。
欠点: 明示的に開くまでコメントが隠れる。

### gracilius への推奨

gracilius は2ペインレイアウト (file tree | editor)
を持ち、MCP 経由で Claude Code と連携することを前提に:

1. 主パターンとしてオプション C (インジケータ + オーバーレイ)
   - diff 行にコメントインジケータ (カウント付きバッジ)
   - Enter または専用キーでコメントを展開
   - 展開されたコメントは Escape で閉じられる
     インラインオーバーレイとして表示

2. トグルとしてオプション A (インライン) をサポート
   - 折り畳み (インジケータのみ) と
     展開 (インラインコメント) モードの切替
   - GitHub の resolved スレッド折り畳みと同様

3. コメント入力
   - 現在の行で新規コメント開始の専用キー (c)
   - 下部またはインラインにテキスト入力エリアを開く
   - Ctrl+Enter で送信、Escape でキャンセル

4. コメントスレッドナビゲーション
   - `]` と `[` でコメントスレッド間をジャンプ
   - resolved と unresolved スレッドの視覚的区別
   - ファイルツリーエントリにスレッド数を表示

### コメント行の視覚的インジケータ

```txt
  10  |  let x = compute();
  11  |  let y = transform(x);     [2]
  12  |  return y;
  13  |  // cleanup                 [1] (resolved)
```

色付きバッジの使用:

- Unresolved: ハイライト背景 (例: 黄色)
- Resolved: 暗め (例: 灰色)
- 自分のコメント: 区別色 (例: 青)

## gracilius に特に関連するパターン

### MCP 駆動のコードレビュー

gracilius は Claude Code から `openDiff` MCP ツール経由で
diff を受け取る。これにより独特なワークフローが生まれる:

1. AI エージェントがユーザーのレビュー用に diff を開く
2. ユーザーがレビューしてフィードバックを提供
3. フィードバックが Claude Code に戻る

これはユーザーがレビューを開始する従来の
コードレビューツールとは異なる。
UI は以下をサポートすべき:

- 受動的な diff 受信:
  Claude が開くと diff が表示される (IDE の動作と同様)
- 素早いフィードバックループ:
  変更の承認/拒否に最小限の摩擦
- バッチレビュー:
  Claude が開いた複数ファイルのレビュー
- Claude へのコメントバック:
  Claude が対応可能な構造化フィードバック

### ブロッキング Diff パターン

MCP 仕様より、`openDiff` はブロッキングツールコール。
TUI は承認または拒否をシグナルする必要がある:

- Accept: Claude が FILE_SAVED 通知を受信
- Reject: Claude が DIFF_REJECTED 通知を受信

diff ビュー下部の2ボタンパターンに適合:

```txt
+----------------------------------------+
| Diff: src/main.go                      |
|                                        |
| [diff content]                         |
|                                        |
+----------------------------------------+
| [a]ccept  [r]eject  [c]omment         |
+----------------------------------------+
```

### ファイルツリー統合

gracilius は既にファイルツリーパネルを持っている。
コードレビューではツリーが以下を示すべき:

- Claude からのペンディング diff を持つファイル
- レビュー済み (accepted/rejected) のファイル
- コメント付きファイル
- 全体のレビュー進捗

```txt
src/
  main.go      [pending]
  handler.go   [accepted]
  utils.go     [rejected]
  auth.go      [2 comments]
```
