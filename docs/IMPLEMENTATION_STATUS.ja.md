# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> **現在の `main` のcode realityを確認するための日本語companion**
>
> 厳密な正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。Roadmapの希望、planned specification、compatibility guaranteeとは分けて扱います。

Hacocoonは **pre-1.0** です。実装済みであっても、CLI/API/state/configが固定されたこと、production support済みであること、real-provider/client acceptanceが完了したことを意味しません。

**完全実装済みproduct milestoneはv0.16まで連続**しています。v0.17はDocker compatibility foundation段階、v0.18-v0.19はplannedです。

## 現在のrepository reality

| 領域 | 現在の状態 | Milestone | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `exec` / `shell` / `run` / `delete` | v0.1-v0.6 | repository coverage。real Incus pending |
| Workspace model / Lease | canonical identity、RO/RW lease、conflict prevention、stale recovery | v0.1-v0.2 | unit/persistence/concurrency/process |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 | unit/process。real SSH pending |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 | unit/process/CLI coverage |
| Git / GitHub push | host-side brokered push。reusable credentialをEnvironmentへexportしない | v0.5 | unit/adversarial/real-git coverage |
| Agent / Orchestrator | `haco run`、machine JSON、security event export | v0.6 | unit/race/process coverage |
| Environment routing | provider-neutral routing seamは現在も実装済み | v0.7 | router unit coverage |
| Remote / Cloud runtime | concrete EC2/AWS/EBS実装はactive treeから外している | v0.7 historical slice | **cloud implementationは現在deferred**。history/designは将来adapterの参考として残す |
| VS Code Client Adapter | `haco-vscode` + loopback SSH + Remote-SSH | v0.8 | helper/static。real Windows/WSL pending |
| Per-agent sandbox broker | opaque session → persisted dedicated Environment binding | v0.9 | ownership/persistence/collision/release proof |
| VS Code Agent Host Adapter | `haco-agent-host prepare/release`、client-side private key | v0.10 | 実装済み。real Agent Host pending |
| Base identity / selection | immutable Base revision pin、`haco base list/inspect`、`create --base`。旧 `haco image ...` は削除 | v0.11 | unit/fake-Incus。real image source pending |
| Resource budgets | CPU/memory/PID/root、Incus pre-start apply/read-back | v0.12 | unit/fake-Incus。real enforcement pending |
| Managed sandbox network | `haco-sandbox0` bridge、`haco-sandbox-egress` ACL、`haco-sandbox` profile。drift/broad fallbackはfail closed | v0.13 | unit/static。real Incus network pending |
| Git fetch plugin | `haco plugin git fetch`。Host `gh auth git-credential`、fixed refspec、hostile repo Git config rejection | v0.14 | CLI/provider/security coverage |
| OCI plugin namespace | OCI/containerd/nerdctl操作を `haco plugin oci ...` に分離。これはrefactor/boundary correctionで独立versionは消費しない | cross-cutting | CLI namespace routing coverage |
| OCI Seed recommendation | `haco plugin oci seed sample/recommend`。latest snapshot、6h freshness、30d window、immutable recommendation | v0.15 | persistence/sampling/ranking coverage |
| OCI Seed auto-selection | eligible immutable identityの上位10%を切り上げ、最低1件 `auto_promote=true` | v0.15 | deterministic coverage。physical Seed build未実装 |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>`。Host cache+tombstone、optional `--all-environments`、`--force`なし | v0.16 | deletion/tombstone/retry coverage |
| OCI deletion override | tombstoneはexplicit overrideまでrecommend/auto-promoteより優先 | v0.16 | persisted state/recommendation coverage |
| Docker compatibility | genuine Docker CLI / on-demand Engine設計 + Hacocoon systemd socket/service foundation。standard runtimeはcontainerd + nerdctl | v0.17 | foundationのみ。full plugin lifecycle/real-host pending |
| Optional Local OCI Registry | optional Registry/proxy。通常pullやSeedの必須依存ではない | v0.18 | planned |
| OCI Seed Builder / COW | trusted Host acquisition、offline builder、immutable Seed publish/current pointer、GC、Btrfs COW | v0.19 | planned |
| CI / release hardening | Go matrix、vet、race、docs consistency、release checks、local CI runner | cross-cutting | real-provider acceptanceは別 |

## 実装の流れ

```text
v0.1  Secure Workspace
 -> v0.2 Workspace Lease
 -> v0.3 Client Access
 -> v0.4 Policy/Capability
 -> v0.5 GitHub push
 -> v0.6 Agent/Orchestrator
 -> v0.7 Provider Routing seam (cloud implementation deferred)
 -> v0.8 VS Code adapter
 -> v0.9 Per-Agent binding
 -> v0.10 Agent Host adapter
 -> v0.11 Base (`haco base`)
 -> v0.12 ResourceBudget
 -> v0.13 Managed Network
 -> v0.14 Git Fetch Plugin
 -> v0.15 OCI Seed Recommendation (`haco plugin oci`)
 -> v0.16 OCI Image Deletion (`haco plugin oci`)
 -> v0.17 Docker compatibility foundation
 -> v0.18 Local Registry planned
 -> v0.19 Seed Builder/COW planned
