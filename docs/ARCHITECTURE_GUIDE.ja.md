# Hacocoon 日本語 Architecture Guide

この資料は、Hacocoon の architecture と **v0.1〜v0.8** の流れを日本語でまとめた読み物です。

> [!NOTE]
> これは理解しやすさを優先した補助資料です。Exact contract や security-sensitive な判断では、対応する英語版 authoritative document を正本として参照してください。

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
- Development review queue。
- Client 固有 SSH configuration / launch behavior。
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

Incus、AWS、GitHub、VS Code、Daintree などの concrete technology は Core の外側です。本当に必要な seam だけを作り、hypothetical backend のための premature abstraction は増やしません。

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

Incus では Workspace は `/workspace` に接続されます。

## Client Adapter と VS Code

v0.8 で **Client Adapter** を明示的な外部 integration layer として導入します。

最初の実装は `haco-vscode` です。

```bash
haco-vscode open .
```

概念的には:

```text
local Workspace
  -> Hacocoon Environment create/reuse
  -> loopback-only SSH prepare
  -> client-side SSH config
  -> VS Code Remote-SSH
  -> /workspace
```

重要なのは、**VS Code を Hacocoon Core に入れない**ことです。

`haco-vscode` は Remote-SSH を再実装せず、Hacocoon の generic client access を VS Code が理解できる SSH configuration と起動 command に翻訳するだけです。

VS Code Extension を将来作る場合も optional な薄い adapter とします。Environment 作成ボタン、status、notification、security approval UX などは追加できますが、editor、terminal、AI chat、Remote-SSH は再実装しません。

## AI を YOLO させる境界

v0.8 の重要な利用イメージです。

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

Environment 内の package install、build、test、source edit、destructive trial は intentionally permissive にできます。

しかし Environment 内で自由であることと、Host や GitHub/AWS で自由であることは別です。Long-lived host credential、`~/.ssh`、`~/.aws`、Incus control socket 等を便利だからという理由で Environment に渡しません。

## Windows + WSL

Hacocoon/Incus が WSL、desktop VS Code が Windows にある場合、実行 Host と Client の filesystem / SSH context は別です。

```text
Windows VS Code
  -> Windows OpenSSH config / key
  -> 127.0.0.1:<port>
  -> WSL / Hacocoon
  -> Incus Environment:22
```

そのため `haco-vscode` は WSL 実行時に Windows user profile を解決し、Windows 側の `.ssh` configuration を管理します。WSL の `~/.ssh/config` だけを書いても desktop VS Code の Remote-SSH integration には不十分、という境界を adapter が吸収します。

## Orchestrator / Daintree

Daintree 等の Orchestrator は Client Adapter と別の責任です。

```text
Daintree / Orchestrator
  -> task / worktree / agent ownership
  -> Workspace
  -> Hacocoon
  -> Environment
```

Daintree が worktree を作れば、その path を Hacocoon に渡します。Task decomposition、parallelism、retry、model/agent selection、development review は Daintree 側です。

Hacocoon は Environment と security boundary を提供します。

## Policy / Capability / Approval

Environment の外にある authority は explicit Capability として扱います。

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

Git worktree ownership と GitHub authority は別です。Protected Git operation は Host-side broker を通し、broad parent credential を Environment に export しません。

## EC2 / EBS

v0.7 の EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方を明示しない限り有効になりません。Real AWS acceptance は別途必要です。

EBS は in-place shrink できないため、shrink-like request は adapter-owned replacement transaction として扱い、source volume を自動削除しません。

## v0.1〜v0.8

### v0.1 — Secure Workspace Runtime MVP
Host directory -> Incus Environment -> exec/shell -> delete の最小縦切り。

### v0.2 — Workspace Abstraction & Lease
Workspace identity、RO/RW lease、ownership safety。

### v0.3 — Client & Interactive Access
Status、SSH、loopback port forwarding、standard client access。

### v0.4 — Policy & Capability Foundation
Allow / deny / require-approval、Human security approval、Audit。

### v0.5 — Git / GitHub Capability
Host-side brokered Git/GitHub authority。

### v0.6 — Agent & Orchestrator Integration
`haco run`、machine-readable output、security event export。Hacocoon 自体は scheduler にならない。

### v0.7 — Remote / Cloud Runtime & External Capabilities
Experimental EC2、AWS capability、EBS replacement flow。

### v0.8 — Client Adapters & VS Code Integration
Client-specific glue を Core の外に明示し、最初の `haco-vscode` で Environment + SSH + VS Code Remote-SSH を一本につなぐ。AI UI は VS Code 側をそのまま使う。

## Breaking Change

Hacocoon はまだ pre-1.0 です。Compatibility より architecture の小ささ、安全性、責任分界を優先するため、rename / delete / replace / CLI redesign / state change / adapter redesign が起こり得ます。

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
10. IDE / Orchestrator は交換可能な外部 client として扱う。

## 次に読む資料

日本語:

- [`../README.ja.md`](../README.ja.md)
- [`README.ja.md`](README.ja.md)
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)

英語の正本:

- [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
- [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- [`01_v0.1_SECURE_WORKSPACE_RUNTIME.md`](01_v0.1_SECURE_WORKSPACE_RUNTIME.md) 〜 [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md)
