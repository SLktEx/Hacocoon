# バージョン番号とリリース状況

公開 G1 は planned のままです。内部の保存 rootfs export に、所有確認した native Incus image と上限付き archive streaming を追加しました。専用 Incus 6.0.5/Btrfs adapter 受入は 13.44 秒で成功し、リリースや checkpoint の宣言は追加しません。[Environment 持ち出し](../design/environment-transfer.ja.md)を参照してください。

v0.57 の OCI image cleanup は partial です。未接続の nerdctl Store の一覧・削除は、production composition と bare controller／CLI を使い、bd1c9a5 の実 Incus/Btrfs で成功しました。導入済み controller 全体の受け入れ、未接続 Docker、候補選択型 GC は未完了です。[画像操作](../design/oci-image-deletion.ja.md)を参照してください。

source repository と OCI Store 一式の確認・削除は、既存の registry・予約記録と Incus volume・device 操作を使います。Workspace の Git 参照、待機中要求の識別、保存済み snapshot を保護します。容量回収と export・移行は別の未完了作業です。

Environment copy は、停止済みの元 Env から独立データを作る操作です。既存の Incus COW と通常の作成経路を使い、schema 変更や自動 backup は追加しません。[仕様](../design/environment-copy.md)を参照してください。

v0.50 checkpoint の公開 snapshot restore が Workspace／OCI の独立コピー、保存 rootfs からの正規作成と
起動をまとめます。既存 Env 名は拒否し、cleanup が不確実なら所有記録を保持します。
schema 13 は変更しません。Base 実体・自動 backup・完全な runtime 復旧は追加しません。
[snapshot 契約](../design/environment-snapshots.md)を参照してください。既存 Env の置換、
復元先 SSH の受け入れ、live OCI 整合性は未完了です。

新規 Host の所有確認済み OCI 領域は setup で自動接続されます。既存データの移行と runtime 受け入れは partial です。

前の checkpoint v0.39 は Windows 通知 review adapter と distribution 別登録を追加します。実機の通知履歴・protocol 起動・古い要求拒否は成功しましたが、通知からの新規回答と Linux 起動は未完了です。[実装状況](../IMPLEMENTATION_STATUS.ja.md)を参照してください。


partial の承認段階で、通常 Git pending／approve／deny を再利用する保存範囲と
永続化 receipt に接続しました。通知・config 管理全体は未完了です。
installed `eb16300b6700` で専用 GitHub への push、保存 ask の再利用、拒否を確認しました。
他の選択肢は repository 内検証に限定されます。現行 Windows GHA は SSH 成功後の
editor 完了待ちで失敗しています。
[ADR 0026](../adr/0026-reusable-git-approval-scope.ja.md) を参照してください。

現在の checkpoint v0.37 は、承認方針の確認・revision に結び付いた編集を追加します。
保存済み承認と同じ Policy・writer を使い、競合を拒否します。installed 受け入れは未確認です。

先行する checkpoint v0.36 は installed Standard mode の Environment DNS 自動設定と、
停止中 Environment の欠落した source guard を起動前に復元する処理を追加します。
名前解決と接続の許可は別です。repository 回帰テストはありますが、新しい Windows DNS
fixture と実際の再起動・VPN の検証は未完了です。

先行する v0.33–v0.35 の desktop SSH、Host recipe、一時実行は `b6c428d` で
GHA 全 4 系統が成功しました。DNS relay の基礎部分 `3c3c101` も全 4 系統が成功済みです。
ローカルの VS Code 1.136.1 Remote-SSH は、明示的な許可後に古い `8752431` installation
で成功し、一時 rule と検証接続を解除しました。正確な範囲と最初の再開失敗は
[実装状態](../IMPLEMENTATION_STATUS.ja.md)を参照してください。

