# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

Status date: 2026-08-29 — v0.10 per-agent sandbox broker implementation pass 後。v0.11 Resource Limits は design-only、v0.12 Agent Host Adapter は PR #111 の integration slot として扱います。

このファイルは **現在の `main` のコードの事実**を説明する日本語 companion です。roadmap の希望や compatibility guarantee ではありません。厳密な正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。

Hacocoon はまだ **pre-1.0** です。実装済みであっても CLI/API/state/config が固定された、本番 support 済み、real-provider/client acceptance 済み、という意味ではありません。

また、version number と implementation completeness は別です。v0.9 は設計のみですが、独立した v0.10 broker foundation は既に実装されています。番号の割り当ては [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。

## 現在の repository reality

| 領域 | 現在の状態 | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `haco exec` / `haco shell` / `haco delete` | v0.1 | unit / process-boundary integration pass。real Incus acceptance pending |
| Workspace model / Lease | canonical external-path Workspace identity、RO/RW lease、RW conflict prevention、process serialization | v0.1-v0.2 | unit / persistence / concurrency / integration pass |
| Incus Environment provider | local default runtime | v0.1+ | unit/process pass。real supported-host acceptance pending |
| Client access | status、loopback forwarding、connection list/remove、SSH prepare/revoke、SSH key hardening | v0.3 | unit/process integration pass。real Incus SSH acceptance pending |
| Policy / Capability | fail-closed policy、allow/deny/require-approval、human security approval、request correlation、JSONL audit | v0.4 | unit/process integration + CLI E2E pass |
| Git / GitHub Capability | host-side brokered push。host credential を Environment に export しない | v0.5 | unit / adversarial / real-git integration / CLI E2E pass |
| Agent / Orchestrator | `haco run`、stable machine JSON、security event export。DAG/model/retry は外部 responsibility | v0.6 | unit/race/process integration + CLI E2E pass |
| Environment routing | provider-neutral Environment router | v0.7 | unit pass |
| EC2 Environment provider | S3 staging + SSM、experimental / disabled by default | v0.7 | fake-AWS integration/E2E pass。real AWS acceptance pending |
| AWS capability | narrow host-side `aws.api` read capability | v0.7 | unit/process/fake-AWS pass。real AWS pending |
| EBS replacement | adapter-owned replacement/migration。in-place shrink と automatic source deletion はしない | v0.7 | unit/fake-AWS process integration pass |
| VS Code Client Adapter | `haco-vscode` が Environment create/reuse -> loopback SSH -> adapter-owned SSH config -> standard Remote-SSH `/workspace` | v0.8 | helper unit coverage。real Windows/WSL + Incus + VS Code acceptance pending |
| Windows / WSL bridge | dedicated Hacocoon WSL 2、systemd PID 1、Windows desktop SSH config を対象 | v0.8 | static/bootstrap checks。real Windows acceptance pending |
| Client Adapter boundary | VS Code / Daintree / JetBrains 等の client-specific ownership は Core 外 | v0.8 | architecture + separate adapter boundary |
| Base Images & Custom Environments | logical Base、immutable revision、custom-image trust boundary を定義 | v0.9 | **design only / implementation pending**。`haco image` / `haco create --base` は未実装 |
| Per-agent sandbox broker | `internal/agenthost` が opaque external session identity を dedicated Environment に bind | v0.10 | allocation/idempotence/rebinding/persistence/ownership proof の unit coverage |
| Agent binding state | `agent-bindings.json` に session -> Environment ownership proof を trusted state として保存。raw session ID は hash 化 | v0.10 | Linux lock + atomic/fsync-backed writes。real crash/fault acceptance pending |
| Agent control-plane separation | coding agent 自身に `haco` / Incus management authority を渡さない | v0.10 | architecture/test contract。real-host adversarial acceptance pending |
| VS Code Agent Host / AHP routing | Agent Host を assigned Environment 側に置く方向 | v0.10 foundation | real Agent Host/AHP + Incus end-to-end routing acceptance pending |
| Sandbox Resource Limits | CPU / memory / PID / root-storage budget contract | v0.11 | **design only / implementation pending** |
| VS Code Remote Agent Host Adapter | PR #111 の active integration candidate。authoritative numbering は v0.12 | v0.12 | **まだ `main` implementation claim ではない**。merge前に branch docs の renumber/rebase が必要 |
| CI / release hardening | Go matrix、vet、race、docs consistency、bootstrap syntax、release packaging、workflow trust guard、trusted-main release-tag boundary | cross-cutting | repository CI 対象。real provider/client acceptance は別 |

## 現在の実装の流れ

```text
Workspace
  -> Environment lifecycle
  -> local Incus by default
  -> Workspace leases / client access
  -> Policy / Approval / Capability
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> experimental EC2 / AWS capability
  -> VS Code Client Adapter
  -> dedicated Windows/WSL 2 + systemd bootstrap
  -> trusted external agent session -> persisted Environment binding broker
```

## v0.8 VS Code path

```bash
haco-vscode open .
```

概念的には:

```text
local Workspace
  -> create/reuse Hacocoon Environment
  -> prepare loopback-only SSH
  -> create adapter-owned SSH host entry
  -> VS Code Remote-SSH
  -> /workspace
```

Private SSH key は Client 側に残します。

## v0.9 Base Images

v0.9 はまだ実装されていません。

予定概念:

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 は revision A のまま
```

logical Base の更新で既存 Environment を silent retarget しない contract です。

## v0.10 Per-Agent Sandbox

実装済み foundation:

```text
trusted client / integration
        |
 opaque session identity
        |
 internal/agenthost broker
        |
 persisted ownership proof
        |
 existing Environment service
        |
 Incus
```

Deterministic Environment name だけでは ownership proof になりません。persisted binding が一致しなければ Acquire/Release は fail closed します。

Parallel RW agent は別 canonical Workspace、通常は別 Git worktree を使います。

## v0.11 Resource Limits

v0.11 は design-only です。

予定する provider-neutral budget:

```text
CPU
memory
process/PID count
root storage size where safely enforceable
```

Resource limit は Capability とは別です。requested limit を provider が enforce できない場合は silent ignore せず fail closed する設計です。

## v0.12 Agent Host Adapter

PR #111 は VS Code Agents window を v0.10 bound Environment へ standard remote SSH で接続する adapter を実装中です。

ただし、`main` には既に v0.11 Resource Limits が存在するため、PR #111 の version assignment は **v0.12** に統一します。

```text
VS Code Agents window
  -> Remote SSH
  -> Hacocoon-managed loopback alias
  -> v0.10 bound Environment
  -> /workspace
```

VS Code が Agent Host / AHP を所有し、Hacocoon は Environment と安全な connection preparation を所有します。

PRがmergeされるまでは `haco-agent-host` を current `main` implementation として報告しません。

## Windows / WSL

Windows bootstrap は dedicated `Hacocoon` WSL instance を create/reuse し、その instance だけ WSL 2 を保証します。

`systemd` / `systemd-sysv` を installし、`/etc/wsl.conf` の他設定を保持しながら `[boot] systemd=true` を保証します。必要な restart も dedicated instance だけです。

`incus-admin` は root-equivalent authority なので silent grant しません。

## EC2

EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方の explicit opt-in が必要です。

## Acceptance の境界

次が pass しても real-provider/client acceptance の代替にはなりません。

- unit test
- process-boundary integration
- fake-provider E2E
- race detector
- vet
- build
- script syntax
- repository CI

Real Incus、Windows/WSL + VS Code、Agent Host/AHP routing、Base/image lifecycle、AWS/EC2/SSM/EBS、resource enforcement は対応環境で別途確認が必要です。

## Compatibility status

v0.1〜v0.12 の design / implementation / reserved integration slot は、concrete interface の互換性保証ではありません。

pre-1.0 の間は CLI、helper binary、state、provider、Base/image lifecycle、Capability/Policy、client/agent integration、resource-budget behavior、host bootstrap、experimental runtime を breaking change で修正できます。

ただし compatibility freedom を理由に unsafe authority boundary、ambiguous ownership、silent data loss を許容しません。

## Release / tag

この implementation status や roadmap version number だけでは release/tag readiness を判断しません。release workflow の trust boundary、acceptance requirement、real-host validation、stability level を別途確認します。
