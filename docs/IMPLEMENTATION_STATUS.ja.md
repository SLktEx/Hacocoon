# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) です。

Hacocoon は pre-1.0 です。v0.17 OCI Seed Builder & Btrfs/COW がpartialなfeature gateなので、完全実装済みの product progression は **v0.16まで連続**しています。一方、v0.18 Docker Compatibilityのrepository実装は番号入れ替えより先にland済みで、その実装はそのまま有効です。

| 領域 | 現在の状態 | Milestone |
|---|---|---:|
| Runtime / Workspace | Incus Environment lifecycle、Workspace identity、RO/RW lease | v0.1-v0.2 |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 |
| Git push | trusted Host がbrokerし、reusable Host credentialをEnvironmentへ渡さない | v0.5 |
| Agent integration | `haco run`、machine output、events。orchestrationはCore外 | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` がcapability auditを最小化済みeventへprojectionし、stable ID、resume cursor、bounded batch、recovery/attention flag、public corruption errorを提供。観測はcapabilityを承認・実行しない | v0.6 / cross-cutting |
| Environment routing | provider-neutral seamは維持。**具体的なcloud implementationは現在deferred**で、EC2/AWS/EBS実装はactive treeにない | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` がexact Environment ensure/reuse、status、loopback SSH/TCP、revoke/delete、`/workspace` discovery、`pkg/interaction` batchをpackage-owned DTOで公開。通常の `haco ssh` がnon-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`、per-agent binding、`haco-agent-host` | v0.8-v0.10 |
| Base | `haco base list` / `inspect`、immutable Base revision | v0.11 |
| Resource budget | CPU / memory / PID / root storage | v0.12 |
| Managed Sandbox Network | `haco-sandbox0`、ACL substrate、`haco-sandbox` profile | v0.13 |
| Git Fetch Plugin | `haco plugin git fetch`、Host `gh auth git-credential` | v0.14 |
| OCI plugin boundary | `HACO_PLUGIN_OCI=nerdctl|docker` の明示opt-in。未設定でもCoreは動作する | cross-cutting |
| OCI Seed Recommendation | `haco plugin oci seed sample` / `recommend`、top 10%を `auto_promote=true` | v0.15 |
| OCI Image Deletion | `haco plugin oci image delete`、deletion tombstone、optional all-environments | v0.16 |
| OCI explicit Seed selection | `haco plugin oci seed pin/unpin/re-enable` がimmutable operator intentをtelemetryと別にpersist。pinとauto-promotionは別扱い、re-enableは既存deletion必須、さらに新しいdeletionがあれば再びdeletionが勝つ | v0.17 partial |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build/current`、explicit immutable operator selection、Tooling/Seed manifest、trusted Host acquisition、offline no-NIC builder、immutable publish/current pointer、exact-parent resolutionを実装。real-host/COW acceptanceとGC/recoveryはpending | v0.17 partial |
| Docker Compatibility | `haco plugin oci docker status/prepare`。Base提供profileとpinned systemd unitを検証し、active vendor daemonを勝手に停止せずEnvironment-local socket activationだけを有効化 | v0.18 implemented |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Client adapter境界

`pkg/clientadapter` がVS Codeに依存しないreusable adapter-facing contractです。exported signatureは `internal/core` typeではなくpackage-owned DTOとpublic error sentinelだけを使います。canonical Host Workspaceとrequested access modeが完全一致する場合だけEnvironmentをensure/reuseし、guest内Workspaceは `/workspace` として公開します。connection metadataのreconcileとpublic `pkg/interaction` event contractも同じ境界から利用できます。

SSH prepareが受け取るのはpublic-key materialだけで、private keyとIDE configはclientが保持します。返却/reconcileされたSSH/TCP connectionはloopback-onlyか再検証し、provider outputがcontract違反ならrejectします。新規作成したinvalid connectionはrevokeし、cleanupを証明できなければrecovery-requiredです。既存の `haco create` + `haco ssh` + 通常の `ssh` がnon-VS-Code proofです。詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Client interaction境界

`pkg/interaction` が reusable なclient-facing event contractです。既存のtrusted capability audit streamを読み、schema/event/request identity、UTC時刻、event kind、Environment/capability/action、attention/recovery flag、closed failure code、次回resume cursorだけを公開します。

raw capability resource、authority attributes、opaque parameters、provider output、approval token、credential、free-form audit reasonはclient schemaへ出しません。browser、VS Code、code-server、JetBrains等のadapterは独立にeventを観測・dedupできますが、eventを読むこと自体には副作用がなく、trusted Policy/Capability approval / execution boundaryの代わりにはなりません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。

Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## OCI Seed / storage

v0.17はfirst repository slice実装済みです。trusted Host acquisition/cache → offline no-NIC Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver clone まで実装されています。immutable pin/re-enable selectionはusage telemetryとは別にpersistし、pinはautomatic promotionとは別のままfuture Seed対象へ含めます。tombstone済みidentityは明示re-enableを要求し、その後さらに新しいdeletionがあれば古いre-enable/pinよりdeletionが再び優先されます。これらselection操作はnetwork/credential authorityを付与しません。複数Environmentで一つのwritable `/var/lib/containerd` を共有しません。

Local Registryはprerequisiteではなくroadmap versionも予約しません。残件はold Tooling/Seed revision GC、restart/crash recovery、authenticated/private-registry combination、physical Btrfs COW measurement、broader real-host acceptanceです。

## Docker compatibility

v0.18のrepository gateは実装済みです。このcodeはfeatureが一時的にv0.17と呼ばれていた時点でlandしましたが、rollbackせずv0.18の実装として扱います。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求します。unit driftや既にactiveなvendor Docker daemonがあればfail closedします。

real Incus/systemd acceptanceはrepository実装とは分離したhost-dependent項目です。

## Cloud

v0.7 の provider-neutral routing seam は残しますが、以前の concrete EC2/AWS/EBS implementation は active tree から意図的に外しています。local/provider contract が安定するまで cloud support は deferred です。