先行するcheckpoint v0.32では既定Storeの自動初期化、Workspaceへの対応付けと再利用、
公開元専用の状態、任意の`--no-oci`を追加しました。公開済みsourceのコピーはローカルの
componentと実Btrfs合成データ試験で確認しました。HostイメージproducerとDockerの
image/runtime確認は未完了で、B4全体はpartialです。v0.31とSSH公開鍵追加はPR #482の
`f8517ba`で4つのGHA workflowがすべて成功しました。


先行するcheckpoint v0.31では `haco env start <name>` による保持済みEnvironmentの再開を追加しました。
ローカルtest/raceと独立した実Incus/WSLの再開fixtureは成功しました。
インストール済み製品経路も`f8517ba`のGHAで成功しました。SSH setup自動化は未実装で、ロードマップC/E全体の完了ではありません。
先行するv0.30のStore copyとB4の残課題は以下に記録しています。


先行するcheckpoint v0.30は `haco plugin oci store create <target> --from <source>` による
未接続の永続Store独立コピーを追加。repositoryとローカル実Incusの合成データによるCOWを
検証する単位であり、trusted HostからのOCI image配布全体・runtime受入・中断コピーの
回復はpartial。前のv0.29で行ったnative WSL・永続Store・Windows OpenSSHの受入は
`c86c43e`に結び付く。`switch-base`は無効・保留のまま。
[実装状況](../IMPLEMENTATION_STATUS.ja.md)に証拠と制約を記載。


過去のv0.28受入：当時の候補はtrusted WSL Windows連携、複数repo、Workspace保持Base切替、
任意OCI一方向配布、OpenSSH設定生成、読みやすいEnvironment表示を追加した。
B1〜B6はローカル実機確認済み。Dockerとnerdctlの配布・guest独立起動/変更/停止、
B5/B6とA回帰は配布物029ff08で確認済み。依頼されたローカル第二段階の導線は完了した。
公開releaseや広いplatform/image matrixの受入を意味しない。

[English](versioning-and-release-status.md) | **日本語**

