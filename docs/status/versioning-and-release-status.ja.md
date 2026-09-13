# バージョンとリリース状況

[English](versioning-and-release-status.md) | 日本語

Hacocoonはpre-1.0です。checkpointは進捗の節目であり、互換性保証や公開リリース、実機対応状況とは別です。[checkpoints.yaml](checkpoints.yaml)が番号・現在値・Gate名の正本です。[実装状況](../IMPLEMENTATION_STATUS.ja.md)と[検証証拠](acceptance-evidence.ja.md)で現在の範囲を確認してください。

## 番号の方針

- 小さなpre-1.0の節目として、意味のある機能・運用・観測・検証の進展に次のminorを使えます。
- 以前の項目が部分実装または実機未確認でも、後の節目へ進めます。番号順は完了順ではありません。
- 修正・文書・テスト・CI・リファクタリングだけで自動的に番号を上げません。意味のある新しい進展かを判断します。
- 番号変更には `tools/bump-milestone v0.N "Gate Name"` を使い、YAML、英日表、実装状況、生成ビルド情報を揃えます。
- 設計だけの計画を実装済みとしません。古いコミット、PR名、開発ブランチの番号は正本ではありません。
- release tagとroadmap milestone番号は別物です。公開作業は[リリース手順](../guides/releasing.ja.md)に従います。

## checkpoint履歴

下表のVersion・Gate列はYAMLの写しです。Gate名は識別子として英語を維持します。
表は各節目の履歴を表し、現在の公開CLIにすべてが残るという意味ではありません。
現在の詳細と残課題は機能別の実装状況・ロードマップへ集約しています。

| Version | Gate | 節目の状態 |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | implemented（現行の機能制約は実装状況を参照） |
| v0.2 | Workspace Abstraction & Lease | implemented（現行の機能制約は実装状況を参照） |
| v0.3 | Client & Interactive Access | implemented（現行の機能制約は実装状況を参照） |
| v0.4 | Policy & Capability Foundation | implemented（現行の機能制約は実装状況を参照） |
| v0.5 | Git / GitHub Capability | implemented（現行の機能制約は実装状況を参照） |
| v0.6 | Agent & Orchestrator Integration | implemented（現行の機能制約は実装状況を参照） |
| v0.7 | Remote / Cloud Runtime & External Capabilities | deferred（具体的クラウドproviderは削除、routing境界は保持） |
| v0.8 | Client Adapters & VS Code Integration | implemented（現行の機能制約は実装状況を参照） |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | implemented（現行の機能制約は実装状況を参照） |
| v0.10 | VS Code Remote Agent Host Adapter | implemented（現行の機能制約は実装状況を参照） |
| v0.11 | Base Images & Custom Environments | implemented（現行の機能制約は実装状況を参照） |
| v0.12 | Sandbox Resource Limits | implemented（現行の機能制約は実装状況を参照） |
| v0.13 | Managed Sandbox Network | implemented（現行の機能制約は実装状況を参照） |
| v0.14 | Git Fetch Plugin | 旧方式はimplemented（移行用hacoq。通常のStore手順とは別） |
| v0.15 | OCI Seed Recommendation | 旧方式はimplemented（移行用hacoq。通常のStore手順とは別） |
| v0.16 | OCI Image Deletion | 旧方式はimplemented（移行用hacoq。通常のStore手順とは別） |
| v0.17 | OCI Seed Builder & Btrfs/COW | partial（実装・検証の残課題あり） |
| v0.18 | Docker Compatibility Plugin | 旧方式はimplemented（移行用hacoq。通常のStore手順とは別） |
| v0.19 | Domain-aware Egress Authorization | implemented（現行の機能制約は実装状況を参照） |
| v0.20 | Managed Btrfs Rootfs Storage | implemented（現行の機能制約は実装状況を参照） |
| v0.21 | Managed Btrfs Transparent Compression | implemented（現行の機能制約は実装状況を参照） |
| v0.22 | Interaction Notification Clients | implemented（現行の機能制約は実装状況を参照） |
| v0.23 | Real Incus E2E Acceptance | implemented（現行の機能制約は実装状況を参照） |
| v0.24 | Structured Logging | implemented（現行の機能制約は実装状況を参照） |
| v0.25 | Incus-owned Btrfs Storage Acceptance | implemented（現行の機能制約は実装状況を参照） |
| v0.26 | Trusted `haco-host` & Default WSL Entry | implemented（現行の機能制約は実装状況を参照） |
| v0.27 | Managed Repository WSL Workflow | implemented（現行の機能制約は実装状況を参照） |
| v0.28 | Multi-repository Development and Optional OCI Distribution | historical（永続resource・独立保存rootfs方式へ置換済み） |
| v0.29 | Persistent OCI Resources and Native Windows Access | implemented（現行の機能制約は実装状況を参照） |
| v0.30 | Independent Persistent Store Copies | implemented（現行の機能制約は実装状況を参照） |
| v0.31 | Retained Environment Resume | implemented（現行の機能制約は実装状況を参照） |
| v0.32 | Automatic Workspace Store Initialization | implemented（現行の機能制約は実装状況を参照） |
| v0.33 | Desktop SSH Setup | implemented（現行の機能制約は実装状況を参照） |
| v0.34 | Host Setup Recipes | implemented（現行の機能制約は実装状況を参照） |
| v0.35 | Temporary Execution | implemented（現行の機能制約は実装状況を参照） |
| v0.36 | Environment Name Resolution | implemented（現行の機能制約は実装状況を参照） |
| v0.37 | Approval Configuration Editing | implemented（現行の機能制約は実装状況を参照） |
| v0.38 | Pending Approval Review | implemented（現行の機能制約は実装状況を参照） |
| v0.39 | Windows Notification Review | implemented（現行の機能制約は実装状況を参照） |
| v0.40 | Host OCI Area Copy Boundary | partial（実装・検証の残課題あり） |
| v0.41 | Interactive Environment Selection | implemented（現行の機能制約は実装状況を参照） |
| v0.42 | Completed OCI Copy Recovery | implemented（現行の機能制約は実装状況を参照） |
| v0.43 | Approved AWS S3 Listing | implemented（現行の機能制約は実装状況を参照） |
| v0.44 | Verified AWS Object Downloads | implemented（現行の機能制約は実装状況を参照） |
| v0.45 | Guest AWS Request Boundary | implemented（現行の機能制約は実装状況を参照） |
| v0.46 | Snapshot Workspace and OCI storage | implemented（現行の機能制約は実装状況を参照） |
| v0.47 | Automatic Base retention | historical（永続resource・独立保存rootfs方式へ置換済み） |
| v0.48 | Retained Base snapshot capture | historical（永続resource・独立保存rootfs方式へ置換済み） |
| v0.49 | Snapshot restore staging | historical（永続resource・独立保存rootfs方式へ置換済み） |
| v0.50 | Public Snapshot Restore | implemented（現行の機能制約は実装状況を参照） |
| v0.51 | Environment Copy | implemented（現行の機能制約は実装状況を参照） |
| v0.52 | Base Builder | implemented（現行の機能制約は実装状況を参照） |
| v0.53 | Workspace Cleanup | implemented（現行の機能制約は実装状況を参照） |
| v0.54 | Base Image Cleanup | implemented（現行の機能制約は実装状況を参照） |
| v0.55 | OCI Store Cleanup | implemented（現行の機能制約は実装状況を参照） |
| v0.56 | Source Repository Cleanup | implemented（現行の機能制約は実装状況を参照） |
| v0.57 | OCI Image Cleanup | partial（実装・検証の残課題あり） |
| v0.58 | Daily CLI Entry and Setup Diagnostics | implemented on development candidate |
| v0.59 | All-branch Git Fetch | 開発候補に実装済み、実Env受入は未確認 |
| v0.60 | Reviewed Git Branch Creation | 実装済み |
| v0.61 | Local GUI Approval Review | 実装済み |
| v0.62 | Temporary Process Streams | 開発候補に実装済み、実機pipe／TTY受入は未確認 |
| v0.63 | Durable Git Push Reconciliation | 実装済み |
| v0.64 | Notification-contained Approval | 実装済み |
| v0.65 | Client TCP Stream Forwarding | 実装済み |

