# 開発サーバーのプレビュー

日本語 | [English](development-preview.md)

状態: **ロードマップ C5 は部分完了**。製品 CLI とループバック接続を実装しています。導入済み Windows の HTTP 応答と Edge の画面を開かない描画試験は成功しました。既定ブラウザーの起動は未確認です。

## 通常の使い方

Environment 内でアプリを起動し、信頼された Host で実行します。

```bash
haco open --port 3000 dev
haco open --port 3000 --close dev
```

最初のコマンドは Environment を再開し、対象ポートへの接続を再利用するか Physical Host の空きループバックポートを割り当てます。HTTP URL を表示し、デスクトップのブラウザーを起動します。Environment が一つなら名前を省略できます。スクリプトでは `--no-browser` で URL 表示だけにできます。

二つ目は Environment を起動せず、対応する TCP 接続を閉じます。開発サーバー自体は停止しません。Environment の削除でも接続は消えます。`haco open dev` は引き続き VS Code を開きます。

## 境界

既存の接続プロバイダーを使い、公開リスナーや外向き通信の Policy 例外を追加しません。Incus プロキシは 127.0.0.1 だけで待ち受け、指定 Environment のループバックポートへ接続します。空きポートの探索だけでは成功とせず、実際の待ち受け開始が必要です。競合した場合は失敗します。

検証済みの接続情報だけからブラウザー URL を生成します。バックエンドが返した任意の URL・ホスト・パス・シェルコードは起動せず、Environment にデスクトップのコマンドを選ばせません。DNS・外向き通信の許可や LAN 公開は追加しません。ただし他のローカルプロセスからは接続できます。

## 検証

接続の再利用、再開せずに閉じる操作、不正な接続先の拒否を検査します。導入済み Windows の試験では通常のプロジェクト準備から Python HTTP サーバーを起動し、Workspace の確認データ、URL 再利用、閉鎖後の接続拒否を調べます。

`bffc3fd` では拡張子のない確認ファイルを文字列として比較して失敗しました。text/plain の .txt ファイルへ修正後、`d4aef8d` の実行34139245378で内容確認と Edge の描画が成功しました。この試験は既定ブラウザーの起動を証明しません。[検証証拠](../status/acceptance-evidence.ja.md#development)と[クライアント連携](client-adapters-and-vscode-integration.md)を参照してください。