```

## Cloud runtimeのdefer

以前のv0.7 EC2 runtime、host-side AWS capability、EBS helper、cloud-specific E2Eはcurrent implementation treeには含めません。

これはcloud providerを今後禁止する意味ではありません。local runtime / Provider contractがまだ大きく変わる段階なので、real cloud acceptanceを継続して回せないconcrete実装を一旦外したものです。provider-neutral routing boundaryは維持しているため、local側が安定した後にadapterとして復活できます。

## 重要な境界

### v0.11 Base

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

`haco base` はEnvironment starting point専用です。OCI/container image lifecycleは `haco plugin oci` に分離されています。

### v0.13 Managed Network

Hacocoon-owned network/profile substrateを使用し、Incus `default` などbroad networkingへsilent fallbackしません。domain-aware authorizationはIncus ACLの機能として偽装せず、上位broker/proxy/pluginへ分離します。

### v0.14 Git Fetch

Fetchは `haco plugin git` 配下です。GitHub credentialはtrusted Host側の `gh auth git-credential` に残し、repository-controlled Git configにprivileged transportを再定義させません。

### v0.15 / v0.16 OCI plugin

```text
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
haco plugin oci image delete docker.io/library/node:24 --all-environments
```

旧 `haco image ...` はcompatibility aliasとして残しません。v0.15 recommendationはphysical Seed build/publish完成を意味せず、v0.16 tombstoneは同一identityのautomatic re-promotionより優先します。physical Seed publish/GCはv0.19です。

### v0.17 Docker compatibility

標準runtimeはcontainerd + nerdctlのままです。Docker compatibilityはoptional plugin/adapterとして扱います。現在はsystemd packaging/foundationまでで、on-demand Engine/plugin lifecycleは未完成です。

### v0.18 / v0.19

Local Registryはoptionalで、normal `nerdctl pull` やSeedのprerequisiteではありません。v0.19はtrusted Host acquisition、offline Builder、immutable Seed publication、revision pin、COWを担当します。複数Environmentで一つのwritable `/var/lib/containerd` を共有する実装は禁止です。

## Acceptanceの境界

unit test、fake-provider E2E、race、vet、build、repository/local CIはreal-provider/client acceptanceの代替ではありません。real Incus lifecycle/network/resource enforcement、Windows/WSL + VS Code、Agent Host、real image/private registry、Docker compatibility、OCI Seed/Btrfsは別途確認が必要です。Cloud acceptanceはconcrete cloud adapterを戻すまでdeferredです。

## Compatibility status

v0.19までのmilestone番号はCLI/API/state/config freezeを意味しません。pre-1.0の間はunsafe authority boundary、ambiguous ownership、accidental provider coupling、不要な複雑性を修正するBreaking Changeを許容します。
