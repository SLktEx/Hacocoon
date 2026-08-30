# バージョン番号とリリース状況

[English](versioning-and-release-status.md) | **日本語**

> **milestone番号の正本 · 2026-08-31更新**

Hacocoonは **pre-1.0** です。milestone番号はproduct/implementationの進行を表し、compatibility guarantee、release tag、production supportの証明ではありません。

現在のcode realityとhost-dependent acceptanceは [`../IMPLEMENTATION_STATUS.ja.md`](../IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 番号付けの方針

> **minor milestoneは惜しまず使うpre-1.0の軽量checkpointであり、完成判定ではありません。**

1. 長く残るproduct機能、implementation slice、operator-facing機能、cross-cutting infrastructureがlandしたら、follow-up、hardening、real-host acceptanceが残っていても次の `v0.N` を使ってよい
2. 前のmilestoneがpartialでも後続milestoneへ進んでよい。version順は時系列であり、過去gateがすべて完了したことを意味しない
3. 無関係なdurable capabilityを1つの番号へ詰め込むより、番号を進める方を優先する。密接な変更は同じmilestoneへまとめてもよい
4. 純粋なbug fix、狭いhardening、refactor、CLI cleanup、docs、release engineering、test-only変更だけでは通常milestoneを消費しない。一方、notification、logging、privileged brokerのように長く残るoperational contractはminorを使ってよい
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
| v0.22 | Notification Delivery & Client Adapters | browser、native OS、VS Code notification clientを実装・テスト済み |
| v0.23 | Structured Logging & Diagnostic Safety | shared `log/slog`、structured diagnostic、secret redactionを実装済み |
| v0.24 | Managed Host Privilege Broker for Btrfs Storage | root-owned narrow storage helper、privilege separation、real normal-user Incus/Btrfs CLI acceptanceを実装済み |

現在のmilestone位置は **v0.24** です。前のpartial milestoneは残件として追跡しますが、後続のdevelopment checkpointを進める妨げにはしません。

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
- v0.22: [`../INTERACTION_EVENTS.ja.md`](../INTERACTION_EVENTS.ja.md)
- v0.23: [`../reference/logging.ja.md`](../reference/logging.ja.md)
- v0.24: [`../design/btrfs-storage-layout.ja.md`](../design/btrfs-storage-layout.ja.md)
- Optional Local OCI Registry: [`../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md)

## Acceptance

- v0.7 cloud implementation: deferred
- v0.8〜v0.13: real Windows/WSL、Agent Host、Base/resource/network combinationは一部host-dependent
- v0.14〜v0.18: repository behaviorは実装済み。private repository/registry、physical COW、Docker real-host combinationはacceptance-sensitive
- v0.19: hostname-aware proxy authorization/enforcementはrepository実装済み。real supported-Incus bridge/nftables/dnsmasq acceptanceはhost-dependent
- v0.20〜v0.21: managed Btrfs storage/compressionは実装済み。physical compression ratio、CPU cost、COW/compaction、broader supported-host behaviorは残件
- v0.22: browser/native/VS Code notification flowはrepository/CI behavior test済み。real desktop integrationはplatform-dependent
- v0.23: structured loggingとredactionはmaintained executable全体へ実装済み。downstream log collectionはdeployment-specific
- v0.24: privileged storage helper lifecycleとordinary-user `haco create` / `exec` / `delete` / `run` をdisposable GitHub-hosted Ubuntu 26.04上のreal Incus + managed Btrfsでacceptance済み。ただし全supported Host構成の証明ではない

> **Hacocoonの「できること」を説明するときに挙げる程度に長く残るcapabilityなら、pre-1.0 minorを惜しまず使ってよい。**