現在のmilestone位置は **v0.65**。上表とこの値はYAMLの写しです。

具体的なクラウドproviderとlocal registryは延期中です。local registryは必須の節目ではなく、番号も予約していません。Base実体の自動保持（旧v0.47–v0.49）は[ADR 0040](../adr/0040-incus-first-snapshots.md)の方式へ置き換わっています。

PR #583のM0/M1候補は、既存開発ブランチとIncus 7.0 LTS共通導入・doctor契約を統合します。
実機受入がpartialのため、開発checkpointはv0.58を維持します。新規タグ・mainマージ・
インストーラ配布・公開リリースではありません。

別のM2候補は全headsの列挙・各refの読み取り承認・通常のブランチ切り替えをv0.59に記録します。
M1の実機残件はこの軽量な開発checkpointを止めません。Gitガイドに制限を記載し、
M2全体の完了・巨大pack対応・リリースとは区別します。

v0.60はGitブランチ一つの新規作成と、正確な既存ブランチ一つの更新を別々に承認する開発単位です。
実Git componentと全体local test CIが成功し、実EnvのGit、GUI回答、巨大レポ受入は未完了です。

v0.61はVS Codeのterminal入力をローカルGUI回答へ置き換え、privateで上限付きのsessionから既存approval/Policy serviceを使います。Windows通知内回答は残件です。リリースやM2完了を意味しません。

v0.62は共通の作成／cleanup lifecycleを使い、一時実行にstdinと任意TTYを追加します。
pipeの出力分離・実際の終了コード・入力上限・cleanup確認を維持します。実機pipe／端末の
受入は未確認であり、M3完了や配布済みのリリースではありません。

v0.64はWindowsの端末回答を、通知内のページ・選択欄・COM受信へ置き換え、既存の非公開確認sessionを再利用します。開発checkpointでありM2/M3完了や配布済みreleaseではありません。導入済み新規回答と表示の見切れは未確認です。

## ロードマップ候補へのmain統合

mainの`74bc2205`には開発ブランチ統合（#581）、通常の人向け出力と明示的な
`--json`（#602）、信頼済みHostのGit/gh（#597）、Hostの作成世代ごとの
script適用と結果保持（#604）が含まれます。この候補は上記v0.59〜v0.64の
開発成果と合わせ、一時実行の世代照合と日英client表示を維持します。

mainは#604で「Host customization lifecycle and results」に独立してv0.59を
割り当てています。そのmain側checkpointは履歴として残し、この候補は既存YAMLの
番号列と統合時点のv0.64を維持しました。この統合はrelease配布、tag変更、候補のmain反映を
意味しません。

v0.65はprivate controller byte sessionによるclient側TCP待受を追加する
開発checkpointです。作成世代の固定、接続結果、半切断、完了・回収は既存の
controller／provider境界を共有します。Linux componentとWindows nativeの
transport基本処理は限定した検証範囲です。導入済みIncus受入とWindows側待受から
`wsl.exe`を通す経路は別残件であり、M3完了や配布済みreleaseではありません。
