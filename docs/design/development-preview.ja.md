# 開発サーバーの preview

日本語 | [English](development-preview.md)

状態: **ロードマップ C5 は partial**。product CLI と loopback 接続を実装しました。
Windows installer で HTTP と Edge headless 描画の検証が PASS になりました。既定ブラウザの起動は未検証です。

## 通常の使い方

Environment 内でアプリを起動し、次を実行します。

```bash
haco open --port 3000 dev
haco open --port 3000 --close dev
```

最初の command は Environment を再開し、対象ポートへの接続を再利用するか
Physical Host の空き loopback ポートを割り当てます。HTTP URL を表示し、
desktop browser を起動します。Environment が一つなら名前を省略できます。
script では `--no-browser` で URL の表示だけにできます。

二つ目は Environment を起動せず、対応する TCP 接続を閉じます。
開発サーバー自体は停止しません。Environment の削除でも provider 接続は消えます。
従来の `haco open dev` は引き続き VS Code を開きます。

## 境界

既存の client connection provider を使います。public listener や egress Policy
例外は追加しません。Incus proxy は 127.0.0.1 だけで待ち受け、指定 Environment の
loopback ポートへ接続します。空きポートの探索だけを接続成功とは扱わず、
provider の bind 成功が必要です。競合した bind は失敗させます。

検証済み loopback 接続だけから browser URL を生成します。provider が返した
任意 URL・host・path・shell code を起動せず、Environment に desktop command を
選ばせません。DNS・外向き通信の許可や LAN 公開は追加しません。
他のローカル process からはアクセス可能です。

## 検証

接続再利用、再開しない close、不正 endpoint 拒否を focused test で扱います。
Windows installer fixture は通常の project setup から Python HTTP server を起動し、
Windows で Workspace marker を取得して、URL 再利用と close 後の接続拒否を確認します。
 `bffc3fd` では Windows HTTP 応答を取得しましたが、拡張子のない marker が byte 列で返り、内容確認で失敗しました。
ブラウザー表示にも適した text/plain の .txt marker に修正しています。d4aef8d の Windows run 34139245378 で HTTP 内容、URL 再利用、close 後の接続拒否、Edge headless の実描画が PASS になりました。既定ブラウザの起動は未検証です。headless 描画の成功を desktop launcher の確認とは扱いません。

[Client adapter](client-adapters-and-vscode-integration.md) も参照してください。
