# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

Status date: 2026-08-29 — v0.9 Per-Agent Sandbox broker implementation pass 後。

このファイルは **現在のコードの事実**を説明するための日本語版です。理想 architecture や互換性保証ではありません。

Hacocoon はまだ **pre-1.0** です。実装済みであることは interface 固定、本番 support、real-provider/client acceptance 済みを意味しません。

| 領域 | 現在の repository reality | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `exec` / `shell` / `delete` | v0.1 | unit / process integration pass。real Incus acceptance は host-dependent |
| Workspace / Lease | canonical Workspace identity、RO/RW lease、RW conflict prevention、stale recovery、process serialization | v0.1-v0.2 | unit / persistence / concurrency / integration pass |
| Incus provider | local default Environment provider | v0.1+ | unit / process pass。real host acceptance pending |
| Client access | status、loopback forward、connection管理、SSH prepare/revoke、public-key hardening | v0.3 | unit / process integration pass。real Incus SSH acceptance pending |
| Policy / Capability | fail-closed allow/deny/require-approval、human security approval、JSONL audit | v0.4 | unit / integration / CLI E2E pass |
| Git / GitHub | Host-side brokered push。broad host credential を Environment に export しない | v0.5 | unit / adversarial / real-git integration / CLI E2E pass |
| Agent / Orchestrator | `haco run`、machine JSON、security event export。DAG/model selection は外部 responsibility | v0.6 | unit / race / integration / CLI E2E pass |
| Environment routing | provider-neutral router | v0.7 | unit pass |
| EC2 provider | S3 staging + SSM、experimental / disabled by default | v0.7 | fake-AWS path pass。real AWS acceptance pending |
| AWS capability | narrow host-side `aws.api` read capability | v0.7 | fake-AWS CLI/integration pass。real AWS pending |
| EBS replacement | adapter-owned replacement/migration。in-place shrink / automatic source deletion なし | v0.7 | fake-AWS integration pass |
| VS Code Client Adapter | separate `haco-vscode` binary。Environment create/reuse -> existing SSH path -> adapter-owned SSH config -> standard Remote-SSH `/workspace` | v0.8 | helper unit coverage added。real VS Code + Incus acceptance pending |
| Windows / WSL bridge | WSL 実行時に Windows user profile を解決し、desktop Client 側 `.ssh` を対象にする | v0.8 | implementation exists。real Windows/WSL acceptance pending |
| Windows / WSL bootstrap | `scripts/bootstrap-windows.ps1` は default/既存の普段使い WSL を選ばず、`Hacocoon` という dedicated WSL 2 instance を create/reuse。Linux dependency setup は `scripts/bootstrap-wsl.sh`、Hacocoon install は既存 `scripts/install.sh` に委譲。無関係な WSL は触らず、`incus-admin` は explicit opt-in | v0.8 | PowerShell / shell syntax は CI 対象。real Windows install/reboot/dedicated WSL/Incus acceptance pending |
| Client Adapter boundary | VS Code / Daintree / JetBrains 等の client-specific behavior を Core に入れない | v0.8 | architecture + separate binary boundary |
| Per-Agent Sandbox Broker | `internal/agenthost` が opaque な外部 Session ID を既存 Environment lifecycle にbind。exact reacquire は idempotent、別 Workspace/access mode への rebind は fail closed | v0.9 | allocation / idempotence / rebind rejection / lookup / release / access mode / path canonicalization / malformed ID の unit coverage |
| Agent control-plane separation | Broker は trusted host-side にあり、Agent を Core 概念へ追加せず、Coding Agent に `haco` 実行を要求しない。raw Session ID も Incus instance 名にしない | v0.9 | repository architecture/test contract。real-host adversarial validation pending |
| VS Code Agent Host / AHP routing | 独立 routing 可能な top-level Agent Session ごとに Environment を割り当て、Agent Host を Workspace の近くで動かす設計 | v0.9 | integration contract 定義済み。real VS Code Agent Host/AHP + Incus E2E acceptance pending |
| CI | Go tests、vet、race、docs consistency、bootstrap syntax、release packaging、host-independent E2E | cross-cutting | v0.9 PR CI pass が merge gate。real provider/client acceptance は別 |

## v0.9 で増えたもの

新しい基盤は次の構造です。

```text
trusted VS Code/AHP integration / trusted client
                 |
          opaque Session ID
                 |
       internal/agenthost Broker
                 |
       existing Environment service
                 |
         Environment provider
                 |
              Incus
```

Agent自身はこのcontrol pathに入りません。Agentから見ると、割り当てられたEnvironment内にWorkspaceと開発ツールがあるだけです。

重要なルールは次です。

- Sessionごとに別Environmentを割り当てる。
- Agent自身に`haco`を叩かせない。
- AgentへIncus socketやHacocoon state/control authorityを渡さない。
- 同じSessionを別Workspaceへ勝手に付け替えない。
- ReleaseはSession bindingから対象Environmentを決め、Clientに任意Environment名を指定させない。
- 並列RW Agentは同じdirectoryを共有せず、通常は別Git worktreeを渡す。

