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
| v0.12 | Sandbox Resource Limits | 設計のみ。実装 pending |

これで **v0.1〜v0.11 まで実装済み milestone が連続**します。次の design/implementation gate は v0.12 です。

## v0.10 の扱い

旧 PR #111 は古い security/docs baseline の integration branch だったため直接 merge せず、最新 `main` に載せ直した PR #137 で `haco-agent-host` を実装しました。

real Windows/WSL + Incus + VS Code Agents window / Agent Host の acceptance は host-dependent のままです。

## v0.11 の扱い

first slice では次を実装します。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

logical Base は作成時に immutable revision へ解決され、その revision が Environment metadata に保存されます。Incus の alias / remote / fingerprint の扱いは adapter 内部に閉じます。

custom build/import、revision history、rollback、GC は first slice の実装完了を意味しません。

## 現在の acceptance watch list

- v0.8: real Windows/WSL + Incus + VS Code Remote-SSH
- v0.9/v0.10: real VS Code Agent Host/AHP routing、real Incus SSH
- v0.11: real Incus image remote / custom Base
- v0.7 EC2: real AWS acceptance。provider は experimental/default-off のまま

## 一文でいうと

> **実装済みを連番にし、その次に active implementation、その後ろに design-only を置く。実装の事実そのものは `IMPLEMENTATION_STATUS.md` で管理する。**
