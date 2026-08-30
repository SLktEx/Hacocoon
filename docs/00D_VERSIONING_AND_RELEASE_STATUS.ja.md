# バージョン番号とリリース状況

[English](00D_VERSIONING_AND_RELEASE_STATUS.md) | **日本語**

> **milestone番号の日本語案内 · 2026-08-30更新**

Hacocoon は **pre-1.0** です。milestone番号はproduct/implementationの進行を表し、compatibility guarantee、release tag、production supportの証明ではありません。

番号の正本は英語版 [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)、実装事実の正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。

## 番号付けの方針

> **独立して価値のある1機能につき、おおむね1つのminor milestoneを使う。**

1. 独立した新機能は通常、次の `v0.N` を使う
2. 同じ1機能を完成させる複数PRは同じmilestoneにまとめてよい
3. security/hardening、bug fix、refactor、CI、docs、release engineering、test-only変更だけでは通常versionを消費しない
4. feature implementation PR自身でこのファイルと `IMPLEMENTATION_STATUS.md` を更新し、後日の整理へ先送りしない
5. design-only specificationはfuture numberを予約できるが、実装されるまでは **planned**
6. historical commit/PR/branch/旧filenameは現在番号の正本ではない
7. release tagとroadmap milestone番号は別物

## 現在の番号

**凡例:** ✅ 実装済み · 🧪 partial/experimental · 🚧 planned

| Version | Gate | `main` の状況 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ 実装済み |
| v0.2 | Workspace Abstraction & Lease | ✅ 実装済み |
| v0.3 | Client & Interactive Access | ✅ 実装済み |
| v0.4 | Policy & Capability Foundation | ✅ 実装済み |
| v0.5 | Git / GitHub Capability | ✅ 実装済み |
| v0.6 | Agent & Orchestrator Integration | ✅ 実装済み |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 experimental実装。EC2はdefault-off |
| v0.8 | Client Adapters & VS Code Integration | ✅ 実装済み |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ 実装済み |
| v0.11 | Base Images & Custom Environments | ✅ first slice実装済み |
| v0.12 | Sandbox Resource Limits | ✅ first slice実装済み |
| v0.13 | Managed Sandbox Network | ✅ 実装済み |
| v0.14 | Git Fetch Plugin | ✅ 実装済み |
| v0.15 | OCI Seed Recommendation | ✅ 実装済み |
| v0.16 | OCI Image Deletion | ✅ first slice実装済み |
| v0.17 | Docker Compatibility Plugin | 🧪 foundation実装済み。full plugin integrationはpending |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

**完全に実装済みのproduct progressionはv0.16まで連続**しています。v0.17はfoundation段階、v0.18/v0.19はplannedです。

## v0.12 → v0.19 再整理

`v0.13` と `v0.13A/B/C` の下に独立機能が増え、元々の「1機能1versionくらい」という方針とずれたため、次のように正式に振り直します。

```text
v0.12  Sandbox Resource Limits                 implemented
v0.13  Managed Sandbox Network                 implemented
v0.14  Git Fetch Plugin                        implemented
v0.15  OCI Seed Recommendation                 implemented
v0.16  OCI Image Deletion                      implemented
v0.17  Docker Compatibility Plugin             partial/foundation
v0.18  Optional Local OCI Registry             planned
v0.19  OCI Seed Builder & Btrfs/COW            planned
```

旧 `v0.13 Local Registry` と `v0.13A/B/C` はhistorical assignmentとしてのみ扱います。

## Specification map

- [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md)
- [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md)
- [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md)
- [`18_v0.18_LOCAL_OCI_REGISTRY.md`](18_v0.18_LOCAL_OCI_REGISTRY.md)
- [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md)

## Acceptance watch list

- **v0.7:** real AWS/EC2/SSM/EBS pending
- **v0.8:** real Windows/WSL + Incus + VS Code pending
- **v0.9/v0.10:** real Agent Host/AHP routing pending
- **v0.11/v0.12:** real Base/image・resource enforcement pending
- **v0.13:** real supported-Incus network/profile/ACL pending
- **v0.14:** brokered fetch実装済み。real private-repository combinationは別途acceptance
- **v0.15/v0.16:** repository behavior実装済み。physical Seed publication/GCはv0.19
- **v0.17:** plugin lifecycle/integration完了まではcomplete扱いしない
- **v0.18/v0.19:** planned

## 一文でいうと

> **独立した新機能ならそのPRで次minorを取る。fix/hardening/refactorならproduct versionは進めない。**
