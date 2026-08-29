# Hacocoon 日本語 Architecture Guide

この資料は、Hacocoon の architecture と **現在の v0.1〜v0.12 roadmap** を日本語でまとめた補助資料です。

> [!NOTE]
> Exact contract や security-sensitive な判断では、対応する英語版 authoritative document を正本として参照してください。現在の番号は `00D_VERSIONING_AND_RELEASE_STATUS.md` が正本です。

## 一言でいうと

**Hacocoon は Secure Workspace Runtime です。**

人間、IDE、Shell、Coding Agent、外部 Orchestrator から Workspace を受け取り、隔離された Environment の中で処理を実行し、Host や外部サービスに対する特権操作を明示的な Policy / Capability boundary で制御します。

```text
Client / IDE / Agent / Orchestrator
                |
       optional Client Adapter
                |
             Workspace
                |
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
       Incus / EC2 / GitHub / AWS ...
```

Hacocoon 自身は IDE、AI chat product、Git worktree manager、Agent scheduler にはなりません。

## 責任分界

Hacocoon が所有するもの:

- Workspace を Environment に安全に結びつけること。
- Environment lifecycle / cleanup。
- Command execution / interactive access。
- Workspace の read/write ownership safety。
- Host authority を跨ぐ Policy / Capability / Approval。
- Privileged operation の audit。
- Client / Orchestrator が利用できる汎用的な境界。

Hacocoon Core が所有しないもの:

- VS Code など IDE の UX。
- AI chat UI / model selection。
- Git branch strategy / worktree orchestration。
- Agent task DAG / retry / budget。
- Client 固有 SSH configuration / launch behavior。
- Agent Host Protocol 固有 detail。
- Provider 固有 Cloud / Storage mechanics。

## Core を小さくする

Core の語彙は小さく保ちます。

```text
Workspace
WorkspaceLease
Environment
Execution
CapabilityRequest
PolicyDecision
ApprovalRequest
```

Incus、AWS、GitHub、VS Code、AHP、Daintree などの concrete technology は Core の外側です。本当に必要な seam だけを作り、hypothetical backend のための premature abstraction は増やしません。

## Workspace / Lease / Environment

Workspace は Core から見ると opaque な作業対象です。普通の directory、Git repository、Git worktree、外部 Orchestrator が作った workspace のどれでも構いません。

WorkspaceLease は同じ Workspace の書き込み競合を防ぎます。

- RO: read-only
- RW: read-write

Environment は Workspace を実際に動かす隔離場所です。標準 provider は local Incus、EC2 は experimental です。

```text
Workspace
   |
   v
Environment
   +-- runtime.incus   <- default
   +-- runtime.ec2     <- experimental
```

## v0.8 Client Adapter と VS Code

最初の Client Adapter は `haco-vscode` です。

```bash
haco-vscode open .
```

```text
local Workspace
  -> Hacocoon Environment create/reuse
  -> loopback-only SSH prepare
  -> client-side SSH config
  -> VS Code Remote-SSH
  -> /workspace
```

重要なのは、**VS Code を Hacocoon Core に入れない**ことです。Editor、terminal、Git UI、AI chat、Remote-SSH は VS Code が所有します。

## AI を YOLO させる境界

```text
VS Code AI / Codex / Copilot / Claude / other agent
                         |
                         v
                 Incus Environment
                 broad local freedom
                         |
              ---- trust boundary ----
                         |
                     Hacocoon
             Policy / Capability / Audit
                         |
              GitHub / AWS / Host / etc.
```

Environment 内では package install、build、test、source edit、destructive trial を permissive にできます。しかし Host や GitHub/AWS で自由になるわけではありません。

Long-lived host credential、`~/.ssh`、`~/.aws`、Incus control socket 等を便利だからという理由で Environment に渡しません。

## v0.9 Per-Agent Sandbox

v0.9 は external Agent Session identity を dedicated Environment に bind する trusted broker foundation です。

```text
trusted client / VS Code integration
              |
       opaque session identity
              |
       internal/agenthost
              |
     persisted binding proof
              |
          Environment
              |
            Incus
```

Coding agent 自身は management path に入りません。

重要なルール:

- raw session ID を runtime name / ownership proof にしない。
- exact reacquire は idempotent。
- Workspace/access mode の rebind は fail closed。
- Release は persisted binding proof を要求する。
- deterministic Environment name だけで他の Environment を adopt/delete しない。
- Parallel RW session は通常、別 Git worktree / 別 canonical Workspace を使う。

## v0.10 VS Code Remote Agent Host Adapter

v0.10 は PR #111 の active integration candidate です。まだ `main` implementation claim ではありません。

```text
VS Code Agents window
        |
    Remote SSH
        |
Hacocoon-managed loopback alias
        |
 v0.9-bound Environment
        |
 /workspace + Agent Host
```

