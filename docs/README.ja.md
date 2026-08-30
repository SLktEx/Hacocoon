# Hacocoon ドキュメント

[English](README.md) | **日本語**

Hacocoonはpre-1.0です。architecture intent、現在のrepository reality、real-host acceptanceを分けて扱います。

## まず読む

- 現在の実装: [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- Architecture / Roadmap: [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- Milestone番号: [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- Security: [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- Plugin境界: [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)

## 番号ルール

独立して使える1機能 ≒ 1 minor milestone。fix / hardening / refactor / CLI namespace cleanup / CI / docsだけでは通常versionを消費しません。

| Version | Gate | State |
|---|---|---|
| v0.13 | Managed Sandbox Network | 実装済み |
| v0.14 | Git Fetch Plugin | 実装済み |
| v0.15 | OCI Seed Recommendation | 実装済み |
| v0.16 | OCI Image Deletion | first slice実装済み |
| v0.17 | Docker Compatibility Plugin | foundation / partial |
| v0.18 | Optional Local OCI Registry | planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | planned |

完全実装済みのproduct progressionは **v0.16まで連続**しています。

## Base と OCI

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci ...
HACO_PLUGIN_OCI=docker   haco plugin oci ...
```

BaseはEnvironmentのstarting identity、OCIはoptionalなdeveloper workload toolingです。Coreはcontainerd / nerdctl / Dockerを必須にしません。

## Cloud

v0.7のprovider-neutral routing seamは維持しますが、concrete EC2/AWS/EBS implementationはactive treeから削除済みで、cloud implementationは現在deferredです。
