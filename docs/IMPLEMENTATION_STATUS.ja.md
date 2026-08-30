# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> **現在の `main` のcode realityを確認するための日本語companion**

厳密な正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。Hacocoonは **pre-1.0** です。

現在、**完全実装済みproduct milestoneはv0.1〜v0.16まで連続**しています。v0.17はDocker compatibility foundation段階、v0.18-v0.19はplannedです。

## 現在のrepository reality

| 領域 | 現在の状態 | Milestone | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `exec` / `shell` / `delete` | v0.1 | unit/process。real Incus pending |
| Workspace model / Lease | canonical identity、RO/RW lease、conflict prevention、stale recovery | v0.1-v0.2 | unit/persistence/concurrency/process |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 | unit/process。real SSH pending |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 | unit/process/CLI E2E |
| Git / GitHub push | host-side brokered push。reusable credentialをEnvironmentへexportしない | v0.5 | unit/adversarial/real-git/E2E |
| Agent / Orchestrator | `haco run`、machine JSON、security events | v0.6 | unit/race/process/E2E |
| EC2 / AWS | experimental/default-off remote runtime、AWS capability、EBS recovery | v0.7 | fake-AWS/E2E。real AWS pending |
| VS Code Client Adapter | `haco-vscode` + Remote-SSH、Windows/WSL bridge/bootstrap | v0.8 | helper/static。real client pending |
| Per-agent sandbox broker | opaque session → persisted dedicated Environment binding | v0.9 | ownership/persistence/collision/release proof |
| VS Code Agent Host Adapter | `haco-agent-host prepare/release`、hashed alias、loopback SSH、client-side private key | v0.10 | PR #137で実装。real Agent Host pending |
| Base identity / selection | immutable Base revision pin、`haco image list/inspect`、`create --base` | v0.11 | unit/fake-Incus。real image source pending |
| Resource budgets | CPU/memory/PID/root、Incus pre-start apply/read-back | v0.12 | unit/fake-Incus。real enforcement pending |
| Managed sandbox network | `haco-sandbox0` bridge、`haco-sandbox-egress` ACL、`haco-sandbox` profile、drift fail-closed | v0.13 | unit/static。real Incus network pending |
| Git fetch plugin | `haco plugin git fetch`、host `gh auth git-credential`、fixed refspec | v0.14 | CLI/provider/security coverage |
| OCI Seed recommendation | latest snapshot、6h freshness、30d window、immutable recommendation、top 10% auto promotion | v0.15 | persistence/sampling/ranking test |
| OCI image deletion | `haco image delete`、digest revalidation、Host cache deletion、tombstone、optional all-Environment deletion | v0.16 | deletion/tombstone/retry test |
| Docker compatibility | design + systemd socket/service packaging foundation | v0.17 | partial。full plugin lifecycle pending |
| Optional Local OCI Registry | optional registry/proxy。direct upstream pullはvalid | v0.18 | planned |
| OCI Seed Builder / COW | offline builder、immutable Seed publish、current pointer、Incus/Btrfs COW | v0.19 | planned |
| CI / release hardening | Go test/vet/race、docs/bootstrap/release/workflow checks、local CI runner | cross-cutting | real provider acceptanceは別 |

## 実装の流れ

```text
v0.1  Secure Workspace
 -> v0.2 Workspace Lease
 -> v0.3 Client Access
 -> v0.4 Policy/Capability
 -> v0.5 GitHub push
 -> v0.6 Agent/Orchestrator
 -> v0.7 EC2/AWS experimental
 -> v0.8 VS Code adapter
 -> v0.9 Per-Agent binding
 -> v0.10 Agent Host adapter
 -> v0.11 Base
 -> v0.12 ResourceBudget
 -> v0.13 Managed Network
 -> v0.14 Git Fetch Plugin
 -> v0.15 OCI Seed Recommendation
 -> v0.16 OCI Image Deletion
 -> v0.17 Docker compatibility foundation
 -> v0.18 Local Registry planned
 -> v0.19 Seed Builder/COW planned
```

## 重要な境界

### v0.13 Managed Network

Hacocoon-owned network/profile substrateを使用し、Incus `default` などbroad networkingへsilent fallbackしません。domain-aware authorizationはIncus ACLの機能として偽装せず、上位broker/proxy/pluginへ分離します。

### v0.14 Git Fetch

Fetchは `haco plugin git` 配下です。GitHub credentialはtrusted Host側の `gh auth git-credential` に残し、repository-controlled Git configにprivileged transportを再定義させません。

### v0.15 / v0.16 OCI selection

`haco image seed sample|recommend` はimmutable identityを観測し、上位10%をfuture Seedへauto-selectします。`haco image delete` のtombstoneは同一identityのrecommend/auto promotionより優先します。ただしphysical Seed build/publishが実装済みという意味ではありません。

### v0.17 Docker compatibility

標準runtimeはcontainerd + nerdctlのままです。Docker compatibilityはoptional plugin/adapterとして扱います。現在はsystemd packaging/foundationまでで、on-demand Engine/plugin lifecycleは未完成です。

### v0.18 / v0.19

Local Registryはoptionalで、normal `nerdctl pull` やSeedのprerequisiteではありません。v0.19はtrusted Host acquisition、offline Builder、immutable Seed publication、revision pin、COWを担当します。複数Environmentで一つのwritable `/var/lib/containerd` を共有する実装は禁止です。

## Acceptanceの境界

unit test、fake-provider E2E、race、vet、build、repository CIはreal-provider/client acceptanceの代替ではありません。real Incus lifecycle/network/resource enforcement、Windows/WSL + VS Code、Agent Host、real image/private registry、AWS/EC2/SSM/EBSは別途確認が必要です。

## Compatibility status

v0.19までのmilestone番号はCLI/API/state/config freezeを意味しません。pre-1.0の間はunsafe authority boundary、ambiguous ownership、accidental provider coupling、不要な複雑性を修正するBreaking Changeを許容します。
