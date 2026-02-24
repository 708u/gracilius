# リファクタリング残課題 (Medium / Low)

High は 969f7cf で対応済み。
以下は未対応の Medium / Low 項目。

## Medium

### 11. `sendNotificationNow` / `sendPendingNotification` 重複

- server.go
- params構築、マーシャル、ブロードキャストが同一
- 共通の送信関数を抽出すべき

### 12. `Start()` / `StartAsync()` のコード重複

- server.go
- HTTPサーバーセットアップの共通化が必要

### 13. `selectionNotification` / `selectionState` 同一構造

- server.go
- 完全に同一のフィールドを持つ2つの型
- 1つに統合可能

### 14. `HandleMessage` の戻り値 `*Notification` が未使用

- handler.go
- 全ケースで `nil` を返し、呼び出し元でも無視
- 戻り値を削除するか、将来的に使う予定がなければ除去

### 15. `clientCapabilities` / `clientInfo` デッドストア

- handler.go
- 保存するだけで参照されない
- 将来使う予定がなければ削除

### 16. `closeAllDiffTabs` のカウンタが常に0

- handler.go
- レスポンスが常に `"CLOSED_0_DIFF_TABS"`
- コールバックから実際のカウントを返すか、
  固定メッセージに変更

### 17. `scanDir` の未使用引数 `rootDir`

- filetree.go

### 18. `focusPane` が int 型でマジックナンバー管理

- model.go
- 0: tree, 1: editor を定数/型定義すべき

### 19. `Update()` が約350行の巨大関数

- update.go
- MouseMsg と KeyMsg の処理を別メソッドに分離

### 20. `getScrollOffset()` は不要なラッパー

- update.go
- `m.scrollOffset` を返すだけ
- 呼び出し元で直接参照に置換

### 21. `getContentHeight()` の冗長なローカル変数

- update.go
- `return max(m.height-5, 5)` で十分

### 22. `getTreeWidth()` 計算の重複

- update.go: WindowSizeMsg 処理と getTreeWidth() で
  同じ `m.width * 70 / 100` を計算

### 23. highlight_test.go のグローバル状態副作用

- `styles.Register()` がグローバルレジストリに登録
- テスト間の状態共有リスク

### 24. highlight_test.go の弱い検証

- `strings.Contains(output, "l")` では
  カーソル位置の正確性を検証できない

### 25. 非選択時のアンカー同期パターン4箇所重複

- update.go
- 対応済み: `syncAnchorToCursor()` で統一済み

### 26. view.go `renderLineWith*` のインターフェース不統一

- 対応済み: styled版に委譲する形で統一済み

### 27. watch.go のデータ競合リスク

- goroutine内で `m.watcher` を参照
- nil設定との競合可能性

### 28. fileio.go の `\r\n` 改行未処理

- `strings.Split(string(content), "\n")` は
  `\r\n` を処理しない
- Windows由来ファイルで行末に `\r` が残る

## Low

### 29. マジックストリング / マジックナンバー各種

- jsonrpc.go: `"2.0"` が3箇所
- handler.go: `"2025-11-25"` (デフォルトプロトコルバージョン)
- handler.go: `-32601` (JSON-RPCエラーコード) が2箇所
- server.go: `"[Comment]"` (コメント通知プレフィックス)
- server.go: debounce 100ms
- highlight.go: `"github-dark"` (テーマ名)
- update.go: スクロール量 `3`、ヘッダー高さ `1`、
  セパレータ幅 `3`、行番号幅 `4`

### 30. main.go: `filepath.Abs` エラー無視

- 対応済み

### 31. main.go: `os.Symlink` エラー無視

- latestシンボリックリンク作成失敗時

### 32. lockfile.go: 文字列スライスでの拡張子除去

- 対応済み: `strings.TrimSuffix` に変更済み

### 33. lockfile.go: `workspaceFoldersMatch` 冗長

- `slices.Equal()` で置換可能

### 34. lockfile.go: `isProcessAlive` が非Windows

- 対応済み: `CheckDuplicateWorkspace` / `isProcessAlive`
  ごと削除済み

### 35. view.go: `headerRendered` 不要な変数

- `header` をそのまま使えばよい

### 36. filetree.go: `sort.Slice` -> `slices.SortFunc`

- Go 1.26 で利用可能

### 37. handler.go: ツール定義が `map[string]any`

- 型安全性がない
- 構造体定義に置換すべき
