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
| v0.15 | OCI Seed Recommendation | historical（Seedの実装を撤去） |
| v0.16 | OCI Image Deletion | historical（現行の管理対象イメージ削除へ置換） |
| v0.17 | OCI Seed Builder & Btrfs/COW | historical（Seedの実装を撤去） |
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
| v0.58 | Daily CLI Entry and Setup Diagnostics | 実装済み |
| v0.59 | Host customization lifecycle and results | 実装済み |
| v0.60 | Git branch read and push authority | 実装済み |
| v0.61 | Interactive temporary execution | 実装済み |
| v0.62 | Packer HCL2 Base builds | 実装済み |
| v0.63 | Ordinary Environment cache collection | 実装済み |
| v0.64 | Client TCP Forwarding | 実装済み |
| v0.65 | Environment Resolver Selection | 実装済み |
| v0.66 | Saved Environment data | 実装済み |
| v0.67 | Portable Environment data | 実装済み |

現在のmilestone位置は **v0.67**。上表とこの値はYAMLの写しです。

具体的なクラウドproviderとlocal registryは延期中です。local registryは必須の節目ではなく、番号も予約していません。Base実体の自動保持（旧v0.47–v0.49）は[ADR 0040](../adr/0040-incus-first-snapshots.md)の方式へ置き換わっています。

## 開発ブランチの統合

`dev/1.x`、`dev/v2`、`dev/2.x`のライフサイクル整理、日常操作の診断、明示的なTCP/UDP接続、
パスによるWorkspace再開とデータforkをmainへ統合しました。取り込んだv0.58の節目を維持し、
統合によるリリースやタグは作成しません。[現在の範囲](../IMPLEMENTATION_STATUS.ja.md)と
[限定された検証証拠](acceptance-evidence.ja.md#development-branch-integration)を参照してください。

## mainへの言語対応統合

既存v0.59の範囲で、#580/#583の日常CLIの日英表示・ヘルプを部分統合します。
開発成果の再利用であり、新しい段階やリリースの公開ではありません。
Windowsの表示とGUI受け入れは別途確認します。
[言語対応範囲](../reference/cli-language.ja.md)を参照してください。

次のv0.59 M1候補は、同種失敗の通知集約とBATの最終結果表示を再利用します。
ローカル回帰・実Windowsコンポーネント確認は限定した証跡であり、段階番号や
リリース識別は変更しません。[確認範囲](acceptance-evidence.ja.md#main-notification-installer)を参照してください。

同じv0.59の範囲で、Hostセッションへの日英表示の引き継ぎと、Windows表示言語の
読み取りを統合します。インストーラはOSの言語設定を保持します。新しい節目や
リリース番号は作成しません。

## 現在の開発節目でのSeed撤去

v0.59のM5候補で、Seedの実行経路とカタログ、収集・推奨、旧削除・再有効化の状態を
撤去します。現行のBaseと永続OCIイメージ操作は維持します。2026-09-15のユーザーの
範囲変更により、旧バージョンとの互換性・移行はM0〜M5の完了条件に含めません。
現在の節目内の整理であり、タグやリリースは作成しません。

## Gitブランチ操作の統合

v0.60は #585/#587 をmainへ再利用した全head取得と、新規branch/fast-forwardの
明示的なpush権限を記録します。M2全体の完了や配布を意味しません。
GUI、認証付きの導入実機、大容量packは別に残ります。
[検証範囲](acceptance-evidence.ja.md#main-git-branches)を参照してください。

## 一時実行の対話操作

v0.61は標準入力・TTY・逐次出力と、作成IDに固定したcleanupを記録します。
main向けGitの節目（#663）に続く開発上の進捗であり、M3全体やWindows・Incusの
受入完了、配布を意味しません。[確認範囲](acceptance-evidence.ja.md#main-interactive-run)を参照してください。

## Workspaceのレポ選択と既存worktree入力

v0.67内のM2候補として、停止した作業のレポ選択コピーとcheckout/linked worktreeの独立取り込みを追加します。
既存の契約を使い、旧版互換は追加しません。実装、導入後受入、後続の性能確認は分け、タグ・リリースは作成しません。


## 保持キャッシュの整理

現在のv0.67候補で、生成元削除後も全再利用元の一覧・完了コピー復旧・確認付き整理を追加します。
既存契約内でM4の使いやすさを補う変更であり、タグ・リリース・旧版移行は追加しません。

## 保存データの削除診断

v0.67内のM5候補として、snapshotの構成ごとの読み取り専用診断と再試行案内を追加した。
M5全体、配布、Btrfs内部の整合性の確認完了とは数えない。

## Baseアーカイブ取り込み

v0.67内のM4候補として、隔離したBaseアーカイブ取り込みと作成環境の有限の資源上限を
追加する。入力の一時保存と既存の公開・cleanup処理を再利用する。開発段階の記録であり、
リリースやM4/M5全体の確認完了ではない。

## 通知セッションの引き継ぎ

v0.67内で、Windows候補は処理の所有と表示準備完了を分け、前の処理のcleanup待ちと
自身の起動に別の上限を設けます。日常利用の起動競合を修正するもので、人による回答の
受け入れ完了や配布ではありません。[ADR 0100](../adr/0100-notification-session-readiness.ja.md)を参照してください。

## Env内キャッシュの掃除

v0.67のM4開発候補に、保持データを残した確認付きcache emptyを追加します。
実装と実機受入を分け、M4/M5全体の完了、タグ、リリースとは扱いません。
