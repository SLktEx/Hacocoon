# Hacocoon Architecture Guide

> **日本語で読むためのarchitecture overview**
>
> 現在の実装事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)、milestone番号は [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。

## Hacocoonとは

**Hacocoonは Secure Workspace Runtime です。**

```text
Client / IDE / Agent / Orchestrator
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
             Incus / EC2

optional integrations:
  haco plugin git ...
  haco plugin oci ...
```

Hacocoon自身はIDE、AI chat product、Git worktree manager、Agent scheduler、Docker/nerdctl productにはなりません。

## 現在のstatus

| Version | Gate | State |
|---|---|---|
| v0.1〜v0.6 | Core runtime / workspace / access / policy / Git push / agent integration | ✅ |
| v0.7 | Remote / Cloud Runtime | 🧪 experimental/default-off |
| v0.8 | Client Adapters & VS Code | ✅ |
| v0.9 | Per-Agent Sandbox | ✅ |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ |
| v0.11 | Base Images | ✅ first slice |
| v0.12 | Sandbox Resource Limits | ✅ first slice |
| v0.13 | Managed Sandbox Network | ✅ |
| v0.14 | Git Fetch Plugin | ✅ |
| v0.15 | OCI Seed Usage & Recommendation | ✅ optional-plugin first slice |
| v0.16 | OCI Image Deletion | ✅ optional-plugin first slice |
| v0.17 | Docker Compatibility | ✅ packaging foundation |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

**実装済みmilestoneは v0.17まで連続**しています。

## Hacocoon Coreが所有するもの

- Workspaceの解決・canonical identity
- WorkspaceLease / ownership safety
- isolated Environment lifecycle / cleanup
- command / interactive execution
- client access primitives
- Policy / Approval / Capability / Audit
- trusted agent-session → Environment binding
- provider-neutral Base identity
- provider-neutral ResourceBudget
- genericなnetwork/resource safety invariant

## Coreが所有しないもの

- VS Code / JetBrains等IDE UX
- AI chat UI / model selection / task DAG / retries
- Git branch/worktree orchestration
- ordinary Git UX
- nerdctl / Docker / containerd workflowのuniversal requirement
- OCI Registry infrastructure
- Btrfs固有mechanics

## Environmentとauthority

```text
Coding Agent
     |
     v
Environment                 <- broad local freedom
     |
----- trust boundary -----
     |
 Hacocoon
 Policy / Capability
     |
GitHub / AWS / Host
```

build/test/package install/source editなどEnvironment内部の自由と、Host/external authorityは分けます。Host HOME、`~/.ssh`、`~/.aws`、reusable registry credential、Incus socket、Hacocoon control state、Host Docker socketをshortcutとしてEnvironmentへ渡しません。

## Workspace / Lease

WorkspaceはCoreから見るとopaqueです。directory、Git repository、Git worktree、外部orchestratorが作ったWorkspaceを同じcontractで扱います。Parallel RW sessionは別canonical Workspace、通常は別Git worktreeを使います。

## Client Adapter

最初のadapterは `haco-vscode`。

```text
Workspace
  -> Environment create/reuse
  -> loopback-only SSH
  -> client-side SSH config
  -> VS Code Remote-SSH
  -> /workspace
```

private SSH keyはclient側に残します。editor / terminal / Git UI / AI UIはVS Code側の責任です。

Browser/Web NotificationやrichなInteraction APIはfuture client/adapter workです。VS Code extensionを使う場合もoptionalで、Core transportにはしません。

## Per-Agent Sandbox / Agent Host

opaque session identityをtrusted stateのpersisted binding proofでdedicated Environmentへbindします。raw session IDをownership proofにせず、Coding agent自身へ`haco`/Incus management authorityを渡しません。

`haco-agent-host`はVS Code Agent Host/AHPとHacocoon Environmentの間のthin adapterです。

## BaseとResourceBudget

Baseはlogical nameからimmutable revisionへcreate時にresolveしてEnvironment metadataへpersistします。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

ResourceBudgetはCPU / memory / PID / root storageをprovider-neutralに表現します。Incusのfinite limitはstart前に設定してread-back verificationし、enforceできないrequested limitはsilent ignoreせずfail closedします。

## v0.13 Managed Sandbox Network

local Incus EnvironmentはHacocoon-managed sandbox network/profileを使い、broad/default networkingへsilent fallbackしません。IP/CIDR transport guardとdomain-aware authorizationは別layerです。

## v0.14 Git plugin

```text
haco plugin git fetch <environment>
haco plugin git push <environment> --branch <branch>
```

GitHub向けprivileged operationはHost側でbrokerします。HTTPSではHost所有の`gh auth git-credential`を利用でき、credentialをSandboxへ渡しません。ordinary Git UXはGit自身の責任です。

## v0.15-v0.17 Optional OCI plugin

OCI/container toolingはCore requirementではありません。

```text
HACO_PLUGIN_OCI=nerdctl
# または
HACO_PLUGIN_OCI=docker
```

未設定ならOCI serviceをcomposeしません。

```text
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

project-maintained nerdctl profileではEnvironment-local `containerd + nerdctl`を使えます。Docker profileではgenuine Docker CLIとon-demand Engine compatibilityを提供できます。ただしどちらもHacocoon universal runtimeではありません。

Top-level `haco image` はBase identityのままで、workload OCI image管理とは分離します。

## v0.18 Optional Local Registry — planned

normal pullはdefaultで許可されたupstreamへ直接行えます。Local Registry/proxyは再download cost、rate limit、centralized policyなど必要性があるinstallation向けのoptional infrastructureです。OCI Seedには必須ではありません。

## v0.19 OCI Seed & COW — planned

```text
trusted Host acquisition
   -> OCI export/stream
   -> Offline Seed Builder
   -> immutable Incus Seed
   -> normal Incus clone
      /       \
    Env A     Env B
```

**1つのwritable `/var/lib/containerd`を複数Environmentで共有しません。** 各Environmentのlogical stateは独立し、physical block sharingはIncus/storage driverのCOWへ任せます。

## EC2

v0.7 EC2 providerはexperimental/default-off。disable時にAWS credential lookupやAWS API callを起こしてはいけません。real AWS acceptanceはrepository/fake-AWS testとは別です。

## Acceptance

unit / fake-provider / race / vet / repository CI / local CIと、real Incus / Windows / AWS / OCI tooling / Btrfs acceptanceは分けます。実装済みでもreal-host acceptance pendingの領域があります。

## Breaking Change

pre-1.0ではarchitectureを小さく安全に保つためrename / delete / replace / CLI/state/adapter redesignを許容します。ただしsecurity boundary regression、silent data loss、unsafe destructive operationは許容しません。

## 次に読む資料

- [`README.ja.md`](README.ja.md)
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)
- [`13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.ja.md`](14_v0.14_GIT_FETCH_PLUGIN.ja.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.ja.md`](15_v0.15_OCI_SEED_RECOMMENDATION.ja.md)
- [`16_v0.16_OCI_IMAGE_DELETION.ja.md`](16_v0.16_OCI_IMAGE_DELETION.ja.md)
- [`17_v0.17_DOCKER_COMPATIBILITY.ja.md`](17_v0.17_DOCKER_COMPATIBILITY.ja.md)
- [`18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.ja.md)
- [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md)
