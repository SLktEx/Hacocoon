# バージョン番号とリリース状況

[English](00D_VERSIONING_AND_RELEASE_STATUS.md) | **日本語**

> **milestone番号の日本語案内 · 2026-08-30更新**

Hacocoonは **pre-1.0** です。milestone番号はproduct/implementationの進行順を表すもので、compatibility guarantee、release tag、production supportの証明ではありません。

**番号の正本は英語版 [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)**、現在の実装事実は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。

## 番号付けの方針

1. 実装済みmilestoneは、変更コストが低い間はなるべく連続させる
2. design-only gateのせいで、既に実装済みの独立機能を後ろの番号へ追いやらない
3. security/hardeningだけでは通常product versionを消費しない
4. projectが便利なprofileを提供しても、optional integrationをCore必須要件へ昇格させない
5. release tagとroadmap milestone番号は別物
6. repository implementationの正本は`IMPLEMENTATION_STATUS.md`

## 現在の番号

**凡例:** ✅ 実装済み · 🧪 historical/deferred · 🚧 planned

| Version | Gate | 状況 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ 実装済み |
| v0.2 | Workspace Abstraction & Lease | ✅ 実装済み |
| v0.3 | Client & Interactive Access | ✅ 実装済み |
| v0.4 | Policy & Capability Foundation | ✅ 実装済み |
| v0.5 | Git / GitHub Capability | ✅ 実装済み |
| v0.6 | Agent & Orchestrator Integration | ✅ 実装済み |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 provider seamは維持。EC2/AWS/EBS実装はdeferred |
| v0.8 | Client Adapters & VS Code Integration | ✅ 実装済み。real client acceptance pending |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ 実装済み。real host acceptance pending |
| v0.11 | Base Images & Custom Environments | ✅ first slice実装済み |
| v0.12 | Sandbox Resource Limits | ✅ first slice実装済み |
| v0.13 | Optional OCI Plugin | ✅ first slice実装。opt-in nerdctl/Docker driver、telemetry、Seed recommendation foundation |
| v0.13A | OCI Seed Build & Btrfs/COW Optimization | 🚧 follow-up planned。Seed build/publishとreal storage acceptanceはpending |
| v0.13B | OCI Seed Automatic Promotion Policy | ✅ selection policy実装。Seed Builderでのconsumeはpending |
| v0.13C | OCI Image Deletion | ✅ first slice実装。plugin-owned deletion/tombstone、replacement Seed publish/GCはpending |

現在の実装progressionは **v0.13 optional-plugin first sliceまで連続**しています。v0.7はcloud-specific実装をdeferしてもprovider-neutral seamが残るため、その番号を維持します。

## Core / optional boundary

OCI specificationが存在しても、OCI toolingがCore機能になるわけではありません。

`HACO_PLUGIN_OCI`未設定なら、Hacocoonは`containerd` / `nerdctl` / Docker CLI / Docker Engineを要求しません。

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference[@digest]>
```

project-maintainedな`containerd + nerdctl`構成はoptional profile。本物のDocker CLI / Docker Engine compatibilityもoptionalです。

## Acceptance watch list

- **v0.7:** cloud implementationは現在deferred。以前のEC2 providerはexperimental/default-offでreal AWS acceptance pendingだった。cloud adapter復活時にacceptanceを再定義する
- **v0.8:** real Windows/WSL + Incus + VS Code Remote-SSH
- **v0.9/v0.10:** real VS Code Agent Host/AHP routing、Incus SSH
- **v0.11:** real Incus image source/custom Base。build/import/history/rollback/GCはfollow-up
- **v0.12:** real Incus CPU/memory/PID/root-storage enforcement
- **v0.13:** plugin境界・driver selection・telemetry・recommendation・deletion logicはrepository実装済み。real OCI profile/container-tool acceptanceはpending
- **v0.13A:** Seed build/publishとreal Btrfs/COW acceptanceはplanned
- **v0.13B:** selection policyは実装済み。future Seed Builderでのconsumeはplanned
- **v0.13C:** deletion/tombstoneは実装済み。replacement Seed publishとold-Seed GCはpending

## 一文でいうと

> **番号はこのファイル、実装事実は`IMPLEMENTATION_STATUS.md`、設計意図はroadmap/specificationを見る。便利なprofileを同梱してもoptional toolingはoptionalのまま。**
