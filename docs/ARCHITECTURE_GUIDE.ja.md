# Hacocoon Architecture Guide

> **日本語で読むためのarchitecture overview**
>
> 現在の実装事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)、milestone番号は [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。

## Hacocoonとは

**Hacocoonは Secure Workspace Runtime です。**

人間、IDE、Shell、Coding Agent、外部OrchestratorからWorkspaceを受け取り、隔離Environmentで処理を実行し、Hostや外部serviceへ跨ぐauthorityをPolicy / Capability / Plugin boundaryで制御します。

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
         Provider / Plugin
                 |
 Incus / EC2 / GitHub / AWS / OCI / Docker compat
```

Hacocoon自身はIDE、AI chat product、Git worktree manager、Agent schedulerにはなりません。

## 現在のstatus

**凡例:** ✅ 実装済み · 🧪 partial/experimental · 🚧 planned

| Version | Gate | State |
|---|---|---|
| v0.1〜v0.6 | Core runtime / workspace / access / policy / Git / agent integration | ✅ |
| v0.7 | Remote / Cloud Runtime | 🧪 experimental。EC2 default-off |
| v0.8 | Client Adapters & VS Code | ✅ |
| v0.9 | Per-Agent Sandbox | ✅ broker foundation |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ |
| v0.11 | Base Images | ✅ first slice |
| v0.12 | Sandbox Resource Limits | ✅ first slice |
| v0.13 | Managed Sandbox Network | ✅ |
| v0.14 | Git Fetch Plugin | ✅ |
| v0.15 | OCI Seed Recommendation | ✅ |
| v0.16 | OCI Image Deletion | ✅ first slice |
| v0.17 | Docker Compatibility Plugin | 🧪 foundation |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

**完全実装済みproduct milestoneはv0.16まで連続**しています。

## Versioningの考え方

Hacocoonはpre-1.0の間、**独立して価値のある1機能につきおおむね1minor milestone**とします。feature PR自身で番号を進め、security/fix/hardening/refactor/CI/docs-onlyでは通常番号を消費しません。

## 責任分界

### Hacocoonが所有するもの

- Workspace canonical identity / lease
- isolated Environment lifecycle / cleanup
- command / interactive execution
- client access primitives
- Policy / Approval / Capability / Audit
- trusted agent-session → Environment binding
- provider-neutral Base identity / ResourceBudget
- managed Incus sandbox network substrate
- authority-sensitive operationのrecovery semantics

### Hacocoon Coreが所有しないもの

- VS Code / JetBrains等IDE UX
- AI chat UI / model selection / token budget
- Agent task DAG / retry strategy
- Git branch/worktree orchestration
- Incus / AWS / OCI / Btrfs / Docker固有mechanics
- GitHub credential transportの具体実装

provider/client/tool固有detailはadapter/pluginへ置きます。

## Coreを小さくする

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

Incus、AWS、GitHub、VS Code、AHP、Btrfs、OCI、Docker等はCore vocabularyへ持ち込みません。

## Environmentとauthority

```text
Coding Agent
     |
 Environment     <- local development freedom
     |
----- trust boundary -----
     |
 Hacocoon / trusted plugins
     |
GitHub / AWS / Host
```

Host HOME、reusable credential、Incus socket、Hacocoon control stateをshortcutとしてEnvironmentへ渡しません。

## v0.8〜v0.12

- **v0.8:** `haco-vscode` がstandard Remote-SSHを使うClient Adapter。private keyはclient-side。
- **v0.9:** opaque session identityをpersisted proof付きdedicated Environmentへbind。deterministic nameはownership proofではない。
- **v0.10:** `haco-agent-host` がv0.9 bindingをVS Code Agents workflowへ接続。VS CodeがAHP behaviorを所有。
- **v0.11:** logical Baseをcreate時にimmutable revisionへpinしてEnvironment metadataへpersist。
- **v0.12:** CPU/memory/PID/root storage ResourceBudget。unsupported finite requestはfail closed。

## v0.13 Managed Sandbox Network

```text
Environment
    |
haco-sandbox profile
    |
haco-sandbox0 + default-deny ACL substrate
```

managed network/profile/ACLをHacocoonが作成・検証し、drift時にIncus `default` へsilent fallbackしません。IP/CIDR transport guardとdomain-aware authorizationは別layerです。

See [`13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md).

## v0.14 Git Fetch Plugin

