# Hacocoon ドキュメント案内

[English](README.md) | **日本語**

このファイルは Hacocoon の資料を、**現在の `main` の実装状況・roadmap・バージョン番号**に合わせて読むための案内です。

> [!NOTE]
> 日本語資料は読みやすさのための companion です。厳密な設計上の正本は英語版 authoritative document です。

Hacocoon はまだ **pre-1.0** です。version number、spec、実装済みという表現は、API / CLI / state / provider / client adapter / Base / agent integration / resource budget の互換性保証や release readiness を意味しません。

## まず日本語で読むなら

1. [`../README.ja.md`](../README.ja.md) — プロジェクト全体の入口。
2. [`ARCHITECTURE_GUIDE.ja.md`](ARCHITECTURE_GUIDE.ja.md) — architecture / security boundary の説明。
3. [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) — `main` に実際に何が入っているか。
4. [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) — 現在の番号割り当て。
5. [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) — 実装済み per-agent Environment broker。
6. [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) — v0.11 Base Images の詳細設計。
7. [`12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md) — Resource Limits の設計。

## 正本の優先順位

資料同士が矛盾して見える場合、英語版は次の順番で優先します。

1. `00_REBASELINE_AND_ROADMAP.md` — product boundary と roadmap progression。
2. `00D_VERSIONING_AND_RELEASE_STATUS.md` — version number の割り当て。
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

```text
v0.1   Secure Workspace Runtime MVP                implemented
v0.2   Workspace Abstraction & Lease               implemented
v0.3   Client & Interactive Access                 implemented
v0.4   Policy & Capability Foundation              implemented
v0.5   Git / GitHub Capability                     implemented
v0.6   Agent & Orchestrator Integration            implemented
v0.7   Remote / Cloud Runtime                      experimental implementation
v0.8   Client Adapters & VS Code                   implemented
v0.9   Per-Agent Sandbox & Agent Host Integration  broker foundation implemented
v0.10  VS Code Remote Agent Host Adapter           PR #111 active integration
v0.11  Base Images & Custom Environments           design only
v0.12  Sandbox Resource Limits                     design only
```

これで **v0.1〜v0.9 まで実装済み milestone が連番**になりました。v0.10 が次の active implementation、v0.11 / v0.12 は design-only です。

以前の `v0.9 Base / v0.10 Per-Agent / v0.11 Resource Limits / v0.12 Agent Host Adapter` という並びは、実装順が飛び飛びで読みづらかったため pre-1.0 のうちに整理しました。

詳細は [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) を参照してください。

## Specification と Implementation は別

`v0.x` の specification があることと、その機能が `main` に実装済みであることは別です。

- v0.7 EC2: code はあるが real AWS acceptance pending。
- v0.8 `haco-vscode`: code はあるが real Windows/WSL + Incus + VS Code acceptance pending。
- v0.9 per-agent broker: broker foundation 実装済み。real Agent Host/AHP routing acceptance pending。
- v0.10 Agent Host Adapter: PR #111 の active integration。merge 前は `main` 実装と扱わない。
- v0.11 Base Images: **design only / implementation pending**。
- v0.12 Resource Limits: **design only / implementation pending**。

実装の事実は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) を見てください。

## v0.8: VS Code Client Adapter

```bash
haco-vscode open .
```

```text
Workspace
  -> Hacocoon Environment
  -> loopback-only SSH
  -> adapter-owned SSH entry
  -> VS Code Remote-SSH
  -> /workspace
```

VS Code の editor / terminal / Git UI / AI UI は VS Code が所有します。Hacocoon Core に IDE 固有ロジックは持ち込みません。

## v0.9: Per-Agent Sandbox

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

Coding agent 自身に `haco` / Incus 管理 authority を渡す設計ではありません。Parallel RW agent は原則として別 worktree / 別 canonical Workspace を使います。

## v0.10: VS Code Remote Agent Host Adapter

v0.10 は PR #111 の active integration candidate です。まだ `main` implementation claim ではありません。

```text
VS Code Agents window
        |
   Remote SSH
        |
Hacocoon-managed loopback SSH alias
        |
v0.9 bound Environment
        |
 /workspace
```

Hacocoon は Environment と安全な接続準備を所有し、VS Code が Agent Host / Agent Host Protocol を所有します。

## v0.11: Base Images & Custom Environments

v0.11 はまだ設計段階です。

```text
logical Base
   -> immutable Base revision
   -> provider-native starting point
   -> Environment
```

Incus alias / remote / fingerprint は adapter detail であり、Core の public identity にしません。logical Base の更新は新規 Environment だけに反映し、既存 Environment を silent retarget しません。

## v0.12: Sandbox Resource Limits

v0.12 は **設計のみ**です。

予定している resource budget:

- CPU
- memory
- process / PID count
- Environment root storage size（安全に enforce できる provider の場合）

```text
ResourceBudget  -> Environment 内部の消費量を制限
Capability      -> Environment 境界を越える authority を制御
```

requested limit を provider が enforce できない場合は silent ignore せず fail closed する contract です。

## Windows / WSL

Hacocoon は専用 `Hacocoon` WSL 2 instance を使い、systemd を PID 1 として Incus を動かす方向です。

普段使いの WSL distribution や global default を勝手に変更せず、`incus-admin` は explicit opt-in のままです。

Real Windows + WSL 2 + systemd + Incus + VS Code の end-to-end acceptance は対応 host で別途必要です。

## EC2

v0.7 EC2 provider は **experimental / disabled by default** です。

```bash
export HACO_RUNTIME_PROVIDER=runtime.ec2
export HACO_EXPERIMENTAL_EC2=1
```

両方の explicit opt-in が必要です。

## ドキュメント更新ルール

1. product boundary / roadmap を変えるなら authoritative English doc を先に更新する。
2. version number を変更するなら `00D_VERSIONING_AND_RELEASE_STATUS.md` を更新する。
3. code reality が変わったら `IMPLEMENTATION_STATUS.md` / `.ja.md` を更新する。
4. versioned filename、heading、本文、index、docs check、PR metadata、日本語 companion を一緒に揃える。
5. implementation claim と real-provider/client acceptance claim を混ぜない。
6. EC2 の experimental/default-off rule を維持する。
7. `python tools/check_docs.py` を実行する。
8. pre-1.0 の breaking change freedom を security boundary の弱体化に使わない。
