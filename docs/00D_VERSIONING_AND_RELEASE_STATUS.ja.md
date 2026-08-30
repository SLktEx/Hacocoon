# バージョン番号とリリース状況

[English](00D_VERSIONING_AND_RELEASE_STATUS.md) | **日本語**

**Status:** バージョン番号・進行状況の日本語案内  
**Date:** 2026-08-30  
**Compatibility:** Hacocoon は pre-1.0 です。

> [!NOTE]
> 厳密な正本は英語版 `00D_VERSIONING_AND_RELEASE_STATUS.md` です。

## 方針

pre-1.0 の間は、バージョン番号をなるべく **実装された順番として読める形** に保ちます。実装の事実は `IMPLEMENTATION_STATUS.md`、roadmap の意図は `00_REBASELINE_AND_ROADMAP.md` を正本として確認します。

## 現在の番号

| Version | Gate | `main` の状況 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | 実装済み |
| v0.2 | Workspace Abstraction & Lease | 実装済み |
| v0.3 | Client & Interactive Access | 実装済み |
| v0.4 | Policy & Capability Foundation | 実装済み |
| v0.5 | Git / GitHub Capability | 実装済み |
| v0.6 | Agent & Orchestrator Integration | 実装済み |
| v0.7 | Remote / Cloud Runtime & External Capabilities | experimental 実装済み。real AWS acceptance pending |
| v0.8 | Client Adapters & VS Code Integration | 実装済み。real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | session -> Environment broker foundation 実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | PR #137 で実装済み。real host acceptance は pending |
| v0.11 | Base Images & Custom Environments | first slice 実装済み。Base selection / immutable revision pin / persisted identity / list・inspect を実装 |
| v0.12 | Sandbox Resource Limits | first slice 実装済み。CPU / memory / PID / root disk の budget、Incus pre-start enforcement、persist/status を実装 |

これで **v0.1〜v0.12 まで実装済み milestone が連続**します。

## v0.10 の扱い

旧 PR #111 は古い security/docs baseline の integration branch だったため直接 merge せず、最新 `main` に載せ直した PR #137 で `haco-agent-host` を実装しました。

real Windows/WSL + Incus + VS Code Agents window / Agent Host の acceptance は host-dependent のままです。

## v0.11 の扱い

first slice では次を実装しています。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

logical Base は作成時に immutable revision へ解決され、その revision が Environment metadata に保存されます。Incus の alias / remote / fingerprint の扱いは adapter 内部に閉じます。

custom build/import、revision history、rollback、GC は first slice の実装完了を意味しません。

## v0.12 の扱い

first slice では次を実装します。

```text
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

- 値の未指定は provider 任せにせず、Hacocoon の明示的な `unlimited` effective budget に解決して保存します。
- Incus は Environment を start する前に CPU / memory / PID / root disk 制限を設定し、読み戻して検証します。
- requested finite limit を provider が enforce できない場合は silent ignore せず fail closed します。
- experimental EC2 は有限 budget を AWS side effect より前に `unsupported` として拒否します。

real supported-Incus 上で実際に resource exhaustion が制限されることの acceptance は host-dependent のままです。

## 現在の acceptance watch list

- v0.8: real Windows/WSL + Incus + VS Code Remote-SSH
- v0.9/v0.10: real VS Code Agent Host/AHP routing、real Incus SSH
- v0.11: real Incus image remote / custom Base
- v0.12: real Incus CPU / memory / PID / root-disk enforcement
- v0.7 EC2: real AWS acceptance。provider は experimental/default-off のまま

## 一文でいうと

> **v0.1〜v0.12 の実装済み milestone を連番にし、repository implementation と real-host acceptance は `IMPLEMENTATION_STATUS.md` で分けて管理します。**
