# Seed private registry acceptance

Status: Host が所有する Basic 認証 credential 経路について、実際の GitHub-hosted Ubuntu runner で acceptance 済みです。

## 実証したこと

`seed-private-registry-acceptance` workflow は、実 containerd daemon と nerdctl に対して production の Incus Seed acquisition 経路を実行します。テストは loopback 上に認証付き OCI Distribution 互換 endpoint を立ち上げ、immutable な SHA-256 identity を持つ OCI manifest/config/layer を提供し、`SandboxProvider.exportSeedImages` に exact な `reference@sha256:...` identity を trusted Host の `hacocoon-seed` namespace へ取得させます。

acceptance では次を必須とします。

- 正しい Basic 認証 credential を含む Docker 互換 Host credential config により、exact immutable image を取得できること
- 取得後も exact digest を trusted Host Seed namespace で inspect できること
- export された Seed archive に username/password の credential sentinel が含まれないこと
- 間違った Host credential では acquisition が失敗し、guest egress や unauthenticated access に fallback しないこと
- acceptance 専用の別 pull 実装ではなく、Seed construction と同じ `exportSeedImages` 経路を使うこと

成功した reference run は Ubuntu 24.04、runner 既定の containerd service、release asset の SHA-256 で pin した nerdctl 2.3.5 を使用しました。

## Transport の範囲

acceptance registry は loopback HTTP です。nerdctl は loopback registry を local/insecure endpoint として扱うためです。このテストが実証するのは Host-owned authentication と immutable identity の境界であり、production registry の TLS PKI や custom CA 設定までは検証したとは主張しません。production registry の transport trust は operator/containerd/nerdctl 側の設定事項です。

## 再実行

`seed-private-registry-acceptance` workflow を `workflow_dispatch` から実行します。registry と一時 credential は隔離された runner 内で生成されるため、repository secret は不要です。

Go acceptance test 自体も `HACO_E2E_PRIVATE_REGISTRY=1` で gate されており、通常の unit/PR CI が実 containerd acceptance を実行したかのように見せることはありません。

## 残る v0.17 acceptance

ここでカバーするのは Host-owned authenticated registry 経路だけです。実 Incus を使う Seed end-to-end acceptance、物理 Btrfs COW measurement、real-host failure injection は別 Issue で追跡します。
