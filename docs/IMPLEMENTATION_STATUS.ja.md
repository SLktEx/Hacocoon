# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) です。

Hacocoon は pre-1.0 です。完全実装済みの product progression は **v0.16 まで連続**しています。v0.17 OCI Seed Builder & Btrfs/COW は planned、v0.18 Docker Compatibility Plugin は foundation が先行実装済みですが complete gate ではありません。v0.18 の完成には v0.17 の Seed/Base pipeline が前提になります。

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
| OCI Seed Builder / Btrfs COW | trusted Host取得/cache → offline builder → immutable Seed → COW | v0.17 planned |
| Docker Compatibility | genuine Docker CLI / on-demand Engine のplugin-owned foundationは存在。full lifecycle/Base integrationはpending | v0.18 partial/foundation |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。

Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

physical Seed build/publishはv0.17です。Local Registryはそのprerequisiteではなくroadmap versionも予約しません。必要性が実測できた場合だけ将来optional infrastructureとして再検討します。

## Docker compatibility の順序

Docker CLI/Engine adapter code と plugin-owned systemd packaging は v0.18 の foundation として先行実装済みです。v0.18 を complete にするには、v0.17 で整える Environment/Base provisioning path の上に Docker 固有 lifecycle integration と supported-host acceptance を載せます。

## Cloud

v0.7 の provider-neutral routing seam は残しますが、以前の concrete EC2/AWS/EBS implementation は active tree から意図的に外しています。local/provider contract が安定するまで cloud support は deferred です。