詳細は [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) を参照してください。

## VS Code Agent Host / AHP

v0.9では、VS Code連携の本命をAgent Host / AHP側に置きます。

```text
VS Code Agents window
        |
      AHP/SSH
        |
trusted Hacocoon integration
        |
 session allocation/routing
        |
  +-----+------------------+
  |                        |
Incus A                  Incus B
  |                        |
Agent Host A             Agent Host B
  |                        |
Agent A                  Agent B
```

ただし、現在のrepositoryに入ったのはSession -> Environment Brokerの基盤です。Real VS Code Agent Host/AHPを使ったper-session routingは、対応Hostでend-to-end acceptanceするまで「実証済み」とは扱いません。

Hooksはlifecycle観測やcleanup補助には使えますが、HooksだけをSandbox transportとはみなしません。実際のexecution hostそのものがIncus Environment内にある必要があります。

## Worktreeとの関係

同じrepositoryを複数Agentで同時にwriteする場合、Hacocoonの既存WorkspaceLeaseを緩めません。

```text
repo
  +-- worktree/a -> Incus A -> Agent A
  +-- worktree/b -> Incus B -> Agent B
```

Git worktreeはコード変更の分離、IncusはOS/runtimeのセキュリティ隔離を担当します。worktree単体をSecurity Sandboxとはみなしません。

## 既存v0.8機能は残る

通常の VS Code 利用では引き続き次が使えます。

```bash
haco-vscode open .
```

内部では概念的に:

```text
Workspace
  -> Environment create/reuse
  -> loopback-only SSH prepare
  -> Hacocoon-owned SSH host entry
  -> code --remote ssh-remote+<alias> /workspace
```

終了して Environment を削除する場合:

```bash
haco-vscode delete .
```

Private SSH key は Client 側に残し、Hacocoon の既存 SSH path には public key のみを渡します。

`haco create / exec / shell / delete` と `haco run` も削除しません。Per-Agent Brokerは追加機能です。

## Windows / WSL bootstrap

Windows host の初期 setup 用:

```powershell
.\scripts\bootstrap-windows.ps1
```

これは普段使いの Ubuntu / Debian を流用せず、標準では次の dedicated instance を作ります。

```text
Instance: Hacocoon
Base: Ubuntu-26.04
```

既に `Hacocoon` が存在する場合だけその instance を再利用し、default WSL や最初に見つかった既存 distribution へ fallback しません。

Fresh PC では Windows reboot や Linux user の初回作成が必要なら一度停止し、`wsl -d Hacocoon` で初期設定後に bootstrap を再実行します。

既存の無関係な WSL は user-owned state として扱い、unregister / reset / delete / WSL 1 conversion / default変更は自動で行いません。

Incus administrator 権限は root 相当なので自動付与しません。明示的に許可する場合だけ:

```powershell
.\scripts\bootstrap-windows.ps1 -GrantIncusAdmin
```

を使います。詳細は [`WINDOWS_WSL_BOOTSTRAP.ja.md`](WINDOWS_WSL_BOOTSTRAP.ja.md) を参照してください。

## AI の扱い

Environment 内では Agent を permissive に動かせますが、Host authorityとは分離します。

```text
Agent
  -> isolated Environment      <- broad local freedom
  -> Hacocoon trust boundary
  -> Policy / Capability / Audit
  -> GitHub / AWS / Host
```

v0.9で追加されたのは、AgentがこのEnvironmentを自分で作る仕組みではなく、trusted control planeがAgentの外側から割り当てる仕組みです。

## Orchestrator

Daintree 等との統合は引き続き Hacocoon の上位です。

```text
Daintree
  -> task / worktree / agent orchestration
  -> Workspace
  -> Hacocoon Environment
```

Hacocoon 自体は worktree manager、Agent scheduler、model router にはなりません。

## EC2 の扱い

v0.7 EC2 provider は引き続き **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方を明示しない限り有効化されません。Real AWS / EC2 / SSM / EBS acceptance は pending です。

## Acceptance の区別

次が pass しても real-provider/client acceptance の代替にはなりません。

- unit test
- process-boundary integration
- fake-provider E2E
- race
- vet
- build
- script syntax
- repository CI

Real Incus、Windows dedicated WSL install、Windows/WSL + VS Code Remote-SSH、AWS/EC2/SSM/EBS、VS Code Agent Host/AHP per-session routing はそれぞれ対応環境で確認します。

## Compatibility status

v0.1〜v0.9 のどの実装も、現在の concrete interface が変更されないという約束ではありません。

Breaking Change により CLI / helper binary / state / provider / capability / client-adapter / agent-integration configuration / host bootstrap behavior 等は変更可能です。

Compatibility のために unsafe authority boundary、曖昧な ownership、silent data loss、不要な complexity を残しません。ただし material change は explicit・tested・documented にします。

## Release / tag

この implementation status だけを見て release/tag readiness を判断しません。Specification の acceptance requirement と、その時点の stability level を別途確認します。
