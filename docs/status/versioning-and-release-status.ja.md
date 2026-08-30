# バージョン番号とリリース状況

[English](versioning-and-release-status.md) | **日本語**

> **milestone番号の正本 · 2026-08-31更新**

Hacocoonは **pre-1.0** です。milestone番号はproduct/implementationの進行を表し、compatibility guarantee、release tag、production supportの証明ではありません。

現在のcode realityとhost-dependent acceptanceは [`../IMPLEMENTATION_STATUS.ja.md`](../IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 番号付けの方針

> **minor milestoneはpre-1.0の軽量な進捗checkpointであり、完成判定ではありません。**

1. 意味のあるproduct、implementation、operator experience、observability、acceptanceの区切りがlandしたら、follow-up slice、hardening、real-host acceptanceが残っていても次の `v0.N` を使ってよい
2. 前のmilestoneがpartialでも、後続milestoneへ進んでよい。version順は時系列であり、過去gateがすべて完了したことを意味しない
3. 粒度は実用優先で決め、pre-1.0では意図的に細かく進めてよい。密接な複数PRを同じmilestoneにまとめてもよく、大きめのfollow-upを次minorへ分けてもよい
4. security/hardening、bug fix、refactor、CLI namespace整理、CI、docs、release engineering、test-only変更は自動的にversionを消費するわけではないが、support、operability、acceptance上の意味あるcheckpointになる場合はminorを使ってよい
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
| v0.20 | Managed Btrfs Rootfs Storage | managed sparse-raw Btrfs poolとIncus rootfs routingを実装済み。broader physical COW/compaction acceptanceはhost-dependent |
| v0.21 | Managed Btrfs Transparent Compression | `compress=zstd:3` default実装済み。`compress-force`は使わない。real compression/performance acceptanceはhost-dependent |
| v0.22 | Interaction Notification Clients | browser、native OS、VS Code notification clientを実装済み。replay/dedup behaviorもtest済み |
| v0.23 | Real Incus E2E Acceptance | GitHub-hosted Ubuntu 26.04でstandalone Incus substrateとHacocoon Core lifecycleをphased gating付きで自動検証 |
| v0.24 | Structured Logging | shared `log/slog`、operation context、sanitize済みDEBUG trace、secret redactionをmaintained executableへ実装済み |
| v0.25 | Managed Btrfs Host Privilege Broker | root-owned typed storage helperとordinary-user real Incus/Btrfs CLI acceptanceを実装済み |
| v0.26 | Trusted `haco-host` & Default WSL Entry | persistent trusted logical Host lifecycle、ownership/collision check、managed-storage配置、default WSL entry、recovery path、real Incus acceptanceを実装済み |

現在のmilestone位置は **v0.26** です。前のpartial milestoneは残件として追跡しますが、後続のdevelopment checkpointを進める妨げにはしません。

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
- v0.24: [`../reference/logging.ja.md`](../reference/logging.ja.md)
- v0.25: [`../design/btrfs-storage-layout.ja.md`](../design/btrfs-storage-layout.ja.md)
- v0.26: [`../design/trusted-host.ja.md`](../design/trusted-host.ja.md)
- Optional Local OCI Registry: [`../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.ja.md)

v0.23は新しいarchitecture contractではなくacceptance checkpointです。実行可能なspecificationはGitHub Actions/CI harnessにあり、support boundaryは `IMPLEMENTATION_STATUS.ja.md` にまとめます。

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
- v0.22: browser/native/VS Code notification deliveryとreplay/dedup behaviorはrepository test済み。desktop/session固有のdeliveryは実client環境に依存する
- v0.23: GitHub-hosted Ubuntu 26.04でstandalone Incus system-container behaviorを先に証明してからCore lifecycle E2Eを実行する。CI上のsupport gapは縮まるが、全supported Host/WSL configurationの証明ではない
- v0.24: maintained executable全体でstructured logging/redaction behaviorを共有する。logging policyはdefense in depthであり、unsafeなcall-site dataを出してよいことにはならない
- v0.25: real helper lifecycleとordinary-user `haco create` / `exec` / `delete` / `run` をreal Incus + managed Btrfsで検証する。broader physical-storage / Windows/WSL acceptanceは引き続き残る
- v0.26: trusted-host creation、exact ownership/collision handling、idempotent ensure、stopped-state recovery、managed-storage配置、raw control-socket非公開をreal Incus acceptanceで検証済み。real Windows/WSL interactive-login behaviorとGit/OCI/credential/control-channelの全面移行はfollow-up

> **意味のあるproduct、operator、observability、acceptanceの進捗がlandしたら次minorへ進めてよい。pre-1.0ではversion番号を節約するよりcheckpointを見える化する。**
