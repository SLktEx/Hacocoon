# バージョン番号とリリース状況

[English](versioning-and-release-status.md) | **日本語**

> **milestone番号の正本 · 2026-08-30更新**

Hacocoonは **pre-1.0** です。milestone番号はproduct/implementationの進行を表し、compatibility guarantee、release tag、production supportの証明ではありません。

現在のcode realityとhost-dependent acceptanceは [`../IMPLEMENTATION_STATUS.ja.md`](../IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 番号付けの方針

> **minor milestoneはpre-1.0の軽量な進捗checkpointであり、完成判定ではありません。**

1. 意味のあるproduct/implementationの区切りがlandしたら、follow-up slice、hardening、real-host acceptanceが残っていても次の `v0.N` を使ってよい
2. 前のmilestoneがpartialでも、後続milestoneへ進んでよい。version順は時系列であり、過去gateがすべて完了したことを意味しない
3. 粒度は実用優先で決める。密接な複数PRを同じmilestoneにまとめてもよく、大きめのfollow-upを次minorへ分けてもよい
4. security/hardening、bug fix、refactor、CLI namespace整理、CI、docs、release engineering、test-only変更だけでは通常versionを消費しないが、現在のcheckpointへ含めてよい
5. milestoneを進める時はこのファイルと `../IMPLEMENTATION_STATUS.md` を更新し、roadmap/index summaryも矛盾しないようにする
6. design-only specificationはfuture numberを予約できるが、実装までは **planned**
7. historical commit/PR/branch/旧document address/過去の番号付けは現在の正本ではない
8. release tagとroadmap milestone番号は別物

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
| v0.19 | Domain-aware Egress Authorization | repository実装完了。real supported-Incus acceptanceはhost-dependent |
| v0.20 | Managed Btrfs Rootfs Storage | first repository slice実装済み。real-host COW/compaction acceptanceはhost-dependent |
| v0.21 | Managed Btrfs Transparent Compression | `compress=zstd:3` default実装済み。`compress-force`は使わない。real compression/performance acceptanceはhost-dependent |

現在のmilestone位置は **v0.21** です。前のpartial milestoneは残件として追跡しますが、後続のdevelopment checkpointを進める妨げにはしません。

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
- v0.19: [`../EGRESS_AUTHORIZATION.ja.md`](../EGRESS_AUTHORIZATION.ja.md)
- v0.20: [`../design/btrfs-storage-layout.ja.md`](../design/btrfs-storage-layout.ja.md)
- v0.21: [`../design/btrfs-storage-layout.ja.md`](../design/btrfs-storage-layout.ja.md)
- Optional Local OCI Registry: [`../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md)

## Acceptance

- v0.7 cloud implementation: deferred
- v0.8〜v0.13: real Windows/WSL、Agent Host、Base/resource/network acceptanceはhost-dependent
- v0.14: private repository combinationはacceptance-sensitive
- v0.15/v0.16: recommendation/deletion repository behaviorは実装済み
- v0.17: Seed build/publish、explicit pin/reenable、保守的old-revision GC、中断builder recovery、deletion-race protection、managed Environment harvestまでrepository実装済み。authenticated/private-registry combination、physical Btrfs COW measurement、broader real-host failure injection、supported-host acceptanceが残る
- v0.18: repository lifecycle/CLIは実装済み。real Incus/systemd socket activation acceptanceはhost-dependent
- v0.19: hostname-aware proxy authorization/enforcementはrepository実装済み。real supported-Incus bridge/nftables/dnsmasq acceptanceはhost-dependent
- v0.20: Hacocoon所有Incus rootfsはlazyなmanaged sparse-raw Btrfs poolを選択する。physical COW/compaction measurementとbroader supported-host acceptanceはhost-dependent
- v0.21: managed Btrfs mountは `compress=zstd:3` を使い、desired stateとして `compress-force` を採用しない。既存COW/reflink sharingを壊し得る自動recompressionは行わない。real compression ratio、CPU cost、supported-host behaviorはhost-dependent

> **意味のある進捗の塊がlandしたら次minorへ進めてよい。すべてのacceptanceが閉じるまで待つ必要はない。**
