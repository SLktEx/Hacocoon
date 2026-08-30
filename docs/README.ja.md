# Hacocoon ドキュメント

[English](README.md) | **日本語**

Hacocoon は **pre-1.0** です。architecture intent、現在のrepository reality、real-host acceptanceを分けて読みます。

> 現在の`main`で何が動くかだけ知りたい場合は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) から読んでください。

## 正本の順番

1. 現在の実装事実: [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
2. milestone番号/status: [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
3. product boundary / roadmap: [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
4. security: [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
5. terminology: [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
6. adapter/plugin境界: [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)

## 現在のmilestone

| Version | Gate | State |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ |
| v0.2 | Workspace Abstraction & Lease | ✅ |
| v0.3 | Client & Interactive Access | ✅ |
| v0.4 | Policy & Capability Foundation | ✅ |
| v0.5 | Git / GitHub Push Capability | ✅ |
| v0.6 | Agent & Orchestrator Integration | ✅ |
| v0.7 | Remote / Cloud Runtime | 🧪 experimental |
| v0.8 | Client Adapters & VS Code | ✅ |
| v0.9 | Per-Agent Sandbox | ✅ |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ |
| v0.11 | Base Images & Custom Environments | ✅ first slice |
| v0.12 | Sandbox Resource Limits | ✅ first slice |
| v0.13 | Managed Sandbox Network | ✅ |
| v0.14 | Git Fetch Plugin | ✅ |
| v0.15 | OCI Seed Usage & Recommendation | ✅ first slice |
| v0.16 | OCI Image Deletion | ✅ first slice |
| v0.17 | Docker Compatibility | ✅ packaging foundation |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

実装済みmilestoneは **v0.17まで連続**。次のplanned sliceはv0.18です。

## v0.13以降の資料

- [`13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.ja.md)
- [`14_v0.14_GIT_FETCH_PLUGIN.ja.md`](14_v0.14_GIT_FETCH_PLUGIN.ja.md)
- [`15_v0.15_OCI_SEED_RECOMMENDATION.ja.md`](15_v0.15_OCI_SEED_RECOMMENDATION.ja.md)
- [`16_v0.16_OCI_IMAGE_DELETION.ja.md`](16_v0.16_OCI_IMAGE_DELETION.ja.md)
- [`17_v0.17_DOCKER_COMPATIBILITY.ja.md`](17_v0.17_DOCKER_COMPATIBILITY.ja.md)
- [`18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.ja.md)
- [`19_v0.19_OCI_SEED_AND_COW.ja.md`](19_v0.19_OCI_SEED_AND_COW.ja.md)

## Base imageとworkload OCI image

Top-level `haco image` はHacocoon Base identityです。

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

workload OCI toolingはoptional pluginです。

```text
HACO_PLUGIN_OCI=nerdctl  # または docker
haco plugin oci ...
```

詳細は [`OCI_RUNTIME_AND_DOCKER_COMPAT.ja.md`](OCI_RUNTIME_AND_DOCKER_COMPAT.ja.md)。

## Git

privileged GitHub fetch/pushは `haco plugin git` 配下。ordinary Git UXはGit自身の責任です。詳細は [`GIT_GITHUB_CAPABILITY.md`](GIT_GITHUB_CAPABILITY.md)。

## Client UI / notification

VS Codeは最初のclient adapterであってCoreではありません。Browser/Web NotificationやrichなInteraction APIはfuture client/adapter work。VS Code extensionもoptionalです。

## 実装とacceptance

specificationの存在だけで実装済みとは扱いません。またrepository testが通ってもreal Incus / Windows / AWS / container tool / Btrfs acceptanceの代わりにはなりません。

変更後は `python tools/check_docs.py` を実行します。
