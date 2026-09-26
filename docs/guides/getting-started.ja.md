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
Gitの認証は信頼されたHost内で行います。native Ubuntuには現在、製品の対話的なtrusted Hostシェル接続コマンドがありません。
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

## リポジトリを登録してHacoを開く

インストール後、**信頼された管理ターミナル**で実行します。
OWNER/APIは利用できるリポジトリに置き換えてください。登録なしでも空の作業領域で開始できます。
非公開リポジトリの認証は信頼されたhaco-host内で
`gh auth login --hostname github.com --git-protocol https`を実行します。
GitとGitHub CLIはインストール時に標準Hostツールとして準備されます。

```bash
haco repo add https://github.com/OWNER/API.git
haco open
```

既定の接続先はデスクトップのVS Codeです。Remote-SSHも導入してください。
シェルを使う場合は代わりに`haco open --client ssh`を実行します。
独立した作業ファイル、既定イメージ、設定済みストレージ、開発環境を準備し、
SSH接続を設定してクライアントを起動します。端末の進捗を確認してください。
Workspace・Envの作成やBaseのbuildを手動で行う必要はありません。

1個なら`/workspace`、複数なら`/workspace/<repository-id>`で編集します。
自動生成されたIDは`haco repo ls`で確認できます。Git情報も独立しており、
remoteは`haco://<id>`を使います。Git brokerは自動接続し、Host認証情報はコピーしません。
`haco open`は最後に開いたEnvironmentへ戻り、まだなければ既定Imageから作成します。
`haco image default [IMAGE]`で既定値を確認・変更し、`haco open --new [IMAGE]`で新しい作業を始めます。

## 必要な権限を確認する

openは通信やGitの権限を与えません。承認待ちは別の**信頼された管理端末**で
`haco approve`を実行し、具体的な要求を確認します。正規の回答後は処理を続行します。
Policyで拒否している場合は承認promptを作らないため、`haco config --edit`で
必要な範囲を確認し、`haco open`を再実行します。作業データは保持します。

初回のSSH準備では`openssh-server`の取得が必要な場合があります。
`haco env list`に表示された環境について、実際のパッケージ取得先hostname・protocol・port
だけを[egressのPolicy例](../design/egress-authorization.md#policy-example)に従って許可します。
無制限のワイルドカードを追加しないでください。sshdを含むBaseならこの取得は不要です。
接続失敗だけで原因を決めず、`haco doctor`と表示された処理段階を確認してください。

fetch/pullや承認付きpushは[Git権限](git-workflow.ja.md#gitの権限設定)に従います。
名前解決と接続は別々に制御します。ローカルで編集・buildできてもremoteへの書込み権限は得ません。

## 開発して、後から再開する

**Environment内**で対象リポジトリへ移動します。

```bash
cd /workspace
git status
# 編集し、このリポジトリのbuild/testを実行します。
```

プロジェクト固有のツールは、許可済みのパッケージ取得や
[セットアップ手順](../design/project-setup.ja.md)で導入します。BaseにGitがない場合も
同じ許可経路で導入します。commit用の名前・メールはEnvironment内で設定します。
通常の`git fetch`、`git pull --ff-only`、`git push`はbrokerを使い、pushには
具体的な要求の承認が必要です。Host認証情報は渡しません。[Git手順](git-workflow.ja.md)を参照してください。

終わったらシェルを抜けるかエディタを閉じます。これだけでは環境を停止・削除しません。
次回は信頼された管理端末で実行します。

```bash
haco open
```

正常な作業環境を再利用し、停止していれば再開します。エディタの未保存内容は保存が必要です。
openはバックアップ作成やpushを行いません。

## 明示的な制御と復旧

通常は登録とopenだけです。意図的な停止や独自設定には、既存の
[詳細CLI操作](../reference/cli.ja.md)を使えます。

```bash
haco env list
haco env stop <environment>
haco open
```

一覧の実際の名前を指定します。停止なら導入したパッケージ、編集内容、設定済みの保存領域が
残ります。`haco env delete <environment>`はrootfsと自動生成したWorkspace・OCIデータを
削除し、明示Volumeは保持します。先に停止するか`-f`を指定します。
削除前に[データの寿命](data-lifetime.ja.md)を確認してください。
独立管理データを使う明示的なディレクトリ操作も利用できます。

登録変更は今後作るEnvironmentだけに反映します。現在の登録集合を使うなら
`haco open --new`、保存済み構成・データを使うなら`haco open --new --snapshot SNAPSHOT`です。
既存作業を自動で作り直すことはありません。所有権が未確定なら状態を確認し、
カタログを編集したり推測でprovider資源を削除したりしないでください。
[作成仕様](../design/environment-creation.ja.md)を参照してください。

リポジトリテストと実機確認は別です。この一連の操作の新規インストール済みIncus・
Windows/WSL・デスクトップでの確認は未実施です。
