# はじめての開発環境

[English](getting-started.md) | 日本語

現在の製品CLI `haco` で、GitHubのリポジトリから一つのEnvironmentを作り、
開発して再開するまでの手順です。ローカル実行機能は実装済みですが、
実機で確認した範囲には制限があります。[実装状況](../IMPLEMENTATION_STATUS.ja.md)を参照してください。

## インストールしてHostに入る

Ubuntu 26.04+とsystemd・Incus、または現在のWSL 2を導入したWindowsを使います。
[リリース](https://github.com/SLktEx/Hacocoon/releases)からCPUの種類に合う
インストーラーパッケージを選びます。開発用スナップショットは公開リリースとは別です。
ソースからのビルドは[開発参加](../../CONTRIBUTING.md)を参照してください。
古いパッケージにmainの全機能が含まれるとは限りません。

WindowsではZIP全体を展開し、そのディレクトリでPowerShellを開いて実行します。

```powershell
.\install-windows.bat
wsl -d Hacocoon
```

専用ディストリビューションとコントローラーが用意され、通常の対話的なWSL起動で
信頼された管理環境 `haco-host` に入ります。Windows版の `haco.exe` はありません。
WSLへコマンドを明示して渡す場合はPhysical Host側で実行されます。

Ubuntuへ直接導入する場合は、Ubuntu用パッケージ全体を展開して `./install-ubuntu.sh` を実行します。
製品コマンドは、そのPhysical Host上のコントローラー接続グループに所属するユーザーで実行します。
Gitの認証は信頼されたHost内で行います。Hostシェルへ明示的に入る一時的な方法は
[CLI移行情報](../reference/cli-migration.md#host-entry)にあります。
Ubuntuへの導入ではログインシェルを変更しません。

中断した登録、現在のパッケージでの再試行、プラットフォーム別の設定、rootでの復旧入口は
[インストールと復旧](installation.ja.md)を参照してください。

**信頼された管理ターミナル**で確認します。

```bash
haco version --json
haco doctor
```

問題を報告するときはビルド情報を添えます。doctorの全項目が成功してから先へ進んでください。
保留・失敗・確認不能は準備完了ではありません。表示された診断に従って解消します。

## 独立したプロジェクトデータを作る

次の操作はEnvironmentではなく、**信頼されたhaco-host内**で行います。
`haco setup` は標準Hostツールとして `git` とGitHub CLI (`gh`) を保証するため、
手動でパッケージを導入する必要はありません。非公開リポジトリの利用やpushが必要な場合だけ
GitHub CLIで認証します。

```bash
# 非公開リポジトリの利用、または書き込み権限があるリポジトリへのpush用:
gh auth login --hostname github.com --git-protocol https
```

認証情報、dotfiles、個人・組織固有の追加ツールは標準Hostツールには含めません。

以下の公開リポジトリは読み取りとローカル編集に使えます。自分の開発では、
URLとブランチを利用権限のあるリポジトリの**既存ブランチ**に置き換えます。
後で設定する権限のURL・名前も一致させてください。

```bash
haco repo clone --branch main sample https://github.com/SLktEx/Hacocoon.git
haco workspace create --repo sample sample-work
haco env create --workspace managed:sample-work sample-dev
haco env status sample-dev
```

取得元のチェックアウトは信頼されたHostに残ります。Workspaceにはファイルと `.git` の
独立したコピーが作られ、Environmentの `/workspace` に接続されます。
Gitのremoteは `haco://sample` となり、Environment作成時にmanaged Git brokerが自動で
配線されます。この配線だけでは上流remoteへ通信せず、実際のネットワーク通信は後で
`git fetch` や `git push` などを実行した時点で始まります。

作成時は既定のBaseを使い、設定されていればWorkspace専用のOCI Storeをコピーまたは再利用します。
コンテナを使わない場合の明示的な選択肢は
`haco env create --no-oci --workspace managed:sample-work sample-dev` です。
二つの作成を両方実行しないでください。設定済みのコピーが失敗したときに、
空のStoreへ黙って置き換えることはありません。[OCI Storeの制約](../design/persistent-oci-store.md)を参照してください。

## 必要な通信だけを許可する

**信頼された管理ターミナル**で `haco config --edit` を実行します。
revisionと既存のPolicyルールを保持してください。編集方法と承認保存との関係は
[設定の参照](../reference/configuration.ja.md)にあります。

SSHの準備ではsshdの導入が必要になるため、先にEnvironmentが利用するパッケージ配布先を許可します。
[外向き通信の例](../design/egress-authorization.ja.md#policy例)に従い、
`network.egress/connect` の対象Environmentを `sample-dev`、
resourceを実際の配布先ホスト名、プロトコルとポートを対応するHTTP／HTTPSの値にします。
Ubuntuの配布先はBaseやCPUの種類で異なります。無制限のワイルドカードで代用しないでください。
必要なパッケージを含むBaseを用意する方法もあります。

通常のfetch／pullと承認付きpushには、
[管理対象Gitの権限設定](git-workflow.ja.md#configure-git-policy)から
登録したURL・ブランチに合うルールを追加します。
アプリが自分で名前解決する場合は、別途 `network.resolve/lookup` の権限が必要です。
[名前解決](../design/name-resolution.ja.md)の許可は接続の許可を兼ねません。
Hostが仲介するGit操作のために、EnvironmentへGitHub認証情報を渡す必要はありません。

## 接続して開発する

**信頼された管理ターミナル**で実行します。

```bash
haco open --client ssh sample-dev
```

デスクトップ側が所有するSSH鍵と、プロバイダー経由で取得したサーバー公開鍵の照合設定が作られ、
`/workspace` のシェルを開きます。WindowsではWSL連携経由でWindows OpenSSHを使います。
通常のHostターミナルを開いたままにしてください。
VS CodeとRemote-SSHをデスクトップに導入済みなら `haco open sample-dev` で開けます。
手動接続や失敗時の確認は[SSHの詳細](../reference/windows-environment-ssh.md)を参照してください。

**EnvironmentのSSHセッション内**で作業します。

```bash
cd /workspace
apt-get update
apt-get install -y git
git status
git fetch origin
git pull --ff-only
# ファイルを編集し、プロジェクトのビルドやテストを実行する。
git config user.name 'Your Name'
git config user.email 'your-address@example.com'
git add <files>
git commit -m 'Describe the change'
```

`<files>` は追加するファイルに置き換えます。
管理対象のSSH設定が認証情報を含まないプロキシ設定を渡しますが、通信にはPolicyの許可が必要です。
開発ツールはEnvironmentに導入するかBaseに含めます。
依存関係を繰り返し導入するには[セットアップ手順の保存](../design/project-setup.ja.md)を使えます。

書き込み権限のある接続先だけにpushしてください。Environmentで `git push` を実行し、
待機中に別の**信頼されたHostターミナル**で `haco git pending` を開きます。
接続先・ref・変更前後のコミットを確認して、
`haco git approve <id>` または `haco git deny <id>` を実行します。
この例の公開Hacocoonリポジトリを使っても、上流への書き込み権限は得られません。
詳細は[Gitの承認と結果不明時の確認](git-workflow.ja.md)にあります。

## 終了して、後で再開する

SSHシェルを終了し、**Host**で実行します。

```bash
haco env stop sample-dev
haco env status sample-dev
```

停止状態とWorkspaceの保持を確認します。停止では利用権の予約、ルートファイルシステム、
Gitの変更、任意のStoreが残ります。ただし `/tmp` はゲストOSの起動時に消去される場合があります。
シェルを閉じるだけではEnvironmentは停止しません。接続を明示的に取り消す場合は
`haco env disconnect sample-dev <connection-id>` を使います。

次回Hostに入り直したら、次の操作で再開します。

```bash
haco open --client ssh sample-dev
```

所有権とネットワークを確認してから停止中のEnvironmentを再開します。
`haco env start sample-dev` も使えます。
古いインスタンスを更新した場合はHostを再起動する前に通常の停止／起動を行います。
理由と手順は[ライフサイクル仕様](../design/workspace-abstraction-and-lease.md#explicit-start-after-a-physical-host-boot)を参照してください。

## 削除は目的を確認して行う

`haco env delete sample-dev` はEnvironment内だけのファイルとパッケージを削除します。
WorkspaceとOCI Storeは残ります。pushやバックアップを行う操作ではありません。
再作成、スナップショット、残ったデータの個別削除は[データの寿命と整理](data-lifetime.ja.md)を参照してください。

作成・コピー・再開・削除で所有権が確認できないと表示されたら、
`haco env list`、`haco workspace list` と報告されたIDを確認します。
カタログの編集、推測したIncusパスの削除、隔離の無効化で続行しないでください。
中断した操作を一般的に復旧する機能は未完成です。
