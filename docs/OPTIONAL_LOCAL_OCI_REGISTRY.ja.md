# Optional Local OCI Registry

Status: **deferredなoptional infrastructureです。roadmap milestoneではなく、normal OCI pullやSeed constructionの必須要件でもありません。**

Hacocoonは全Environmentの `nerdctl pull` をHacocoon-managed Registry経由にしません。標準方針はconfigured upstreamへのnormal pullと、trusted Host-side cache/Seed acquisitionです。

## Default behavior

```text
Environment -> nerdctl pull -> configured upstream registry
```

Docker Hub、GHCR、private registry等への到達可否はHacocoon network policyに従います。

## Authentication boundary

- Host credentialをcoding Environmentへsilent copyしない
- Environment-owned credentialは通常のnerdctl/Docker-compatible設定を利用可能
- scoped/short-lived credential brokerはLocal Registryなしでも成立可能
- OCI Seed acquisitionはEnvironment credentialと分離したtrusted Host-side credentialを使う

## Registryが有効になりうるケース

将来、実測やpolicy上の理由がある場合だけoperatorがLocal Registry/proxyを選べます。

- 多数のEnvironmentが同じnon-seeded imageを繰り返しpullする
- upstream rate/bandwidth limitが問題になる
- centralized OCI policy/audit pointが必要
- Environment Internet accessを制限しinternal distribution endpointだけ許可したい

これはproduct milestoneを予約しません。将来、本当に独立したHacocoon機能として実装するなら、その時点の次minorを使います。

## 将来enabledする場合のrequirements

OCI Distribution-compatible implementationを優先し、public exposeをdefaultにせず、proxy auth時のreusable upstream credentialはtrusted sideに保持します。mandatory mediation設定時にRegistry failureをunrestricted direct-Internet fallbackへsilent downgradeしません。shared nameへのarbitrary push authorityをdefaultで与えず、immutable identityにはdigestを使い、GCはconservativeにします。

## OCI Seedとの関係

OCI SeedはLocal Registryに依存しません。v0.18 Seed Builderはofflineのまま、trusted HostからOCI contentを受け取ります。
