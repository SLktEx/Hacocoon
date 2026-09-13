# ドキュメント

[English](README.md) | 日本語

## はじめて使う

[利用開始ガイド](guides/getting-started.ja.md)で、インストールから環境作成、接続、
開発、停止・再開まで進めます。設計資料を先に読む必要はありません。
Hacocoonはpre-1.0です。[使える範囲と制約](IMPLEMENTATION_STATUS.ja.md)も確認してください。

## 目的別の操作

- [インストール・中断後の再実行](guides/installation.ja.md)
- [Git・複数リポジトリ・push承認](guides/git-workflow.ja.md)
- [停止・削除・再作成とデータの寿命](guides/data-lifetime.ja.md)
- [Envの移送](design/environment-transfer.ja.md#commands)と[データ退避・移行の確認](guides/data-evacuation.ja.md)
- [容量回収と結果の確認](design/storage-reclamation.ja.md#起動と結果確認)
- [Host の準備](design/trusted-host.ja.md)・[プロジェクトのセットアップ](design/project-setup.ja.md)、[Webプレビュー](design/development-preview.ja.md)、[一時実行](design/temporary-execution.ja.md)

- [日常操作とsetup診断](reference/daily-workflow.ja.md)
- [Workspaceの準備・再開・fork](design/workspace-workflow.md)
- [明示的なTCP/UDP開発接続](design/network-connections.md)

## 用語・仕組み

- [Host・Workspace・Environment・Base・OCI Storeと寿命](guides/data-lifetime.ja.md)
- [正式な用語と責任境界（英語）](reference/terminology-and-boundaries.md)
- [設計原則](DESIGN_PRINCIPLES.ja.md)と[セキュリティ境界（英語）](security/security-architecture.md)

## コマンド・設定の参照

- [現行CLI](reference/cli.ja.md)と[旧CLIの移行情報（英語）](reference/cli-migration.md)
- [設定と承認ポリシー](reference/configuration.ja.md)、[通信の認可](design/egress-authorization.ja.md)、[AWS操作](design/aws-operations.ja.md)
- [クライアントAPI](reference/client-adapter.ja.md)、[通知イベント](reference/interaction-events.ja.md)、[ログ](reference/logging.ja.md)
- [ビルド・リリースの識別](reference/build-release-identity.ja.md)

## 内部設計・開発参加

[開発参加方針](../CONTRIBUTING.md)からビルド・CIへ進んでください。
[設計文書](design)と[ADR](adr)は実装する機能に応じて読みます。
入口となる設計は[Workspaceとリース](design/workspace-abstraction-and-lease.md)、
[コントローラー通信](design/controller-client-transport.ja.md)、
[信頼済みHost](design/trusted-host.ja.md)、[Core・Standard・Plugin](design/plugin-architecture.md)です。

[文書の役割・正本・更新ルール](DOCUMENTATION_STYLE_GUIDE.md)は一か所で管理します。
公開準備は[公開設定の点検](guides/releasing.ja.md)と[リリースの安全性](security/release-security.ja.md)を参照してください。

## 現在の状況・今後の計画

- [実装状況](IMPLEMENTATION_STATUS.ja.md): 機能、使える範囲、制約、残課題
- [検証証拠](status/acceptance-evidence.ja.md): 実機結果、失敗、未確認事項
- [ロードマップ（英語）](status/architecture-and-roadmap.md): 未完了の作業と将来計画
- [バージョンとリリース状況](status/versioning-and-release-status.ja.md): チェックポイント方針と履歴
