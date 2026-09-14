# Seed private registry acceptance

状態: **撤去したSeed経路の過去の受入**。Seed取得実装・試験・手動workflow jobを撤去しました。以下のHost所有Basic認証の過去の実績は保持し、現行の永続Storeの受入とは扱いません。

## 実証したこと

`incus-core-e2e` workflow 内の手動 `authenticated-private-registry` job は、実 containerd daemon と nerdctl に対して production の Incus Seed 取得経路を実行します。テストはループバック上に認証付き OCI Distribution 互換接続先を立ち上げ、不変の SHA-256 識別を持つ OCI manifest/config/layer を提供し、`SandboxProvider.exportSeedImages` に正確な `reference@sha256:...` 識別を信頼された Host の `hacocoon-seed` 名前空間へ取得させます。

検証では次を必須とします。

- 正しい Basic 認証認証情報を含む Docker 互換 Host 認証情報設定により、正確な不変のイメージを取得できること
- 取得後も正確なダイジェストを信頼された Host Seed 名前空間で inspect できること
- export された Seed アーカイブに username/password の認証情報 sentinel が含まれないこと
- 間違った Host 認証情報では取得が失敗し、guest egress や unauthenticated access に代替経路しないこと
- 検証専用の別 pull 実装ではなく、Seed construction と同じ `exportSeedImages` 経路を使うこと

成功した reference run は Ubuntu 24.04、runner 既定の containerd サービス、release asset の SHA-256 で固定した nerdctl 2.3.5 を使用しました。

## Transport の範囲

検証 registry はループバック HTTP です。nerdctl はループバック registry を local/insecure 接続先として扱うためです。このテストが実証するのは Host 所有 authentication と不変の識別の境界であり、production registry の TLS PKI や custom CA 設定までは検証したとは主張しません。production registry の通信 trust は operator/containerd/nerdctl 側の設定事項です。

## 過去の再現と残る範囲

撤去した試験と手動jobはGit履歴の `aaa4aa5760cd48aec5a4fc26cbdfd612119bdc82` に残ります。
現行の実行手順ではありません。通常PRで手動専用試験がSKIPだった記録を、job撤去でPASSへ
変更しません。

過去の実績はHost所有のBasic認証取得だけです。Seed/CoW全体や失敗注入の受入は、
この試験では証明されていません。現行の永続Storeの認証互換性は製品経路で別途確認が必要で、
旧データの退避・復元・照合も未完了です。
