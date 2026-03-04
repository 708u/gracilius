# fix: word wrap時のvisual selectionハイライトバグ修正

## 目的

TUI幅が小さくword wrapが発生している状態でvキーによる
visual selectionを行うと発生する2つのバグを修正:

1. selectionのbackground colorがfile treeペインに漏れる
2. wrap継続行にselectionハイライトが適用されない

原因: `ansi.Hardwrap`がANSI stateを分割境界をまたいで
伝播しないため、前半セグメントの末尾にresetが残らず、
後半セグメントの先頭にselection bgが付与されない。

## 作業内容

1. `renderEditor()`内の3箇所で、editor行末に`ansiReset`を
   付与し`padRight`で幅を統一する修正を適用（leak修正）
2. `/simplify` で3エージェント並列レビュー実施、
   全指摘がfalse positiveまたはover-engineeringに該当し
   変更不要と判断
3. コミット・プッシュ・PR #37 作成
4. wrap継続セグメントがselection範囲内の場合に
   `selectionBgSeq()`を再適用する修正を追加
5. PR description更新、twigプロンプトリネーム完了

## 変更ファイル

- internal/tui/view.go:342-375

## 利用したSkill

- /simplify
- /commit-push-update-pr
- /export-session

## Pull Request

<https://github.com/708u/gracilius/pull/37>
