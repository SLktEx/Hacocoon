# バージョン番号とリリース状況

[English](00D_VERSIONING_AND_RELEASE_STATUS.md) | **日本語**

**Status:** バージョン番号・進行状況の日本語案内  
**Date:** 2026-08-29  
**Compatibility:** Hacocoon は pre-1.0 です。

> [!NOTE]
> 厳密な正本は英語版 `00D_VERSIONING_AND_RELEASE_STATUS.md` です。この日本語版は同じ判断を読みやすく説明します。

## 何のための資料か

並行して複数の設計・実装を進めていると、別々の機能が同じ `v0.x` を名乗ることがあります。この資料は、その衝突を防ぎ、`main` 上でどの番号が何に割り当てられているかを明確にするためのものです。

重要な原則は次です。

> **バージョン番号の割り当ては `main` が正本です。open PR が先に名乗っていても、`main` で既に別の機能へ割り当て済みなら、未merge側を変更します。**

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
| v0.9 | Base Images & Custom Environments | 設計のみ。実装 pending |
| v0.10 | Per-Agent Sandbox & Agent Host Integration | session -> Environment broker foundation 実装済み |
| v0.11 | Sandbox Resource Limits | 設計のみ。実装 pending |
| v0.12 | VS Code Remote Agent Host Adapter | PR #111 の integration 用に予約。まだ `main` 実装とは扱わない |

番号は「下の番号が全部実装完了してから次へ進む」という意味ではありません。実際に v0.9 は設計のみですが、独立した v0.10 broker foundation はすでに実装されています。

## 今回見つかった衝突

PR #111 は当初 **`v0.11 VS Code Remote Agent Host Adapter`** として作られていました。一方、`main` には既に **v0.11 Sandbox Resource Limits** の仕様が入っています。

そのため、現在の正しい並びは次です。

```text
v0.10  Per-Agent Sandbox & Agent Host Integration
v0.11  Sandbox Resource Limits
v0.12  VS Code Remote Agent Host Adapter
```

既に `main` にある v0.11 を動かすより、まだ merge 前の #111 を v0.12 に変更する方が履歴・docs consistency・merge safety の面で安全です。

PR #111 を merge する前に、versioned document の filename、heading、本文、README/roadmap link、docs check、PR title/body、日本語 companion を v0.12 にそろえる必要があります。

## 今後のルール

1. `main` に roadmap/spec の割り当てが入った時点で、その番号は予約済みとする。
2. 新しい番号を決める前に `main` docs と open PR の両方を確認する。
3. 衝突した場合は、原則として merge 前の作業側を次の空き番号へ変更する。
4. filename だけでなく、heading・本文・index・docs check・PR metadata・日本語版までまとめて変更する。
5. Security/hardening だけのPRは通常、product roadmap version を消費しない。
6. version number と implementation status を混同しない。実装済みかどうかは `IMPLEMENTATION_STATUS.md` / `.ja.md` を見る。
7. design gate の番号が存在しても、その version の release/tag が ready という意味ではない。

## 現在の watch list

- PR #111: Agent Host adapter — **v0.12 として扱う**。merge前にbranch docsのrenumberが必要。
- PR #113: patched Go toolchain hardening — roadmap version 不要。
- PR #114: Incus delete/absence verification hardening — roadmap version 不要。

## 一文でいうと

> **`main` が Hacocoon のバージョン番号を決め、`IMPLEMENTATION_STATUS` が実装の事実を決める。並行ブランチはその両方に合わせてから merge する。**
