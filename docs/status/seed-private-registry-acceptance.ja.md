# Seed private registry acceptance

Status: Host が所有する Basic 認証認証情報経路について、実際の GitHub-hosted Ubuntu runner で検証済みです。

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

## 再実行

`incus-core-e2e` workflow を `workflow_dispatch` から実行すると、`authenticated-private-registry` job が手動実行時だけ走ります。registry と一時認証情報は隔離された runner 内で生成されるため、リポジトリ secret は不要です。

Go 検証テスト自体も `HACO_E2E_PRIVATE_REGISTRY=1` で gate されており、通常の unit/PR CI が実 containerd 検証を実行したかのように見せることはありません。

## 残る v0.17 acceptance

ここでカバーするのは Host 所有 authenticated registry 経路だけです。実 Incus を使う Seed end-to-end 検証、物理 Btrfs COW 測定、real-host 失敗 injection は別 Issue で追跡します。
