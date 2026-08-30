# バージョン番号とリリース状況

[English](versioning-and-release-status.md) | **日本語**

> **milestone番号の正本 · 2026-08-30更新**

Hacocoonは **pre-1.0** です。milestone番号はproduct/implementationの進行を表し、compatibility guarantee、release tag、production supportの証明ではありません。

現在のcode realityとhost-dependent acceptanceは [`../IMPLEMENTATION_STATUS.ja.md`](../IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 番号付けの方針

> **独立して価値のある1機能につき、おおむね1つのminor milestoneを使う。**

1. 独立した新機能は通常、次の `v0.N` を使う
2. 同じ1機能を完成させる複数PRは同じmilestoneにまとめてよい
3. security/hardening、bug fix、refactor、CLI namespace整理、CI、docs、release engineering、test-only変更だけでは通常versionを消費しない
4. feature implementation PR自身でこのファイルと `../IMPLEMENTATION_STATUS.md` を更新する
5. design-only specificationはfuture numberを予約できるが、実装までは **planned**
6. historical commit/PR/branch/旧document address/過去の番号付けは現在の正本ではない
7. release tagとroadmap milestone番号は別物

## 現在の番号

| Version | Gate | `main` の状態 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | 実装済み |
| v0.2 | Workspace Abstraction & Lease | 実装済み |
| v0.3 | Client & Interactive Access | 実装済み |
| v0.4 | Policy & Capability Foundation | 実装済み |
| v0.5 | Git / GitHub Capability | 実装済み |
| v0.6 | Agent & Orchestrator Integration | 実装済み |
| v0.7 | Remote / Cloud Runtime & External Capabilities | provider routing seam維持、concrete cloudはdeferred |
| v0.8 | Client Adapters & VS Code Integration | 実装済み |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | broker foundation実装済み |
| v0.10 | VS Code Remote Agent Host Adapter | 実装済み |
| v0.11 | Base Images & Custom Environments | first slice実装済み |
| v0.12 | Sandbox Resource Limits | first slice実装済み |
| v0.13 | Managed Sandbox Network | 実装済み |
| v0.14 | Git Fetch Plugin | 実装済み |
| v0.15 | OCI Seed Recommendation | 実装済み |
| v0.16 | OCI Image Deletion | first slice実装済み |
| v0.17 | OCI Seed Builder & Btrfs/COW | repository build/publish + operations-hardening slice実装済み。real-host/private-registry/COW acceptanceはpending |
| v0.18 | Docker Compatibility Plugin | repository実装完了、real-host acceptanceは別 |

v0.17がpartialなfeature gateなので、完全実装済みのproduct progressionは **v0.16まで連続**しています。v0.18はrepository実装済みでも、前のv0.17 gateにacceptance残件があります。

v0.7のprovider-neutral routing seamは維持しますが、concrete EC2/AWS/EBS codeはactive treeになく、**cloud implementationは現在deferred**です。

**Local Registry infrastructureはdeferred/unversionedです。** 通常pullやSeed constructionの必須要件ではなく、roadmap milestoneを予約しません。

## Specification map

Document addressはsemantic nameを使うため、milestone assignmentが変わってもpathは変わりません。

- v0.13: [`../design/managed-sandbox-network.ja.md`](../design/managed-sandbox-network.ja.md)
- v0.14: [`../design/git-fetch-plugin.ja.md`](../design/git-fetch-plugin.ja.md)
- v0.15: [`../design/oci-seed-recommendation.ja.md`](../design/oci-seed-recommendation.ja.md)
- v0.16: [`../design/oci-image-deletion.ja.md`](../design/oci-image-deletion.ja.md)
- v0.17: [`../design/oci-seed-and-cow.ja.md`](../design/oci-seed-and-cow.ja.md)
- v0.18: [`../design/docker-compatibility-plugin.ja.md`](../design/docker-compatibility-plugin.ja.md)
- Optional Local OCI Registry: [`../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md)

## Acceptance

- v0.7 cloud implementation: deferred
- v0.8〜v0.13: real Windows/WSL、Agent Host、Base/resource/network acceptanceはhost-dependent
- v0.14: private repository combinationはacceptance-sensitive
- v0.15/v0.16: recommendation/deletion repository behaviorは実装済み
- v0.17: Seed build/publishに加え、explicit pin/reenable、保守的old-revision GC、中断builder recovery、deletion-race protectionまでrepository実装済み。credential-free Environment harvestを含むprivate-registry combination、physical Btrfs COW measurement、broader real-host failure injection、supported-host acceptanceが残る
- v0.18: repository lifecycle/CLIは実装済み。real Incus/systemd socket activation acceptanceはhost-dependent

> **独立した新機能ならそのPRで次minorを取る。fix/hardening/refactorならproduct versionは進めない。**
