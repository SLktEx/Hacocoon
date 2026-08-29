# バージョン番号とリリース状況

[English](00D_VERSIONING_AND_RELEASE_STATUS.md) | **日本語**

**Status:** バージョン番号・進行状況の日本語案内  
**Date:** 2026-08-30  
**Compatibility:** Hacocoon は pre-1.0 です。

> [!NOTE]
> 厳密な正本は英語版 `00D_VERSIONING_AND_RELEASE_STATUS.md` です。この日本語版は同じ判断を読みやすく説明します。

## 方針

pre-1.0 の間は、バージョン番号をなるべく **実装された順番として読める形** に保ちます。

ルールは次です。

1. 実装済み milestone は可能な限り連番にする。
2. 未実装の design gate が、独立して既に実装された機能より若い番号を占有しないようにする。
3. 次に merge される active implementation を、最後の実装済み milestone の次へ置く。
4. design-only gate はその後ろへ送る。
5. Security / hardening だけの変更は通常 product version を消費しない。
6. 実装の事実は `IMPLEMENTATION_STATUS.md` が正本。
7. roadmap の番号と tag/release は別物として扱う。

これは以前の「`main` に design 番号が入ったら原則固定」というルールを置き換えます。pre-1.0 の今は、偶然できた分かりにくい番号を守るより、読みやすさを優先します。

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
| v0.10 | VS Code Remote Agent Host Adapter | PR #111 の active integration。まだ `main` 実装とは扱わない |
| v0.11 | Base Images & Custom Environments | 設計のみ。実装 pending |
| v0.12 | Sandbox Resource Limits | 設計のみ。実装 pending |

これで **v0.1〜v0.9 まで実装済み milestone が連続**します。v0.10 が次の実装中 gate、v0.11 / v0.12 は design-only としてその後ろです。

## 2026-08-30 の整理

変更前:

```text
v0.9   Base Images & Custom Environments             design only
v0.10  Per-Agent Sandbox & Agent Host Integration    implemented
v0.11  Sandbox Resource Limits                       design only
v0.12  VS Code Remote Agent Host Adapter             active PR
```

変更後:

```text
v0.9   Per-Agent Sandbox & Agent Host Integration    implemented
v0.10  VS Code Remote Agent Host Adapter             active PR
v0.11  Base Images & Custom Environments             design only
v0.12  Sandbox Resource Limits                       design only
```

これは roadmap / documentation の番号整理です。Git history を書き換えたり、新しい実装が完了したと主張したりはしません。

## 移行ルール

maintained docs では次のように読み替えます。

- 旧 `v0.10 Per-Agent Sandbox` → **v0.9**
- PR #111 / 旧 `v0.12 Agent Host Adapter` → **v0.10**
- 旧 `v0.9 Base Images` → **v0.11**
- 旧 `v0.11 Resource Limits` → **v0.12**

過去の commit message、closed PR title、既に作られた一時的な candidate branch 名は履歴として旧番号が残る場合があります。現在の番号の正本にはしません。

## 現在の watch list

- PR #111: Agent Host adapter — 現在の番号では **v0.10**。real Windows/WSL + Incus + VS Code Agents-window acceptance は host-dependent。
- PR #113: patched Go toolchain hardening — roadmap version 不要。
- PR #114: Incus delete/absence verification hardening — roadmap version 不要。

## 一文でいうと

> **実装済みを連番にし、その次に実装中、その後ろに design-only を置く。実装の事実そのものは `IMPLEMENTATION_STATUS.md` で管理する。**
