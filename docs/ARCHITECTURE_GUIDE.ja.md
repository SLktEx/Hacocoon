# Hacocoon Architecture Guide

> **日本語で読むためのarchitecture overview**
>
> 現在の実装事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)、milestone番号は [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。Security-sensitiveな判断では英語版authoritative documentを正本とします。

## Hacocoonとは

**Hacocoonは Secure Workspace Runtime です。**

人間、IDE、Shell、Coding Agent、外部OrchestratorからWorkspaceを受け取り、隔離されたEnvironmentの中で処理を実行し、Hostや外部サービスへ跨ぐauthorityをPolicy / Capability boundaryで制御します。

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
         Provider / Adapter
                 |
      Incus / GitHub / External
```

Hacocoon自身はIDE、AI chat product、Git worktree manager、Agent schedulerにはなりません。

## 現在のstatus

**凡例:** ✅ 実装済み · 🧪 experimental/historical · 🚧 planned

| Version | Gate | State |
|---|---|---|
| v0.1〜v0.6 | Core runtime / workspace / access / policy / Git / agent integration | ✅ 実装済み |
| v0.7 | Remote / Cloud Runtime | 🧪 provider routing seamは維持。以前のEC2/AWS/EBS実装はdeferred |
| v0.8 | Client Adapters & VS Code | ✅ 実装済み |
| v0.9 | Per-Agent Sandbox | ✅ broker foundation実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ `haco-agent-host` 実装済み |
| v0.11 | Base Images | ✅ first slice実装済み |
| v0.12 | Sandbox Resource Limits | ✅ first slice実装済み |
| v0.13 | Local OCI Registry | 🚧 planned。`main`には未実装 |
| v0.13A | OCI Seed & Btrfs/COW | 🚧 planned second slice |

**実装済みmilestoneは v0.1〜v0.12 まで連続**しています。

> [!IMPORTANT]
> Specificationがあることと、実装済みであることは別です。特にv0.13/v0.13Aはdesign contractであり、current implementationではありません。

## 責任分界

### Hacocoonが所有するもの

- Workspaceの解決・canonical identity
- isolated Environment lifecycle / cleanup
- command / interactive execution
- Workspace lease / ownership safety
- client access primitives
- Policy / Approval / Capability / Audit
- trusted agent-session → Environment binding
- provider-neutral Base identity
- provider-neutral ResourceBudget
- authority-sensitive operationのrecovery semantics

### Hacocoon Coreが所有しないもの

- VS Code / JetBrains等IDEのUX
- AI chat UI / model selection / token budget
- Agent task DAG / retry strategy
- Git branch strategy / worktree orchestration
- VS Code Agent Host Protocol固有detail
- Incus / cloud / OCI / Btrfs等provider固有mechanics

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

Incus、cloud provider、GitHub、VS Code、AHP、Daintree、Btrfs、OCI registryなどはadapter/integration側に置きます。

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
GitHub / External / Host
```

Environment内ではbuild/test/package install/source editなどを自由にできます。しかしHost authorityまで自由になるわけではありません。

以下をshortcutとしてEnvironmentへ渡しません。

- host HOME
- `~/.ssh`
- `~/.aws`
- reusable GitHub/cloud/registry credentials
- Incus control socket
- Hacocoon control state

## Workspace / Lease

WorkspaceはCoreから見るとopaqueです。通常directory、Git repository、Git worktree、外部Orchestratorが作ったWorkspaceを同じcontractで扱います。

Parallel RW sessionは原則として別canonical Workspace、通常は別Git worktreeを使います。worktree ownership自体はHacocoon Coreの責任ではありません。

## v0.8 Client Adapter

最初のadapterは `haco-vscode` です。

```bash
haco-vscode open .
```

```text
Workspace
  -> Environment create/reuse
  -> loopback-only SSH
  -> client-side SSH config
  -> VS Code Remote-SSH
  -> /workspace
```

Private SSH keyはclient側に残します。VS Codeのeditor / terminal / Git UI / AI UIはVS Code側の責任です。

## v0.9 Per-Agent Sandbox

```text
trusted client
     |
opaque session identity
     |
persisted binding proof
     |
dedicated Environment
```

重要なrule:

- raw session IDをruntime nameやownership proofにしない
- exact reacquireのみidempotent
- Workspace/access-mode rebindはfail closed
- releaseはpersisted ownership proofを要求
- deterministic Environment nameだけでadopt/deleteしない
- Coding agent自身にHacocoon/Incus管理authorityを渡さない

## v0.10 VS Code Remote Agent Host Adapter

`haco-agent-host` は `main` に実装済みです。

```text
VS Code Agents window
        |
    Remote SSH
        |
  haco-agent-host
        |
 v0.9-bound Environment
        |
    /workspace
```

Clientがprivate keyを保持し、HacocoonはEnvironment選択と安全なconnection preparationを所有します。VS CodeがAgent Host/AHP behaviorを所有します。

## v0.11 Base Images

```text
logical Base
    |
provider source
    |
resolve once
    v
immutable revision
    |
Environment
```

first sliceでは次を実装済みです。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

Mutable alias/sourceをcreate時にimmutable revisionへ解決し、Environment metadataにpersistします。Custom build/import/history/rollback/GCはfollow-upです。

## v0.12 ResourceBudget

```text
Environment
  +-- CPU
  +-- memory
  +-- PID/process
  +-- root storage
```

ResourceBudgetはCapabilityとは別です。

```text
ResourceBudget -> Environment内部の消費上限
Capability     -> Environment境界を跨ぐauthority
```

Incusではfinite limitを`start`前に設定し、read-back verificationします。requested finite limitをenforceできないproviderはfail closedします。

## Managed Incus Network

Local Incus EnvironmentはHacocoon-managed sandbox network/profileを使い、broad/default networkingへsilent fallbackしない方向です。

```text
Environment
    |
managed haco-sandbox network/profile
    |
default-deny transport boundary
    |
higher-level policy / broker
```

IP/CIDR levelのtransport guardと、hostname/domain-aware authorizationは別レイヤです。Domain-aware egressは上位broker/policyで扱います。

## v0.13 Local OCI Registry — planned

v0.13は **未実装** です。

目的は、Environment内のordinary `nerdctl pull` / `containerd` image resolutionをHacocoon Local OCI Registry/cache gatewayへtransparentに寄せることです。

```text
Environment containerd
       |
       v
Hacocoon Local Registry
       |
 trusted upstream path
       |
 allowed OCI registry
```

Reusable upstream credentialはtrusted sideに残し、local registryが必要なmodeではdirect registry fallbackを許可しません。

## v0.13A OCI Seed & COW — planned

Local Registryの次のoptimization sliceです。

```text
pinned Base
   |
Seed Builder
   |
OCI images by immutable digest
   |
publish immutable Incus Seed
   |
normal Incus clone
   +---- Env A
   +---- Env B
   +---- Env C
```

**1つのwritable `/var/lib/containerd` を複数Environmentで共有してはいけません。** 各Environmentは独立したlogical containerd stateを持ち、Btrfs/COWのphysical block sharingはIncus/storage driverに任せます。

## Windows + WSL

Hacocoon/IncusがWSL、desktop VS CodeがWindowsの場合、HostとClientのfilesystem/SSH contextは別です。

Hacocoonはdedicated WSL 2 instanceを使い、systemdをPID 1としてIncusを動かします。Unrelated WSL distribution/global defaultを勝手に変更せず、`incus-admin`はexplicit opt-inのままです。

## Orchestrator

```text
Daintree / other orchestrator
  -> task / branch / worktree / agent ownership
  -> Workspace
  -> Hacocoon
  -> Environment
```

Task decomposition、parallelism、retry、model selection、development reviewはOrchestrator側です。

## Approval boundary

Development approvalとSecurity approvalは分けます。

```text
Development approval -> Human / GitHub / Orchestrator
Security approval    -> Hacocoon Policy / Capability
```

## Cloud Runtime

Remote / Cloud Runtimeは現在 **deferred** です。Current buildが登録するEnvironment ProviderはIncusのみで、以前のEC2 runtime、AWS capability、EBS helperはactive implementation treeにありません。

以前のv0.7 EC2 providerは **experimental / disabled by default** でした。歴史的には次のexplicit opt-inを使っていましたが、現在のsupported runtime configurationではありません。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Provider-neutral routing seamとGit history / v0.7 design contractは残しています。Local側のcontractが安定した後にcloud adapterを復活させる場合も、explicit opt-in、trusted-side credential、provider side effect前のfail-closedを維持します。

## Acceptanceの読み方

| Evidence | 意味 |
|---|---|
| unit / adversarial test | logic/invariant coverage |
| process / fake-provider E2E | external infraなしのexecutable integration |
| repository CI | host-independent regression |
| real Incus / Windows / future cloud | actual provider/client acceptance |

実装済みでもreal-host acceptance pendingの領域があります。Cloud acceptanceはadapter再導入時に改めて定義します。詳細は `IMPLEMENTATION_STATUS.ja.md` を参照してください。

## Breaking Change

Hacocoonはpre-1.0です。Compatibilityよりarchitectureの小ささ、安全性、責任分界を優先するため、rename / delete / replace / CLI redesign / state change / adapter redesignが起こり得ます。

ただしsecurity boundary regression、silent data loss、unsafe destructive operationは許容しません。

## 次に読む資料

日本語:

- [`../README.ja.md`](../README.ja.md)
- [`README.ja.md`](README.ja.md)
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md)
- [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md)
- [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md)
- [`12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md)
- [`13A_v0.13_OCI_SEED_AND_COW.ja.md`](13A_v0.13_OCI_SEED_AND_COW.ja.md)

英語の正本:

- [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
- [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
- [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
