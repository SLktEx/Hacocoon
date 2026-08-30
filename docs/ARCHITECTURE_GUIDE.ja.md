# Hacocoon Architecture Guide

> **日本語で読むための architecture overview**
>
> 実装事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)、milestone番号は [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。

## Hacocoonとは

**Hacocoonは Secure Workspace Runtime です。** Workspaceを隔離Environmentで実行し、Hostや外部サービスのauthorityをtrusted sideのPolicy / Capabilityで制御します。

```text
Client / IDE / Agent / Orchestrator
                |
             Workspace
                v
          Hacocoon Core
                |
        Provider / Adapter / Plugin
                |
        Incus / GitHub / optional OCI tooling
```

v0.7のprovider-neutral routing seamは維持していますが、concrete EC2/AWS/EBS implementationはactive treeから外しており、cloud implementationは現在deferredです。

## 現在のmilestone

| Version | Gate | State |
|---|---|---|
| v0.1〜v0.6 | runtime / workspace / access / policy / Git / agent integration | 実装済み |
| v0.7 | Remote / Cloud Runtime | provider routing seamのみ維持。concrete cloudはdeferred |
| v0.8〜v0.10 | VS Code / per-agent / Agent Host | foundation含め実装済み |
| v0.11 | Base Images | first slice実装済み |
| v0.12 | Sandbox Resource Limits | first slice実装済み |
| v0.13 | Managed Sandbox Network | 実装済み |
| v0.14 | Git Fetch Plugin | 実装済み |
| v0.15 | OCI Seed Recommendation | 実装済み |
| v0.16 | OCI Image Deletion | first slice実装済み |
| v0.17 | Docker Compatibility Plugin | foundation / partial |
| v0.18 | OCI Seed Builder & Btrfs/COW | planned |

完全実装済みのproduct progressionは **v0.16まで連続**しています。

Local OCI Registryはroadmap milestoneを予約せず、deferredなoptional infrastructureとして扱います。

## Coreを小さくする

CoreはWorkspace、Environment、Execution、Lease、Policy/Approval/Audit、Base identity、ResourceBudgetなどのprovider-neutral contractを持ちます。

Incus、GitHub、cloud provider、VS Code、Btrfs、OCI、Docker、nerdctlなどの具体物はadapter/plugin側です。

## Base と OCI

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci ...
HACO_PLUGIN_OCI=docker   haco plugin oci ...
```

BaseはEnvironmentのstarting identity、OCIはdeveloper workload toolingです。`HACO_PLUGIN_OCI`未設定でもCoreはcontainerd / nerdctl / Dockerを要求しません。

project-maintained OCI plugin profileではcontainerd + nerdctlを使えます。Docker互換はgenuine Docker CLIとEnvironment-local/on-demand Engineをoptionalに提供します。Host Docker socketは渡しません。

## Network / Git / OCI Seed

- v0.13: `haco-sandbox0` / ACL / profileをHacocoonが管理・検証し、drift時はfail closed。
- v0.14: `haco plugin git fetch` はHost側 `gh auth git-credential` を使い、credentialをSandboxへ渡さない。
- v0.15: OCI usage telemetry / recommendation / top 10% auto promotion。
- v0.16: immutable image identityのdeletion tombstone。
- v0.18: trusted Host acquisition/cache → offline Seed Builder → immutable Seed → normal Incus/Btrfs COW。writable `/var/lib/containerd` は共有しない。
- Optional Local Registry: normal pullやSeed constructionのprerequisiteではなく、実測上必要な場合だけ将来再検討。

See [`18_v0.18_OCI_SEED_AND_COW.ja.md`](18_v0.18_OCI_SEED_AND_COW.ja.md) and [`OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](OPTIONAL_LOCAL_OCI_REGISTRY.ja.md).

## Client interaction

Browser/Web interaction・notificationはclient/adapter層に置きます。VS Code extensionはoptionalなIDE-native UXであり、Core dependencyにはしません。

## Versioning

独立して使える1機能 ≒ 1 minor milestone。security fix / bug fix / hardening / refactor / CI / docsだけでは通常versionを消費しません。
