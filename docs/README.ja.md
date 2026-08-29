# Hacocoon ドキュメント案内

[English](README.md) | **日本語**

このファイルは、2026-08-29 時点の Hacocoon の資料を、**現在の `main` の実装状況・roadmap・バージョン番号**に合わせて読むための案内です。

> [!NOTE]
> 日本語資料は読みやすさのための companion です。厳密な設計上の正本は英語版 authoritative document です。

Hacocoon はまだ **pre-1.0** です。version number、spec、実装済みという表現は、API / CLI / state / provider / client adapter / Base / agent integration / resource budget の互換性保証や release readiness を意味しません。

## まず日本語で読むなら

1. [`../README.ja.md`](../README.ja.md) — プロジェクト全体の入口。
2. [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md) — architecture / security boundary の説明。
3. [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) — `main` に実際に何が入っているか。
4. [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) — v0.11/v0.12 の衝突を含む、現在の番号割り当て。
5. [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) — v0.9 Base Images の詳細設計。
6. [`10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](10_v0.10_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) — per-agent Environment broker。
7. [`11_v0.11_SANDBOX_RESOURCE_LIMITS.ja.md`](11_v0.11_SANDBOX_RESOURCE_LIMITS.ja.md) — Resource Limits の設計。

## 正本の優先順位

資料同士が矛盾して見える場合、英語版は次の順番で優先します。

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary と roadmap progression。
2. `00D_VERSIONING_AND_RELEASE_STATUS.md` — version number の割り当てと衝突解消ルール。
3. `00C_TERMINOLOGY_AND_BOUNDARIES.md` — architecture vocabulary。
4. `00B_SECURITY_ARCHITECTURE.md` — trust / security の横断ルール。
5. `IMPLEMENTATION_STATUS.md` — **現在の code reality**。
6. 各 versioned specification。
7. `CLIENT_ACCESS.md` / `REMOTE_CLOUD_PROVISIONING.md` / `BASE_IMAGES.md` 等の詳細資料。
8. `00A_PLUGIN_ARCHITECTURE.md`。
9. `90_CODEX_IMPLEMENTATION_HANDOFF.md`。
10. `91_IMPLEMENTATION_REFERENCE_NOTES.md`。
11. `adr/` 配下。

## 現在のバージョン整理

現在の authoritative numbering は次です。

```text
v0.1   Secure Workspace Runtime MVP                implemented
v0.2   Workspace Abstraction & Lease               implemented
v0.3   Client & Interactive Access                 implemented
v0.4   Policy & Capability Foundation              implemented
v0.5   Git / GitHub Capability                     implemented
v0.6   Agent & Orchestrator Integration            implemented
v0.7   Remote / Cloud Runtime                      experimental implementation
v0.8   Client Adapters & VS Code                   implemented
v0.9   Base Images & Custom Environments           design only
v0.10  Per-Agent Sandbox & Agent Host Integration  broker foundation implemented
v0.11  Sandbox Resource Limits                     design only
v0.12  VS Code Remote Agent Host Adapter           PR #111 integration slot
```

特に **v0.11 は Resource Limits** です。PR #111 が当初 `v0.11 Agent Host Adapter` を名乗っていたため番号が衝突しましたが、`main` に先に入っている v0.11 を維持し、PR #111 側を **v0.12** に変更する方針で統一します。

詳細は [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。

## Specification と Implementation は別

`v0.x` の specification があることと、その機能が `main` に実装済みであることは別です。

代表例:

- v0.7 EC2: code はあるが real AWS acceptance pending。
- v0.8 `haco-vscode`: code はあるが real Windows/WSL + Incus + VS Code acceptance pending。
- v0.9 Base Images: **design only / implementation pending**。
- v0.10 per-agent broker: broker foundation は実装済みだが real VS Code Agent Host/AHP routing acceptance pending。
- v0.11 Resource Limits: **design only / implementation pending**。
- v0.12 Agent Host adapter: PR #111 の integration slot であり、merge 前は `main` 実装と扱わない。

実装の事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) を見てください。

## v0.8: VS Code Client Adapter

通常の VS Code path は次です。

```bash
haco-vscode open .
```

概念的には:

```text
Workspace
  -> Hacocoon Environment
  -> loopback-only SSH
  -> adapter-owned SSH entry
  -> VS Code Remote-SSH
  -> /workspace
```

VS Code の editor / terminal / Git UI / AI UI は VS Code が所有します。Hacocoon Core に IDE 固有ロジックは持ち込みません。

## v0.9: Base Images & Custom Environments

v0.9 はまだ設計段階です。

```text
logical Base
   -> immutable Base revision
   -> provider-native starting point
   -> Environment
```

Incus alias / remote / fingerprint は adapter detail であり、Core の public identity にしません。

logical Base の更新は新規 Environment だけに反映し、既存 Environment を silent retarget しません。

## v0.10: Per-Agent Sandbox

v0.10 は trusted integration が外部 session identity を dedicated Environment に結びつける broker foundation を実装しています。

```text
trusted VS Code / client
        |
 opaque session ID
        |
 internal/agenthost broker
        |
 persisted binding proof
        |
 Environment
        |
 Incus
```

Coding agent 自身に `haco` / Incus 管理 authority を渡す設計ではありません。

Parallel RW agent は原則として別 worktree / 別 canonical Workspace を使います。

## v0.11: Sandbox Resource Limits

v0.11 は **設計のみ**です。

予定している resource budget:

- CPU
- memory
- process / PID count
- Environment root storage size（安全に enforce できる provider の場合）

Resource budget は Capability ではありません。

```text
ResourceBudget  -> Environment 内部の消費量を制限
Capability      -> Environment 境界を越える authority を制御
```

requested limit を provider が enforce できない場合は silent ignore せず fail closed する contract です。

## v0.12: VS Code Remote Agent Host Adapter

v0.12 は PR #111 の Agent Host adapter 用に予約しています。まだ `main` implementation claim ではありません。

目標形:

```text
VS Code Agents window
        |
   Remote -> SSH
        |
Hacocoon-managed loopback SSH alias
        |
v0.10 bound Environment
        |
 /workspace
```

Hacocoon は Environment と安全な接続準備を所有し、VS Code が Agent Host / Agent Host Protocol を所有します。

## Windows / WSL

Hacocoon は専用 `Hacocoon` WSL 2 instance を使い、systemd を PID 1 として Incus を動かす方向です。

普段使いの WSL distribution や global default を勝手に変更せず、`incus-admin` は explicit opt-in のままです。

Real Windows + WSL 2 + systemd + Incus + VS Code の end-to-end acceptance は対応 host で別途必要です。

## Orchestrator

Daintree 等は Hacocoon の上位です。

```text
Daintree / other orchestrator
  -> task / worktree / model / retry
  -> Workspace
  -> Hacocoon Environment
```

Hacocoon 自体は AI scheduler / model router / DAG engine にはなりません。

## EC2

v0.7 EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方の explicit opt-in が必要です。Real AWS / EC2 / SSM / EBS acceptance は pending です。

## ドキュメント更新ルール

1. product boundary / roadmap を変えるなら authoritative English doc を先に更新する。
2. version number を新規割り当て・変更・衝突解消するなら `00D_VERSIONING_AND_RELEASE_STATUS.md` を更新する。
3. code reality が変わったら `IMPLEMENTATION_STATUS.md` / `.ja.md` を更新する。
4. versioned filename、heading、本文、index、docs check、PR metadata、日本語 companion を一緒に揃える。
5. implementation claim と real-provider/client acceptance claim を混ぜない。
6. EC2 の experimental/default-off rule を維持する。
7. `python tools/check_docs.py` を実行する。
8. pre-1.0 の breaking change freedom を security boundary の弱体化に使わない。
