# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の code reality を示す companion です。番号の正本は [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) です。

Hacocoon は pre-1.0 です。完全実装済みの product progression は **v0.17 まで連続**しています。v0.18 OCI Seed Builder & Btrfs/COW はrepository implementation slices実装済み / partialです。

| 領域 | 現在の状態 | Milestone |
|---|---|---:|
| Runtime / Workspace | Incus Environment lifecycle、Workspace identity、RO/RW lease | v0.1-v0.2 |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 |
| Git push | trusted Host がbrokerし、reusable Host credentialをEnvironmentへ渡さない | v0.5 |
| Agent integration | `haco run`、machine output、events。orchestrationはCore外 | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` がcapability auditを最小化済みeventへprojectionし、stable ID、resume cursor、bounded batch、recovery/attention flag、public corruption errorを提供。観測はcapabilityを承認・実行しない | v0.6 / cross-cutting |
| Environment routing | provider-neutral seamは維持。**具体的なcloud implementationは現在deferred**で、EC2/AWS/EBS実装はactive treeにない | v0.7 |
| VS Code / Agent Host | `haco-vscode`、per-agent binding、`haco-agent-host` | v0.8-v0.10 |
| Base | `haco base list` / `inspect`、immutable Base revision | v0.11 |
| Resource budget | CPU / memory / PID / root storage | v0.12 |
| Managed Sandbox Network | `haco-sandbox0`、ACL substrate、`haco-sandbox` profile | v0.13 |
| Git Fetch Plugin | `haco plugin git fetch`、Host `gh auth git-credential` | v0.14 |
| OCI plugin boundary | `HACO_PLUGIN_OCI=nerdctl|docker` の明示opt-in。未設定でもCoreは動作する | cross-cutting |
| OCI Seed Recommendation | `haco plugin oci seed sample` / `recommend`、top 10%を `auto_promote=true` | v0.15 |
| OCI Image Deletion | `haco plugin oci image delete`、deletion tombstone、optional all-environments | v0.16 |
| OCI deletion override | tombstoneはrecommendation/auto-promotionと既存pinより優先し、exact immutable `image reenable`でだけ解除する | v0.16 / v0.18 integration |
| Docker Compatibility | `haco plugin oci docker status/prepare`。Base提供profileとpinned systemd unitを検証し、active vendor daemonを勝手に停止せずEnvironment-local socket activationだけを有効化 | v0.17 |
| OCI Seed Builder / Btrfs COW | `seed build/current`、Baseごとの `pin/unpin/pins`、exact immutable `image reenable`、conservative `seed gc/recover`、trusted Host acquisition、offline no-NIC build、immutable publish/current pointer、exact-parent resolution、build前のinterrupted-builder recoveryを実装。real-host/private-registry/COW acceptanceはpending | v0.18 partial |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Client interaction境界

`pkg/interaction` が reusable なclient-facing event contractです。既存のtrusted capability audit streamを読み、schema/event/request identity、UTC時刻、event kind、Environment/capability/action、attention/recovery flag、closed failure code、次回resume cursorだけを公開します。

raw capability resource、authority attributes、opaque parameters、provider output、approval token、credential、free-form audit reasonはclient schemaへ出しません。browser、VS Code、code-server、JetBrains等のadapterは独立にeventを観測・dedupできますが、eventを読むこと自体には副作用がなく、trusted Policy/Capability approval / execution boundaryの代わりにはなりません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。

Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## Docker compatibility

v0.17のrepository gateは実装済みです。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求します。unit driftや既にactiveなvendor Docker daemonがあればfail closedします。

real Incus/systemd acceptanceはrepository実装とは分離したhost-dependent項目です。

## OCI Seed / storage

v0.18はinitial build/publish sliceに加えてoperations-hardening sliceも実装済みです。trusted Host acquisition/cache → offline no-NIC Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver clone まで実装されています。複数Environmentで一つのwritable `/var/lib/containerd` を共有しません。

Baseごとのexplicit pinはimmutable OCI identityとしてpersistします。deletion tombstoneはrecommendationと既存pinより優先し、exact immutable identityを明示re-enableするまで解除しません。`seed recover` はexact Hacocoon temporary builderをreconcileしてからconservative GCを実行し、`seed build` もSeed build lockを保持して新しいbuild前にrecoveryします。GCはIncus-owned Btrfs internalsを直接触らず、current/in-use/instance-base/external-alias imageをretentionします。

Local Registryはprerequisiteではなくroadmap versionも予約しません。

## 残るacceptance

repository testはreal-host acceptanceの代わりではありません。v0.18では、credential leakなしのauthenticated/private-registry combination（support可能な場合のcredential-free Environment harvestingを含む）、physical Btrfs COW measurement、real supported-host acceptance、broader real-host failure injectionが残っています。

## Cloud

v0.7 の provider-neutral routing seam は残しますが、以前の concrete EC2/AWS/EBS implementation は active tree から意図的に外しています。local/provider contract が安定するまで cloud support は deferred です。