Private SSH key は Client 側に残し、Hacocoon には public key だけを渡します。VS Code が Agent Host / AHP を所有し、Hacocoon は Environment と安全な connection preparation を所有します。

## v0.11 Base Images & Custom Environments

v0.11 は design-only です。

```text
logical Base name
      |
      v
immutable Base revision
      |
      v
Environment
```

Incus provider では Base revision を fingerprint に内部 mapping できますが、alias / remote / fingerprint を Core public vocabulary にしません。

Logical Base を更新しても既存 Environment は silent retarget しません。

```text
Environment 1 -> revision A
my-dev        -> revision B
Environment 2 -> revision B
```

Custom Base は untrusted guest contents です。Image metadata だけで Host mount、device、privileged mode、credential、external authority を追加できません。

## v0.12 Sandbox Resource Limits

v0.12 は design-only で、Environment に provider-neutral ResourceBudget を持たせます。

```text
Environment
  +-- CPU ceiling
  +-- memory ceiling
  +-- PID/process ceiling
  +-- root-storage ceiling where enforceable
```

ResourceBudget は Capability とは別です。

```text
ResourceBudget -> sandbox 内の消費上限
Capability     -> sandbox 外へ跨ぐ authority
```

requested limit を provider が enforce できない場合は silent ignore せず fail closed します。Resource setting は client/agent access より前に適用します。

Base metadata から limit を上げたり、Agent が自分の limit を management API 経由で上げたりできないようにします。

## Windows + WSL

Hacocoon/Incus が WSL、desktop VS Code が Windows にある場合、実行 Host と Client の filesystem / SSH context は別です。

```text
Windows VS Code
  -> Windows OpenSSH config / key
  -> 127.0.0.1:<port>
  -> WSL / Hacocoon
  -> Incus Environment:22
```

Hacocoon は dedicated WSL 2 instance を使い、systemd を PID 1 として Incus を動かします。Unrelated WSL distributions/global default を勝手に変更せず、`incus-admin` は explicit opt-in のままです。

## Orchestrator / Daintree

```text
Daintree / Orchestrator
  -> task / worktree / agent ownership
  -> Workspace
  -> Hacocoon
  -> Environment
```

Task decomposition、parallelism、retry、model/agent selection、development review は Orchestrator 側です。Hacocoon は Environment と security boundary を提供します。

## Policy / Capability / Approval

```text
CapabilityRequest
       |
       v
PolicyEvaluator
       +-- allow
       +-- deny
       +-- require-approval
                         |
                    Human approval
                         |
                 Capability provider
                         |
                       Audit
```

Development approval と Security approval は分けます。

```text
Development approval -> Human / GitHub / Orchestrator
Security approval    -> Hacocoon Policy / Capability
```

## Git / GitHub

Protected Git operation は Host-side broker を通し、broad parent credential を Environment に export しません。

## EC2 / EBS

v0.7 EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

Real AWS acceptance は別途必要です。

## 現在の roadmap

```text
v0.1  Secure Workspace Runtime MVP                         implemented
v0.2  Workspace Abstraction & Lease                        implemented
v0.3  Client & Interactive Access                          implemented
v0.4  Policy & Capability Foundation                       implemented
v0.5  Git / GitHub Capability                              implemented
v0.6  Agent & Orchestrator Integration                     implemented
v0.7  Remote / Cloud Runtime & External Capabilities       experimental implementation
v0.8  Client Adapters & VS Code Integration                implemented
v0.9  Per-Agent Sandbox & Agent Host Integration           broker foundation implemented
v0.10 VS Code Remote Agent Host Adapter                    active PR #111
v0.11 Base Images & Custom Environments                    design only
v0.12 Sandbox Resource Limits                              design only
```

実装済み milestone は v0.1〜v0.9 まで連番です。

## Breaking Change

Hacocoon はまだ pre-1.0 です。Compatibility より architecture の小ささ、安全性、責任分界を優先するため、rename / delete / replace / CLI redesign / state change / adapter redesign / roadmap renumbering が起こり得ます。

ただし security boundary regression、silent data loss、unsafe destructive operation は許容しません。Partial failure / retry / recovery を正常系と同じく設計対象にします。

## 開発時の判断基準

1. Core を小さくする。
2. Responsibility boundary を明確にする。
3. Provider / Client detail を Core に漏らさない。
4. Premature abstraction を作らない。
5. Host authority を Environment に渡さない。
6. Security-sensitive operation は fail closed。
7. Cleanup / retry / partial failure を重要視する。
8. Environment 内の自由と Host authority を混同しない。
9. 実装の存在と real-provider/client acceptance を混同しない。
10. Version assignment と implementation reality を別々の正本で管理する。

## 次に読む資料

日本語:

- [`../README.ja.md`](../README.ja.md)
- [`README.ja.md`](README.ja.md)
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md)
- [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md)
- [`12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md)

英語の正本:

- [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
- [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
- [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
