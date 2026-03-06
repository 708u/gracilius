# Shift+D (ClearAll) に件数表示付き y/n 確認を追加

## 目的

Shift+D でコメントが即座に全消去されるが、
誤操作時に元に戻す手段がないため、
影響範囲（件数）を示した上で y/n で確認するようにする。

## 作業内容

- `Model` に `clearAllPending bool` フィールドを追加
- `keyMap` に `Confirm` キーバインド (`y`) を追加
- `handleKeyPress` 冒頭に `clearAllPending` 処理を追加
  - `y` でコメント全消去、それ以外のキーでキャンセル
- `ClearAll` ハンドラをコメント0件時はスキップするよう変更
- `renderFooter` に `Clear N comments? (y/n)` 表示を追加
- `/simplify` でレビュー実施、冗長な `activeTabState()` 呼び出しを修正

## 変更ファイル

- @internal/tui/model.go
- @internal/tui/keys.go
- internal/tui/update_key.go:15-45, 302-307
- internal/tui/view.go:294-305

## 利用したSkill

- /simplify
- /commit-push-update-pr
- /export-session

## Pull Request

<https://github.com/708u/gracilius/pull/48>

## 未完了タスク

- [ ] `.twig-claude-prompt-feat-clear-all-confirm.sh` を
      `.completed-twig-claude-prompt.sh` にリネーム
