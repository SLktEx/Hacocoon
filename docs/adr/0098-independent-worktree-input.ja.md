# ADR 0098: 管理importの境界でローカルGit入力をコピーする

状態: accepted
日付: 2026-09-15

## 決定

Linux/WSL clientが明示したcheckout・linked worktreeをprovider共通のtreeとして読み取ります。
作業ファイル・独立したobjects/refs・選んだHEAD/indexをコピーし、元の設定・hooks・認証情報・他worktreeの管理情報は含めません。入力元のGitを実行して探索・exportしません。
Git接続先は明示された現在のHost登録先から選びます。

管理用転送の分割送信とリポジトリimportの共通処理を再利用します。
Incus adapterだけがtreeを非公開の検査済みvolume archiveへ変換します。
作成前の所有予約と、結果不明時の正確な回復記録を保持します。
CoreへGit worktreeやIncus archiveの用語を持ち込みません。
ローカル参照は遠隔操作より先にimport開始を記録します。

## 採用しない方式

共通Gitディレクトリのmountは他作業とHost情報を露出させます。
元のGit設定やhooksをHost準備で実行すると信頼境界を越えます。
client指定の絶対パスをcontrollerが読むと、管理側権限の不正利用につながります。
別のlifecycle台帳は所有権・cleanupの判断を重複させます。周辺の認証情報を暗黙コピーすることはGit接続の代わりになりません。

## 結果

新しい独立Workspaceを保持し、元ファイルは残します。通常Policyと明示的push承認は維持します。
コピー中は元の書き込みを止める必要があり、partial/sparse clone・別objects領域は初期非対応です。
同期・旧版移行・性能の受入は追加しません。
