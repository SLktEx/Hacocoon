# 日常の開発手順

日本語 | [English](daily-workflow.md)

状態: **implemented CLI workflow**。provider・desktopの実機確認は[検証証拠](../status/acceptance-evidence.ja.md#development-branch-integration)に別途記録します。

## 最初に準備する

[Windows/WSL installer](../guides/installation.ja.md)でインストールします。Windows端末の`wsl -d <インストールしたディストリビューション名>`は、管理login entry設定済みならtrusted `haco-host`を開きます。haco-hostは信頼された管理基盤です。信頼できないツールはEnv内で実行します。管理コマンドはWSL/Linux Physical Hostからも同じcontrollerへ接続できます。

**trusted haco-host内**で準備します:

```bash
haco doctor
haco repo clone --branch main sample https://github.com/OWNER/REPO.git
haco workspace create --repo sample sample-work
haco env create --workspace managed:sample-work sample-dev
haco open sample-dev
```

OWNER/REPOと既存branchを置き換えてください。private Gitの認証はtrusted haco-hostに保持します。[管理repository手順](../guides/git-workflow.ja.md)を参照してください。Baseは設定済み既定値を使います。任意OCI Storeのコピーは自動で、`--no-oci`で省略できます。CoreにOCI runtimeは必須ではありません。

**WSL/Linux Physical Host内**の既存ファイルなら`haco env create --workspace /absolute/path/to/work sample-dev`も使えます。pathはWindowsやhaco-hostコンテナではなく、そのPhysical Host上のものです。書込み可能なWorkspaceはEnv内の処理からも変更できます。`haco open .`は準備済みの所有者を固定したWorkspace参照を再開します。ディレクトリ内容を暗黙にコピー・マウントしません。[準備とfork](../design/workspace-workflow.md)を参照してください。

`haco open`はdesktop所有のSSH鍵・設定を準備しeditorを起動します。`haco open --client ssh sample-dev`は標準SSH shellを開きます。接続準備だけなら`haco ssh setup sample-dev`です。秘密鍵はdesktopに保持します。editorプロセスの起動だけで接続・build/testが正常とは判定しません。[Windows SSH](windows-environment-ssh.md)も参照してください。

`sshd`を含まないBaseでは、最初のSSH準備時に`openssh-server`を導入する場合があります。
公式Base契約で使うUbuntuの固定package repositoryである`archive.ubuntu.com`、
`security.ubuntu.com`、`ports.ubuntu.com`の通常HTTP/HTTPS portは製品のegress baselineとして
Policy追加なしで利用できます。公式Ubuntu Baseからbuildしたcustom Baseが同じsourceを保持して
いる場合もこのbaselineを使えます。third-party repository、PPA、任意mirrorは自動追加しません。
**trusted haco-host**の`haco config`は管理者Policyを表示し、`haco approve --list`は判断せずに
承認待ちを一覧します。固定package宛先をさらに制限したい場合は一致する`deny`または
`require-approval`を追加し、それ以外のrepositoryやnetwork destinationには通常のPolicy/承認を
使います。[egress baseline](../design/egress-authorization.ja.md#製品既定の-package-repository-許可)を
参照してください。SSH失敗だけでは原因は確定しません。Envの状態・接続とPolicyを確認してから、
明示的にSSH準備をやり直します。導入済みパッケージは停止・起動をまたいでEnvのrootfsに残ります。

## 作業・停止・翌日の再開

**Env内**の`/workspace`で編集し、そのrepositoryのbuild/testを実行します。Go repositoryなら例として`go build ./...`、`go test ./...`です。標準Ubuntu packageのupdate/installは上記の固定baselineを使えます。third-party package repository、その他network access、Git操作には従来どおり該当するPolicy/承認を使います。`haco run --no-oci -- <command>`は別の一時Envでの実行で、指定した常用Env内での実行ではありません。

Env shellを終了するか**trusted haco-host端末**へ戻ります:

```bash
haco env stop sample-dev
# 翌日、trusted haco-host内:
haco env list
haco env status sample-dev
haco env start sample-dev
haco open sample-dev
```

停止ではEnv rootfs、Workspace、OCIデータが残ります。翌日も同じツール・rootfsを使う場合はstopを使ってください。

## 対象・script利用・キャンセル

変更操作には明示的な名前を指定します。openとssh setupは端末の番号一覧から選択でき、空入力なら接続変更前にキャンセルします。Envが一つだけなら自動選択できます。scriptでは常に名前を指定してください。非対話で選択が曖昧な場合は入力待ちにしません。オプションは対象より前に置き、各commandの`--help`で書式を確認します。

結果はstdout、進捗・診断はstderrです。scriptには`haco env list --json`、`haco env status --json sample-dev`を使います。createの既存JSON結果も維持します。保持データの削除は端末確認か明示的な`--yes`が必要で、pipe/FIFOで入力待ちにしません。Ctrl+Cで観測が終わってもcontrollerの変更処理が終わったとは限らないため、再実行前にstatusを確認します。端末終了をcleanup成功と解釈しません。一時実行は別契約で時間制限付きcleanupを要求し、その確認結果を返します。

## 失敗後の操作

| 表示 | 次の操作 |
|---|---|
| setupのrunning / succeeded / failed | 観測した工程です。進捗率ではありません。固定stage/reasonとrequest IDを確認します。 |
| busy | 別操作が対象を使用中です。その操作を確認・待機します。承認待ちとは異なります。 |
| Capabilityの承認待ち | 別のtrusted端末で`haco approve`により一覧・内容を確認します。観測だけでは承認しません。 |
| create/start/stop/delete/SSH失敗 | `haco env status sample-dev`と`haco doctor sample-dev`で確認するまでresource状態は不明です。cleanup済みと推測しません。 |
| SSH準備後のeditor起動失敗 | Envと接続は残ります。`haco open --client ssh sample-dev`を使うかdesktop editorを直します。 |
| setup customization失敗 | 基盤工程が成功している場合もあり、保存scriptと副作用が残ります。内容を確認・修正してから明示的に再実行します。`--clear-script`は保存recipeを除去するだけで副作用を戻しません。 |

setupでは`haco doctor`を実行します。**WSL/Linux Physical Hostの管理者**は次のjournalでsetupの`request_id`を探せます。Envにはこの管理情報を渡しません。

```bash
journalctl -u haco-controller.service --since '30 minutes ago' --no-pager
```

backendの生出力や秘密は診断フィールドに含めません。streamが切れた場合は最終結果が未確認で、controllerのsetupが続いている可能性があります。[setup診断](../design/trusted-host.ja.md#setupの進捗と失敗診断)を参照してください。

## 必要なものだけ削除する

**trusted haco-host内**の`haco env delete sample-dev`は、明示対象と消える・残るデータを表示して既存の正規削除APIを呼びます。Env runtime/rootfs/接続を除去し、Workspace、OCI Store、独立snapshotは保持します。明示Env名を削除意思として扱う既存契約を維持し、新たなpromptや互換性を壊すcommand改名は追加しません。失敗時には不在を確認できておらず、所有権・leaseはlifecycle APIの規則で保持します。

`haco workspace list`、`haco plugin oci store list`、`haco snapshot list`で保持データを確認します。別のWorkspace/Store deleteは消える内容を表示し、確認または`--yes`を要求します。Workspace削除では未commit・未追跡・未pushの作業も消え得ます。必要なデータは独立snapshotやexportに保持してください。今回の変更で保持単位は再設計しません。