`haco plugin git fetch` はHost側の `gh auth git-credential` を使い、reusable credentialをEnvironmentへcopyしません。repository-controlled credential helper/transport rewriteをprivileged pathで拒否します。

See [`14_v0.14_GIT_FETCH_PLUGIN.ja.md`](14_v0.14_GIT_FETCH_PLUGIN.ja.md).

## v0.15 OCI Seed Recommendation

`haco image seed sample|recommend` でEnvironmentごとのlatest OCI identity snapshotをtrusted Hostへ保存し、immutable digest付きcandidateをrecommendします。deterministic top 10%をfuture Seedのauto-promotion対象にします。

physical Seed build/publishはまだv0.19です。

See [`15_v0.15_OCI_SEED_RECOMMENDATION.ja.md`](15_v0.15_OCI_SEED_RECOMMENDATION.ja.md).

## v0.16 OCI Image Deletion

`haco image delete` はHost Seed cacheからexact immutable identityを安全に削除し、tombstoneでv0.15のautomatic re-promotionを抑止します。`--all-environments` はexplicitで、forced removalはしません。

See [`16_v0.16_OCI_IMAGE_DELETION.ja.md`](16_v0.16_OCI_IMAGE_DELETION.ja.md).

## v0.17 Docker Compatibility Plugin

標準runtimeは**containerd + nerdctl**です。DockerはDocker CLI/Engine APIを要求するtool向けのoptional compatibility pluginです。

現在はsystemd socket/service packaging foundationまで。`dockerd`をHacocoon標準の常駐runtimeにはしません。

See [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md) と [`OCI_RUNTIME_AND_DOCKER_COMPAT.ja.md`](OCI_RUNTIME_AND_DOCKER_COMPAT.ja.md).

## v0.18 Optional Local OCI Registry — planned

Local Registry/proxyは必須infraではありません。normal `nerdctl pull` はnetwork policyが許せばconfigured upstreamへ直接行けます。rate limit、repeated pull cost、restricted network、central policy pointが必要なinstallationだけoptionalに利用します。

See [`18_v0.18_LOCAL_OCI_REGISTRY.ja.md`](18_v0.18_LOCAL_OCI_REGISTRY.ja.md).

## v0.19 OCI Seed Builder & COW — planned

```text
trusted Host acquisition
      |
OCI export/stream
      v
Offline Seed Builder
      |
immutable Incus Seed
      |
Incus clone / Btrfs COW
      v
independent Environments
```

Seed Builderはgeneral networkなし、preferably NICなし。複数Environmentで同じwritable `/var/lib/containerd` を共有してはいけません。Btrfs/COWはIncus/storage driverのnormal clone semanticsへ任せます。

See [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md).

## Windows + WSL

Hacocoon/IncusがWSL、desktop VS CodeがWindowsの場合はHost/Client filesystemとSSH contextを分離します。dedicated WSL2 + systemdを使い、unrelated distribution/global defaultsを勝手に変更しません。

## Orchestrator boundary

Task decomposition、parallelism、branch/worktree ownership、retry、model selection、development reviewはexternal Orchestrator側です。HacocoonへはWorkspaceとして渡します。

## Approval boundary

```text
Development approval -> Human / GitHub / Orchestrator
Security approval    -> Hacocoon Policy / Capability
```

## EC2

v0.7 EC2 providerは **experimental / disabled by default**。real AWS acceptanceはfake-AWS/repository testと分離します。

## Acceptanceの読み方

| Evidence | 意味 |
|---|---|
| unit / adversarial test | logic/invariant coverage |
| process / fake-provider E2E | external infraなしのintegration |
| repository/local CI | host-independent regression |
| real Incus / Windows / AWS | actual provider/client acceptance |

## Breaking Change

Hacocoonはpre-1.0です。Compatibilityよりarchitectureの小ささ、安全性、責任分界を優先できます。ただしsecurity regression、silent data loss、unsafe destructive operationは許容しません。

## 次に読む資料

- [`README.ja.md`](README.ja.md)
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- [`13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.ja.md`](14_v0.14_GIT_FETCH_PLUGIN.ja.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.ja.md`](15_v0.15_OCI_SEED_RECOMMENDATION.ja.md)
- [`16_v0.16_OCI_IMAGE_DELETION.ja.md`](16_v0.16_OCI_IMAGE_DELETION.ja.md)
- [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md)
- [`18_v0.18_LOCAL_OCI_REGISTRY.ja.md`](18_v0.18_LOCAL_OCI_REGISTRY.ja.md)
- [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md)
