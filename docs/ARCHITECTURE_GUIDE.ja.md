# Hacocoon Architecture Guide

> **日本語で読むためのarchitecture overview**
>
> 現在の実装事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)、milestone番号は [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。Security-sensitiveな判断では英語版authoritative documentを正本とします。

## Hacocoonとは

**Hacocoonは Secure Workspace Runtime です。**

人間、IDE、Shell、Coding Agent、外部OrchestratorからWorkspaceを受け取り、隔離されたEnvironmentで処理を実行し、Hostや外部サービスへ跨ぐauthorityをPolicy / Capability boundaryで制御します。

```text
Client / IDE / Agent / Orchestrator
                |
       optional Client Adapter
                |
             Workspace
                v
        +------------------+
        |     Hacocoon     |
        | Environment      |
        | Execution        |
        | Policy/Approval  |
        | Capability/Audit |
        +--------+---------+
                 |
         Provider / Adapter / Plugin
                 |
 Incus / GitHub / optional OCI tooling / future external adapters
```

現在のEnvironment runtime実装はIncusです。provider-neutral routing seamは維持しますが、以前のconcrete EC2/AWS/EBS implementationはlocal/provider contractが安定するまでdeferredです。

Hacocoon自身はIDE、AI chat product、Git worktree manager、Agent schedulerにはなりません。

## 現在のmilestone

**凡例:** ✅ 実装済み · 🧪 partial/historical · 🚧 planned

| Version | Gate | State |
|---|---|---|
| v0.1〜v0.6 | Core runtime / workspace / access / policy / Git / agent integration | ✅ 実装済み |
| v0.7 | Remote / Cloud Runtime | 🧪 provider routing seam維持。cloud implementation deferred |
| v0.8 | Client Adapters & VS Code | ✅ 実装済み |
| v0.9 | Per-Agent Sandbox | ✅ broker foundation実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ 実装済み |
| v0.11 | Base Images | ✅ first slice実装済み |
| v0.12 | Sandbox Resource Limits | ✅ first slice実装済み |
| v0.13 | Managed Sandbox Network | ✅ 実装済み |
| v0.14 | Git Fetch Plugin | ✅ 実装済み |
| v0.15 | OCI Seed Recommendation | ✅ 実装済み |
| v0.16 | OCI Image Deletion | ✅ first slice実装済み |
| v0.17 | Docker Compatibility Plugin | 🧪 foundation実装済み |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

**完全実装済みのproduct progressionは v0.16 まで連続**しています。

## Versionの考え方

原則は **「独立して使える1機能 ≒ 1 minor milestone」** です。

- 新しい独立機能は次の `v0.N` を取る。
- 同じ機能を完成させる複数PRは同じmilestoneにまとめてよい。
- security fix / bug fix / refactor / CI / docsだけでは通常versionを消費しない。
- 新機能PR自身でversioningとimplementation statusを更新する。

## Coreを小さくする

Core vocabularyは意図的に小さく保ちます。

```text
Workspace
WorkspaceLease
Environment
Execution
CapabilityRequest
PolicyDecision
ApprovalRequest
BaseName / BaseRevision / BaseRef
ResourceBudget
```

Incus、cloud provider、GitHub、VS Code、AHP、Btrfs、OCI registry、Docker/nerdctlなどはprovider / adapter / plugin側に置きます。

## Environmentとauthorityの境界

```text
Coding Agent
     |
     v
Environment                <- broad local freedom
     |
----- trust boundary -----
     |
 Hacocoon
 Policy / Capability
     |
GitHub / external services / Host
```

Host HOME、`~/.ssh`、reusable parent credentials、Incus socket、Hacocoon control stateをshortcutとしてEnvironmentへ渡しません。

## Cloud runtime

v0.7で導入したprovider-neutral routing seamは現役です。一方、以前のEC2 runtime、AWS capability、EBS helperはcurrent implementation treeには含めずdeferredとしています。

これは将来のcloud adapterを否定するものではありません。local runtimeとProvider contractが安定した後、Coreへcloud固有概念を混ぜずadapterとして戻せる境界を維持します。

## Client / Agent

- **v0.8:** `haco-vscode` がgeneric Environment/client-accessをVS Code Remote-SSHへ変換する。
- **v0.9:** trusted opaque session identityをpersisted ownership proof付きでdedicated Environmentへbindする。
- **v0.10:** `haco-agent-host` がv0.9-bound Environmentへのloopback-only SSH接続を準備する。

Coding agent自身にHacocoon/Incus management authorityを渡しません。

## Base と ResourceBudget

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

Logical Baseはcreate時にimmutable revisionへpinされます。ResourceBudgetはCPU / memory / PID / root storageをprovider-neutralに保持し、Incus finite limitはstart前に設定・read-back verifyします。

## v0.13 Managed Sandbox Network

Local Incus EnvironmentはHacocoon-managed `haco-sandbox0` / `haco-sandbox` / `haco-sandbox-egress` substrateを使います。Managed stateの欠落・drift時にbroad/default networkへsilent fallbackしません。

## v0.14 Git Fetch Plugin

```text
haco plugin git fetch <environment> [--remote <remote>]
```

Hostの `gh auth git-credential` をtrusted credential boundaryとして使い、credentialをEnvironmentへコピーしません。

## v0.15 / v0.16 OCI Plugin

```text
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference[@digest]> [--all-environments]
```

- **v0.15:** Environmentごとのlatest OCI snapshot、30日recommendation、上位10%の`auto_promote`。
- **v0.16:** immutable digestを基準にしたimage deletion、manual tombstone、明示的なall-Environment deletion。

Baseは `haco base`、container imageは `haco plugin oci` です。

## v0.17 Docker Compatibility Plugin

標準OCI runtimeは **containerd + nerdctl** のままです。Docker互換はoptional plugin/integrationとして扱います。現在はsystemd packaging/socket activation foundationまで実装済みで、plugin lifecycle/real-host integrationは未完です。

## v0.18 Optional Local OCI Registry

Local Registry/proxyは必須infrastructureではありません。通常のEnvironment `nerdctl pull`、usage telemetry、Seed constructionはRegistryなしでも成立します。

## v0.19 OCI Seed Builder & Btrfs/COW

```text
trusted Host acquisition
       |
       v
Offline Seed Builder
       |
immutable Seed revision
       |
Incus clone / storage-driver COW
       +-- Env A
       +-- Env B
```

複数Environmentで1つのwritable `/var/lib/containerd`を共有してはいけません。各Environmentは独立したlogical containerd stateを持ち、Btrfsのphysical block sharingはIncus/storage driverの通常clone semanticsに任せます。

## Acceptanceの読み方

| Evidence | 意味 |
|---|---|
| unit / adversarial test | logic/invariant coverage |
| process / fake-provider E2E | external infraなしのexecutable integration |
| repository CI | host-independent regression |
| real Incus / Windows / external services | actual provider/client acceptance |

Cloud acceptanceはconcrete cloud adapterがcurrent treeへ戻るまでdeferredです。

## 次に読む資料

- [`README.ja.md`](README.ja.md)
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [`13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.ja.md`](14_v0.14_GIT_FETCH_PLUGIN.ja.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.ja.md`](15_v0.15_OCI_SEED_RECOMMENDATION.ja.md)
- [`16_v0.16_OCI_IMAGE_DELETION.ja.md`](16_v0.16_OCI_IMAGE_DELETION.ja.md)
- [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md)
- [`18_v0.18_LOCAL_OCI_REGISTRY.ja.md`](18_v0.18_LOCAL_OCI_REGISTRY.ja.md)
- [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md)
