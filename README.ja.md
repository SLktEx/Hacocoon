<div align="center">

<img src="docs/assets/readme/hacocoon-logo.webp" alt="Hacocoon" width="520">

# Hacocoon

**読み方: はこーん**

人間・開発ツール・コーディングエージェントのための、安全な開発作業環境。

[English](README.md) · [はじめて使う](docs/guides/getting-started.ja.md) · [ドキュメント](docs/README.ja.md)

[![CI](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml/badge.svg)](https://github.com/SLktEx/Hacocoon/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

</div>

Hacocoonは、開発ツールを隔離された **Environment** で実行し、プロジェクトのファイルを
永続的な **Workspace** に保存します。Hostの認証情報や外部サービスへのアクセスは
権限の確認を経由します。エージェントにHostの管理権限を渡さずに、編集・ビルド・テストを任せられます。

> [!WARNING]
> Hacocoonは **pre-1.0** です。互換性を壊す変更が今後もあります。
> 現在はUbuntu 26.04+上のIncusを使います。Windowsでは専用のWSL 2ディストリビューションを作成します。
> コード上の実装と実機で確認した範囲は異なります。[実装状況](docs/IMPLEMENTATION_STATUS.ja.md)を確認してください。

## はじめて使う

[インストール → 作成 → 接続 → 開発 → 終了・再開](docs/guides/getting-started.ja.md)に進んでください。
各操作を実行するターミナル、必要な権限、終了後に残るデータを順に説明しています。

インストール後、信頼された管理環境 `haco-host` に入ったら、基本の流れは次のとおりです。

```bash
haco doctor
haco repo clone --branch main sample https://github.com/SLktEx/Hacocoon.git
haco workspace create --repo sample sample-work
haco env create --workspace managed:sample-work sample-dev
haco open --client ssh sample-dev
```

managed Git WorkspaceからEnvironmentを作ると、Git brokerは自動で接続されます。
ネットワークを使う前に、ガイドに従ってGitとパッケージ取得の権限を設定します。
開発シェルを終了し、Hostで `haco env stop sample-dev` を実行すると停止します。
次回は `haco open --client ssh sample-dev` で再開できます。
Remote-SSHを導入したVS Codeを使う場合は `haco open sample-dev` を実行します。

現在の製品CLIは `haco` です。旧CLIだけに残る操作は[CLI移行情報](docs/reference/cli-migration.md)にまとめています。

## 何が残るか

| 対象 | 役割 | Environmentを削除した後 |
|---|---|---|
| Host | 管理操作、認証情報、コントローラーへの接続 | 残る |
| Workspace | プロジェクトのファイルと独立したGit情報 | 未push・未追跡の作業も残る |
| Environment | 実行中のツール、パッケージ、ルートファイルシステム | 削除される |
| Base | 作成時の開始イメージ | 独立して残る。作成後の変更は取り込まない |
| OCI Store | 任意のコンテナイメージ、管理情報、ビルドキャッシュ | 残る。コンテナの自動再開はしない |

停止ならEnvironment自体も残ります。削除すると、そのルートファイルシステムは失われます。
整理する前に[データの寿命](docs/guides/data-lifetime.ja.md)を確認してください。
エージェントは書き込み可能なWorkspaceを変更できます。隔離はバックアップの代わりにはなりません。

## 次に読む

- [目的別ガイドと参照資料](docs/README.ja.md)：Git承認、SSH、セットアップ、プレビュー、保存・削除。
- [Base](docs/design/base-images-and-custom-environments.md)：`haco base list`、内容の確認、ビルド。
- [任意のOCI Store](docs/design/persistent-oci-store.md)：`haco plugin oci`。CoreはDockerやcontainerdを必須にしません。
- [セキュリティ設計](docs/security/security-architecture.md)：共有カーネルの限界と権限境界。
- [実装状況](docs/IMPLEMENTATION_STATUS.ja.md)、[ロードマップ](docs/status/architecture-and-roadmap.md)、[バージョンと公開状況](docs/status/versioning-and-release-status.ja.md)。
- [開発参加とローカルCI](CONTRIBUTING.md)：ビルド、検証、現在の共同開発者限定のPR方針。

ライセンスは[Apache License 2.0](LICENSE)です。
