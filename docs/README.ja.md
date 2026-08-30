# Hacocoon ドキュメント

[English](README.md) | **日本語**

Hacocoon は **pre-1.0** です。architecture intent、repository reality、real-host acceptanceを分けて読みます。

> [!TIP]
> **「いま `main` で何が使える？」なら [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) から読んでください。**

## まずここから

| 知りたいこと | 読む資料 |
|---|---|
| Hacocoon 全体像 | [`../README.ja.md`](../README.ja.md) |
| 現在の実装状況 | [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) |
| architecture / roadmap | [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md) |
| authoritative version番号 | [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md) |
| security boundary | [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md) |
| 用語・責務境界 | [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md) |
| VS Code / client | [`08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md`](08_v0.8_CLIENT_ADAPTERS_AND_VSCODE_INTEGRATION.md) |
| Per-Agent Sandbox | [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.ja.md) |
| VS Code Agent Host | [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.ja.md) |
| Base Image | [`BASE_IMAGES.ja.md`](BASE_IMAGES.ja.md) |
| Resource Limit | [`12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.ja.md) |
| Managed Sandbox Network | [`13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md) |
| Git Fetch Plugin | [`14_v0.14_GIT_FETCH_PLUGIN.ja.md`](14_v0.14_GIT_FETCH_PLUGIN.ja.md) |
| OCI Seed Recommendation | [`15_v0.15_OCI_SEED_RECOMMENDATION.ja.md`](15_v0.15_OCI_SEED_RECOMMENDATION.ja.md) |
| OCI Image Deletion | [`16_v0.16_OCI_IMAGE_DELETION.ja.md`](16_v0.16_OCI_IMAGE_DELETION.ja.md) |
| Docker Compatibility Plugin | [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.ja.md) |
| Optional Local OCI Registry | [`18_v0.18_LOCAL_OCI_REGISTRY.ja.md`](18_v0.18_LOCAL_OCI_REGISTRY.ja.md) |
| OCI Seed Builder / COW | [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md) |

## 正本の使い分け

1. **現在のcode reality:** `IMPLEMENTATION_STATUS.md`
2. **milestone番号/status:** `00D_VERSIONING_AND_RELEASE_STATUS.md`
3. **product boundary / roadmap:** `00_REBASELINE_AND_ROADMAP.md`
4. **canonical terminology:** `00C_TERMINOLOGY_AND_BOUNDARIES.md`
5. **security rule:** `00B_SECURITY_ARCHITECTURE.md`
6. **feature contract:** 各versioned specification
7. **detail / operation:** `CLIENT_ACCESS.md`, `REMOTE_CLOUD_PROVISIONING.md`, `BASE_IMAGES.md`, `OCI_RUNTIME_AND_DOCKER_COMPAT.md` など
8. **plugin / adapter guidance:** `00A_PLUGIN_ARCHITECTURE.md`
9. **implementation workflow:** `90_CODEX_IMPLEMENTATION_HANDOFF.md`

README/indexは入口であり、current code realityを上書きしません。

## 番号付けルール

> **独立して価値のある1機能につき、おおむね1つのminor milestone。**

feature implementation PR自身で番号/statusを更新します。security fix、bug fix、hardening、refactor、CI、docs、test-only変更だけでは通常product versionを進めません。

## 現在のmilestone

**凡例:** ✅ 実装済み · 🧪 partial/experimental · 🚧 planned

| Version | Gate | State |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ |
| v0.2 | Workspace Abstraction & Lease | ✅ |
| v0.3 | Client & Interactive Access | ✅ |
| v0.4 | Policy & Capability Foundation | ✅ |
| v0.5 | Git / GitHub Capability | ✅ |
| v0.6 | Agent & Orchestrator Integration | ✅ |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 experimental |
| v0.8 | Client Adapters & VS Code Integration | ✅ |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ foundation |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ |
| v0.11 | Base Images & Custom Environments | ✅ first slice |
| v0.12 | Sandbox Resource Limits | ✅ first slice |
| v0.13 | Managed Sandbox Network | ✅ |
| v0.14 | Git Fetch Plugin | ✅ |
| v0.15 | OCI Seed Recommendation | ✅ |
| v0.16 | OCI Image Deletion | ✅ first slice |
| v0.17 | Docker Compatibility Plugin | 🧪 foundation |
| v0.18 | Optional Local OCI Registry | 🚧 |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 |

**完全実装済みmilestoneはv0.16まで連続**しています。正確なcode realityは [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) を参照してください。

## v0.13-v0.19 の読み方

```text
v0.13 managed Incus sandbox network
 -> v0.14 brokered Git fetch plugin
 -> v0.15 OCI Seed candidate recommendation
 -> v0.16 OCI identity delete/tombstone
 -> v0.17 optional Docker compatibility plugin (partial)
 -> v0.18 optional Local Registry (planned)
 -> v0.19 offline immutable Seed Builder + COW (planned)
```

重要な区別:

- v0.15 recommendationは実装済みですがphysical Seed build/publishは未実装
- v0.16はHost cache/future Seed selectionとexplicit指定時のcurrent Environmentへ作用し、published immutable Seedをin-place mutationしない
- v0.17でもstandard runtimeはcontainerd + nerdctlのまま
- v0.18はoptionalで、policyが許せばnormal `nerdctl pull` はconfigured upstreamへ直接行ける
- v0.19で複数Environmentに一つのwritable `/var/lib/containerd` を共有してはいけない

## Specification と Implementation は別

versioned specificationが存在しても実装済みとは限りません。current code realityは `IMPLEMENTATION_STATUS.md` で判断します。

またunit/fake-provider/repository testはreal Incus、Windows/VS Code、private registry、AWS acceptanceの代替ではありません。

## Breaking Change

Hacocoonはpre-1.0で、security/ownership boundaryを明確化するためのBreaking Changeを許容します。accidental compatibilityを守るために安全性を弱めるより、明示的な修正を優先します。

## ドキュメント更新ルール

1. 事実を所有するauthoritative documentを更新
2. code realityが変わったら `IMPLEMENTATION_STATUS.md` を更新
3. 独立機能が増えたら**そのfeature PRで** `00D_VERSIONING_AND_RELEASE_STATUS.md` と次minorを更新
4. fix/hardening/refactor/CI/docs-onlyではproduct versionを進めない
5. implementation claimとreal-host acceptanceを分離
6. experimental/default-off/partialを明記
7. `python tools/check_docs.py` を実行