Incus起動時のPID guardはv0.28内の保守修正とし、milestone番号は変更しない。
network/proxy process記録を別namespaceのPIDへ再適用する経路を防ぐ。
検証とinstallationの範囲は[実装状況](../IMPLEMENTATION_STATUS.ja.md#incus起動時のpid再利用防止)を参照。

v0.27候補は管理対象repoのWSL利用経路を実装する。独立Workspace copy、標準SSH、
通常Git helperのfetch/pull、送信内容を固定したpush承認、作業を保持する正常停止が対象。
実Gitのローカル回帰は成功し、branchのWindows配布物 `7a4d122` でA1〜A6を受入した。
承認付きremote pushとWorkspace保持停止まで確認済み。手動setupは残り、この候補のfresh導入や
広いhost matrixは未検証。
正確な根拠は[実装状況](../IMPLEMENTATION_STATUS.ja.md)を参照。

> **人間向けcheckpoint policy/status view · 2026-08-31更新**

Hacocoonは **pre-1.0** です。milestone番号はproduct/implementationの進行を表し、compatibility guarantee、release tag、production supportの証明ではありません。

[`checkpoints.yaml`](checkpoints.yaml) が **checkpoint番号・current checkpoint・Gate identity** のmachine-readable正本です。このdocumentは番号付けpolicyと人間が管理するimplementation/acceptance statusを説明します。現在のcode realityとhost-dependent acceptanceは [`../IMPLEMENTATION_STATUS.ja.md`](../IMPLEMENTATION_STATUS.ja.md) を参照してください。

## 番号付けの方針

> **minor milestoneはpre-1.0の軽量な進捗checkpointであり、完成判定ではありません。**

1. 意味のあるproduct、implementation、operator experience、observability、acceptanceの区切りがlandしたら、follow-up slice、hardening、real-host acceptanceが残っていても次の `v0.N` を使ってよい
2. 前のmilestoneがpartialでも、後続milestoneへ進んでよい。version順は時系列であり、過去gateがすべて完了したことを意味しない
3. 粒度は実用優先で決め、pre-1.0では意図的に細かく進めてよい。密接な複数PRを同じmilestoneにまとめてもよく、大きめのfollow-upを次minorへ分けてもよい
4. security/hardening、bug fix、refactor、CLI namespace整理、CI、docs、release engineering、test-only変更は自動的にversionを消費するわけではないが、support、operability、acceptance上の意味あるcheckpointになる場合はminorを使ってよい
5. milestone更新は `tools/bump-milestone` を通し、`checkpoints.yaml`、このhuman-readable table/current宣言、英語mirror、`../IMPLEMENTATION_STATUS.md`、generated build identityをまとめて同期する
6. design-only specificationはfuture numberを予約できるが、実装までは **planned**
7. historical commit/PR/branch/旧document address/過去の番号付けは現在の正本ではない
8. release tagとroadmap milestone番号は別物

## 現在のcheckpoint status

Controller経由setup、trusted network、controller所有Standard proxy、設定/live storageの読み取り専用診断は現在のcheckpoint内でimplemented。`c749ff9033b33c3526e108f60ce2009638075152` のpackageでWindows・Ubuntu・Incus全4job、実機cached BAT適用/再実行・通常/cold入口・readiness 6項目・trusted-hostデータ保持が成功した。

今回指定されたWSL M0–M1の範囲は **implemented、受入済み**。candidate `81c0d16`（同一treeのPR merge `9049df3`）でinstall済みEnvironmentの許可proxy通信/直接通信拒否も成功した。登録時の停止/続行とfresh Windows package gateは `4df465a` で成功した。最新の依頼範囲では実Windows OS再起動と続行案内の追加作り込みを対象外とし、網羅的な受入条件を増やしたり、具体的な変更・失敗なしに成功済みの手元検証を繰り返したりしない。namespaceをまたぐIncus起動時のPID再利用にはADR 0013で対処済み。残るupstreamのprocess lifecycle上の制約は別途記録している。commitを固定した証拠、package識別、受入の制約は[実装status](../IMPLEMENTATION_STATUS.ja.md)を正本とする。

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
| v0.20 | Managed Btrfs Rootfs Storage | Incus-owned loop-backed Btrfs poolとHacocoon rootfs routingを実装済み。broader physical COW/compaction acceptanceはhost-dependent |
| v0.21 | Managed Btrfs Transparent Compression | default Incus pool作成時に `compress=zstd:3` を要求し、`compress-force` は使わない。real compression/performance acceptanceはhost-dependent |
| v0.22 | Interaction Notification Clients | browser、native OS、VS Code notification clientを実装済み。replay/dedup behaviorもtest済み |
| v0.23 | Real Incus E2E Acceptance | GitHub-hosted Ubuntu 26.04でstandalone Incus substrateとHacocoon Core lifecycleをphased gating付きで自動検証 |
| v0.24 | Structured Logging | shared `log/slog`、operation context、sanitize済みDEBUG trace、secret redactionをmaintained executableへ実装済み |
| v0.25 | Incus-owned Btrfs Storage Acceptance | ordinary-user real Incus/Btrfs CLI acceptanceでIncus-owned pool lifecycleとpolicyを検証済み |
| v0.26 | Trusted `haco-host` & Default WSL Entry | persistent trusted logical Host lifecycle、ownership/collision check、managed-storage配置、default WSL entry、recovery path、real Incus acceptanceを実装済み |
| v0.27 | Managed Repository WSL Workflow | 実装済み |
| v0.28 | Multi-repository Development and Optional OCI Distribution | 実装済み |
| v0.29 | Persistent OCI Resources and Native Windows Access | 実装済み |
| v0.30 | Independent Persistent Store Copies | 実装済み |
| v0.31 | Retained Environment Resume | 実装済み |
| v0.32 | Automatic Workspace Store Initialization | 実装済み |
| v0.33 | Desktop SSH Setup | 実装済み |
| v0.34 | Host Setup Recipes | 実装済み・Windows の保存/再実行/更新/解除は bcc1baf で成功 |
| v0.35 | Temporary Execution | 実装済み・通常 run と中断後削除は 4adfe19 の実 Incus で成功 |
| v0.36 | Environment Name Resolution | 実装済み |
| v0.37 | Approval Configuration Editing | 実装済み |
| v0.38 | Pending Approval Review | 実装済み |
| v0.39 | Windows Notification Review | 実装済み |
| v0.40 | Host OCI Area Copy Boundary | partial |
| v0.41 | Interactive Environment Selection | 実装済み |
| v0.42 | Completed OCI Copy Recovery | 実装済み |
| v0.43 | Approved AWS S3 Listing | 実装済み |
| v0.44 | Verified AWS Object Downloads | 実装済み |
| v0.45 | Guest AWS Request Boundary | 実装済み |
| v0.46 | Snapshot Workspace and OCI storage | 実装済み |
| v0.47 | Automatic Base retention | 実装済み |
| v0.48 | Retained Base snapshot capture | 実装済み |
| v0.49 | Snapshot restore staging | 実装済み |
| v0.50 | Public Snapshot Restore | 実装済み |
| v0.51 | Environment Copy | 実装済み |
| v0.52 | Base Builder | 実装済み |
| v0.53 | Workspace Cleanup | 実装済み |
| v0.54 | Base Image Cleanup | 実装済み |
| v0.55 | OCI Store Cleanup | 実装済み |
| v0.56 | Source Repository Cleanup | 実装済み |
| v0.57 | OCI Image Cleanup | partial |

現在のmilestone位置は **v0.57** です。この宣言と上のVersion/Gate列は `checkpoints.yaml` のmirrorで、status列だけを人間が管理します。前のpartial milestoneは残件として追跡しますが、後続のdevelopment checkpointを進める妨げにはしません。

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
- v0.20: Hacocoon所有Incus rootfsはlazyな `haco-local-default` Incus-owned loop-backed Btrfs poolを選択する。physical COW/compaction measurementとbroader supported-host acceptanceはhost-dependent
- v0.21: default Incus-owned Btrfs pool作成時に `compress=zstd:3` を要求し、`compress-force` と `autodefrag` は要求しない。real compression ratio、CPU cost、supported-host behaviorはhost-dependent
- v0.22: browser/native/VS Code notification deliveryとreplay/dedup behaviorはrepository test済み。desktop/session固有のdeliveryは実client環境に依存する
- v0.23: GitHub-hosted Ubuntu 26.04でstandalone Incus system-container behaviorを先に証明してからCore lifecycle E2Eを実行する。CI上のsupport gapは縮まるが、全supported Host/WSL configurationの証明ではない
- v0.24: maintained executable全体でstructured logging/redaction behaviorを共有する。logging policyはdefense in depthであり、unsafeなcall-site dataを出してよいことにはならない
- v0.25: ordinary-user `haco create` / `exec` / `delete` / `run` をreal Incusで検証し、Incus-owned sparse backing file、loop attach、Btrfs mount、zstd policy、pool reuse、cleanupを確認する。broader physical-storage / Windows/WSL acceptanceは引き続き残る
- v0.26: trusted-host creation、exact ownership/collision handling、idempotent ensure、stopped-state recovery、managed-storage配置、raw control-socket非公開をreal Incus acceptanceで検証済み。real Windows/WSL interactive-login behaviorとGit/OCI/credential/control-channelの全面移行はfollow-up

> **意味のあるproduct、operator、observability、acceptanceの進捗がlandしたら次minorへ進めてよい。pre-1.0ではversion番号を節約するよりcheckpointを見える化する。**

d4aef8d の Windows 受け入れ確認では、VS Code に加えて C4 の基本 recipe 操作、C5 の HTTP／Edge preview、C6 の Environment doctor 前提確認が PASS になりました。C4 の再作成・キャンセル、既定ブラウザ起動、VPN／NRPT は別の受け入れ項目です。partial の承認 checkpoint では、追加 CLI 引数なしで名前付き要求を catalog の作成 ID に結び付けます。
