# バージョン番号とリリース状況

[English](00D_VERSIONING_AND_RELEASE_STATUS.md) | **日本語**

> **milestone番号の日本語案内 · 2026-08-30更新**

Hacocoon は **pre-1.0** です。milestone番号はproduct/implementationの進行順を表すためのもので、compatibility guarantee、release tag、production supportの証明ではありません。

番号の正本は英語版 [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)、現在の実装事実は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。

## 番号付けの方針

1. できるだけ **独立して価値のある1機能を1 milestone** として扱う
2. 実装済みmilestoneは、変更コストが低い間はなるべく連続させる
3. security/hardeningだけでは通常product versionを消費しない
4. optional integrationは、project推奨構成を提供してもCore dependencyにはしない
5. planned specificationは実装されるまでplannedと明記する
6. release tagとroadmap milestone番号は別物

## 現在の番号

**凡例:** ✅ 実装済み · 🧪 experimental · 🚧 planned

| Version | Gate | `main` の状況 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ 実装済み |
| v0.2 | Workspace Abstraction & Lease | ✅ 実装済み |
| v0.3 | Client & Interactive Access | ✅ 実装済み |
| v0.4 | Policy & Capability Foundation | ✅ 実装済み |
| v0.5 | Git / GitHub Push Capability | ✅ 実装済み |
| v0.6 | Agent & Orchestrator Integration | ✅ 実装済み |
| v0.7 | Remote / Cloud Runtime | 🧪 experimental。EC2は明示opt-in |
| v0.8 | Client Adapters & VS Code | ✅ 実装済み |
| v0.9 | Per-Agent Sandbox | ✅ 実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ 実装済み |
| v0.11 | Base Images & Custom Environments | ✅ first slice |
| v0.12 | Sandbox Resource Limits | ✅ first slice |
| v0.13 | Managed Sandbox Network | ✅ 実装済み |
| v0.14 | Git Fetch Plugin | ✅ `haco plugin git fetch` 実装済み |
| v0.15 | OCI Seed Usage & Recommendation | ✅ optional OCI pluginのfirst slice実装済み |
| v0.16 | OCI Image Deletion | ✅ optional OCI pluginのfirst slice実装済み |
| v0.17 | Docker Compatibility | ✅ packaging foundation実装済み。Base/Seed組み込みとreal-host acceptanceはpending |
| v0.18 | Optional Local OCI Registry | 🚧 planned。標準必須インフラにはしない |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

**実装済みmilestoneは v0.1〜v0.17 まで連続**しています。次のplanned product sliceは v0.18 です。

## OCI / Docker / nerdctl の境界

OCI/container toolingはHacocoon Coreの必須要件ではありません。必要なinstallationだけ明示的にpluginを有効化します。

```text
HACO_PLUGIN_OCI=nerdctl
HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

`HACO_PLUGIN_OCI` 未設定なら、Environment管理のためだけにnerdctl、Docker CLI、dockerd、Host OCI cache、Local Registryを要求しません。

`haco image list` / `haco image inspect` はHacocoonの **Base identity** を扱うCore-facing commandとして残します。workload OCI imageのtelemetry/Seed/deletionは `haco plugin oci` 配下です。

## Git Fetch

GitHub向けのprivileged fetch/pushはCore commandではなくplugin namespaceです。

```text
haco plugin git fetch <environment>
haco plugin git push <environment> --branch <branch>
```

HTTPS GitHub認証ではHost側の `gh auth git-credential` をbroker経路から明示利用でき、credentialそのものをSandboxへコピーしません。

## 旧番号について

過去のcommitや資料にある「v0.13 Local OCI Registry」「v0.13A/B/C Seed関連」は旧番号です。historical commit messageやclosed PR titleは履歴として残しますが、現在の番号はこのファイルを正本とします。

## Acceptance watch list

- **v0.7:** real AWS/EC2/SSM/EBS
- **v0.8〜v0.10:** real Windows/WSL + Incus + VS Code / Agent Host
- **v0.11〜v0.13:** real Incus image/resource/network
- **v0.14:** Host `gh` / SSH credentialを含む実環境Git acceptance
- **v0.15〜v0.17:** optional OCI pluginのreal container-tool/Base integration
- **v0.18〜v0.19:** plannedのみ

## 一文でいうと

> **番号はこのファイル、実装事実は `IMPLEMENTATION_STATUS.md`、OCI/Docker/nerdctlは必要な人だけ使うoptional plugin。**
