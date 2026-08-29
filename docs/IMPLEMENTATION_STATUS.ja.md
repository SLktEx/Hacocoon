# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

Status date: 2026-08-30 — v0.10 Agent Host adapter merge / v0.11 Base first-slice 実装後。

このファイルは **現在の `main` のコードの事実**を説明する日本語 companion です。roadmap の希望や compatibility guarantee ではありません。厳密な正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。

Hacocoon はまだ **pre-1.0** です。実装済みであっても CLI/API/state/config が固定された、本番 support 済み、real-provider/client acceptance 済み、という意味ではありません。

現在は **v0.1〜v0.11 を実装済み milestone として連番**にしています。v0.12 Resource Limits は design-only です。

## 現在の repository reality

| 領域 | 現在の状態 | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `haco exec` / `haco shell` / `haco delete` | v0.1 | unit / process-boundary integration pass。real Incus acceptance pending |
| Workspace model / Lease | canonical Workspace identity、RO/RW lease、RW conflict prevention、process serialization | v0.1-v0.2 | unit / persistence / concurrency / integration pass |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 | unit/process pass。real Incus SSH acceptance pending |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 | unit/process + CLI E2E pass |
| Git / GitHub Capability | host-side brokered push。host credential を Environment に export しない | v0.5 | unit / adversarial / real-git / CLI E2E pass |
| Agent / Orchestrator | `haco run`、stable machine JSON、security event export | v0.6 | unit/race/process + CLI E2E pass |
| EC2 Environment / AWS | experimental / disabled by default | v0.7 | fake-AWS integration/E2E pass。real AWS acceptance pending |
| VS Code Client Adapter | `haco-vscode` -> loopback SSH -> standard Remote-SSH `/workspace` | v0.8 | helper unit。real Windows/WSL + Incus + VS Code acceptance pending |
| Per-agent sandbox broker | `internal/agenthost` が opaque session identity を dedicated Environment に bind | v0.9 | ownership / persistence / collision / release proof unit coverage |
| Agent binding state | `agent-bindings.json` に ownership proof を trusted state として保存。raw session ID は hash 化 | v0.9 | lock + atomic/fsync-backed writes |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release`。hashed alias、loopback SSH、client-side private key、`code --agents` | v0.10 | PR #137 で `main` 実装済み。real Agent Host acceptance pending |
| Base identity | `BaseName` / `BaseRevision` / `BaseRef` を provider-neutral に保持し Environment に保存 | v0.11 | unit / routing / fake-Incus E2E |
| Incus Base pinning | logical Base source を create 時に immutable fingerprint へ解決し、mutable alias ではなく pinned fingerprint から init | v0.11 | alias movement / malformed fingerprint / injection 系 unit coverage。real Incus image acceptance pending |
| Base CLI | `haco image list` / `haco image inspect <base>` / `haco create --base <base> ...` | v0.11 | CLI parse + fake-Incus E2E。status に persisted Base revision を出力 |
| Custom Base mapping | `HACO_INCUS_BASES_JSON` で host/operator が logical mapping を追加。`haco/` namespace は予約 | v0.11 | adversarial input test。build/import/history/rollback/GC は未実装 |
| Sandbox Resource Limits | CPU / memory / PID / root-storage budget contract | v0.12 | **design only / implementation pending** |
| CI / release hardening | Go matrix、vet、race、docs consistency、bootstrap/release/workflow trust checks | cross-cutting | real provider/client acceptance は別 |

## 現在の実装の流れ

```text
Workspace
  -> Environment lifecycle
  -> Workspace leases / client access
  -> Policy / Approval / Capability
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> experimental EC2 / AWS
  -> VS Code Client Adapter
  -> trusted session -> Environment binding broker
  -> VS Code Remote Agent Host adapter
  -> logical Base -> immutable revision -> Environment
```

## v0.9 Per-Agent Sandbox

Deterministic Environment name だけでは ownership proof になりません。persisted binding が一致しなければ Acquire/Release は fail closed します。

Parallel RW agent は別 canonical Workspace、通常は別 Git worktree を使います。

## v0.10 Agent Host Adapter

```text
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

VS Code が Agent Host / AHP を所有し、Hacocoon は Environment ownership と安全な connection preparation を所有します。coding agent に `haco` / Incus management authority を渡しません。

real Windows/WSL + Incus + VS Code Agents window acceptance は host-dependent です。

## v0.11 Base Images

実装された first slice:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

```text
my-dev -> revision A -> Environment 1
my-dev -> revision B -> Environment 2
Environment 1 の persisted identity は revision A のまま
```

Incus alias / remote / fingerprint は adapter detail です。custom build/import、revision history、rollback、GC は first slice では未実装です。

## v0.12 Resource Limits

v0.12 は design-only です。予定する provider-neutral budget は CPU、memory、process/PID count、safely enforceable な root storage です。

requested limit を provider が enforce できない場合は silent ignore せず fail closed する設計です。

## Acceptance の境界

unit test、fake-provider E2E、race、vet、build、repository CI は real-provider/client acceptance の代替ではありません。

Real Incus、Windows/WSL + VS Code、v0.9/v0.10 Agent Host routing、v0.11 real image sources、AWS/EC2/SSM/EBS、v0.12 resource enforcement は対応環境で別途確認が必要です。

## Compatibility status

pre-1.0 の間は CLI、helper binary、state、provider、Base/image lifecycle、Capability/Policy、client/agent integration、resource-budget behavior、host bootstrap、experimental runtime を Breaking Change で修正できます。

ただし compatibility freedom を理由に unsafe authority boundary、ambiguous ownership、silent data loss を許容しません。
