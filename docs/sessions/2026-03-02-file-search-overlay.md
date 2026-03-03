# File Search Overlay 実装

## 目的

gracilius TUIにファイル検索オーバーレイ機能を追加する。
`/` キーでオーバーレイを表示し、ファイル名のファジーフィルタで
素早くファイルを開けるようにする。
`bubbles/list.Model` を使用してフィルタリング・カーソル・
スクロールをライブラリに委譲する。

## 作業内容

1. `internal/tui/search.go` を新規作成
   - `fileItem` 型 (list.Item/DefaultItem 実装)
   - `searchOverlay` 構造体 (active, list.Model)
   - `scanAllFiles` / `collectFiles` で filetree.go の
     `scanDir` を再利用した再帰スキャン
   - `open` / `close` / `update` / `selectedPath` / `view` メソッド
2. `internal/tui/keys.go` に `Search` キーバインド (`/`) 追加
3. `internal/tui/model.go` に `search` フィールド追加、
   `openFileByPath` メソッドを `toggleTreeEntry` から抽出
4. `internal/tui/update.go` に search モードのキー/マウス処理追加
5. `internal/tui/view.go` に search overlay 描画分岐追加
6. `internal/tui/search_test.go` でテスト作成
7. `/simplify` で3並列レビュー実施、以下を修正:
   - `scanAllFiles` を `scanDir` ベースに変更 (重複走査排除)
   - `overlayBorder` を package-level var から関数内に移動
   - `close()` / `update()` メソッド追加でカプセル化改善
   - `open()` の冗長な呼び出し削除
   - overlay close 時に `SetItems(nil)` でメモリ解放

## 変更ファイル

- @internal/tui/search.go
- @internal/tui/search_test.go
- internal/tui/keys.go:8-26, 91-98, 102-115
- internal/tui/model.go:90-95, 114-165, 185-190
- internal/tui/update.go:5, 105-110, 237-257, 404-408
- internal/tui/view.go:34-40

## 利用したSkill

- /simplify

## Pull Request

<https://github.com/708u/gracilius/pull/26>

## 未完了タスク

- [ ] main ブランチの取り込み
- [ ] `.twig-claude-prompt-feat-file-search-overlay.sh` を
  `.completed-twig-claude-prompt.sh` にリネーム
