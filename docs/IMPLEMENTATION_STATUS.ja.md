# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) です。

Hacocoon は pre-1.0 です。v0.17 OCI Seed Builder & Btrfs/COW は first repository slice まで実装済みですが、feature gateとしてはpartialです。そのため、完全実装済みの product progression は **v0.16まで連続**しています。v0.18 Docker Compatibilityはrepository gateまで実装済みです。両実装は旧v0.17/v0.18番号のもとで逆順にlandしており、codeをrollbackせず新しい正本番号へ付け替えます。

| 領域 | 現在の状態 | Milestone |
|---|---|---:|
| Runtime / Workspace | Incus Environment lifecycle、Workspace identity、RO/RW lease | v0.1-v0.2 |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 |
| Git push | trusted Host がbrokerし、reusable Host credentialをEnvironmentへ渡さない | v0.5 |
| Agent integration | `haco run`、machine output、events。orchestrationはCore外 | v0.6 |
| Environment routing | provider-neutral seamは維持。**具体的なcloud implementationは現在deferred**で、EC2/AWS/EBS実装はactive treeにない | v0.7 |
| VS Code / Agent Host | `haco-vscode`、per-agent binding、`haco-agent-host` | v0.8-v0.10 |
| Base | `haco base list` / `inspect`、immutable Base revision | v0.11 |
| Resource budget | CPU / memory / PID / root storage | v0.12 |
| Managed Sandbox Network | `haco-sandbox0`、ACL substrate、`haco-sandbox` profile | v0.13 |
| Git Fetch Plugin | `haco plugin git fetch`、Host `gh auth git-credential` | v0.14 |
| OCI plugin boundary | `HACO_PLUGIN_OCI=nerdctl|docker` の明示opt-in。未設定でもCoreは動作する | cross-cutting |
| OCI Seed Recommendation | `haco plugin oci seed sample` / `recommend`、top 10%を `auto_promote=true` | v0.15 |
| OCI Image Deletion | `haco plugin oci image delete`、deletion tombstone、optional all-environments | v0.16 |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build/current`、Tooling/Seed manifest、trusted Host acquisition、NICなしoffline builder、immutable publication/current pointer、exact-parent resolutionを実装。real-host/COW acceptanceとGC/recoveryはpending | v0.17 partial |
| Docker Compatibility | `haco plugin oci docker status/prepare`。Base提供profileとpinned systemd unitを検証し、active vendor daemonを勝手に停止せずEnvironment-local socket activationだけを有効化 | v0.18 |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。

Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## OCI Seed Builder / storage

v0.17はfirst repository sliceまで実装済みです。trusted Host acquisition/cache → NICなしoffline Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver cloneという流れが実装されています。複数Environmentでwritable `/var/lib/containerd` を共有しません。

Local Registryはそのprerequisiteではなくroadmap versionも予約しません。残る主なacceptanceはreal Incus/containerd/nerdctl、old Tooling/Seed revision GCとrestart/crash recovery、authenticated/private-registry combinations、physical Btrfs COW measurementです。

## Docker compatibility

v0.18のrepository gateは実装済みです。このcodeはfeatureがv0.17と呼ばれていた時点でlandしましたが、削除せずv0.18の実装として扱います。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求します。unit driftや既にactiveなvendor Docker daemonがあればfail closedします。

real Incus/systemd acceptanceはrepository実装とは分離したhost-dependent項目です。

## Cloud

v0.7 の provider-neutral routing seam は残しますが、以前の concrete EC2/AWS/EBS implementation は active tree から意図的に外しています。local/provider contract が安定するまで cloud support は deferred です。
