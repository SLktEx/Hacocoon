# 日常の開発手順

日本語 | [English](daily-workflow.md)

状態: **implemented CLI workflow**。provider・desktopの実機確認は[検証証拠](../status/acceptance-evidence.ja.md#development-branch-integration)に別途記録します。

## 登録して、開く

[インストール](../guides/installation.ja.md)後、信頼された管理端末で実行します。

```bash
haco repo add api https://github.com/OWNER/API.git
haco repo add web https://github.com/OWNER/WEB.git
haco open
```

URLを作業対象に置き換えてください。独立したファイル、既定Base、設定済みの保存領域を
準備し、環境を作成・再利用してRemote-SSH導入済みのVS Codeを開きます。
シェルなら`haco open --client ssh`です。認証情報は信頼されたHostに保持します。
編集先は`/workspace/api`・`/workspace/web`で、リポジトリ1個なら`/workspace`です。

進捗はstderrに表示します。承認待ちがある場合は別の信頼された端末で`haco approve`を
実行すると、回答後に処理を続行します。既定denyではpromptを作りません。
`haco config --edit`で必要なパッケージ・Gitの権限範囲を確認し、`haco open`を再実行します。
登録やopenがpush・無制限の通信を許可することはありません。
[初回の権限確認](../guides/getting-started.ja.md#必要な権限を確認する)を参照してください。

## 作業に戻る

同じ管理ユーザーのhomeから再び`haco open`を実行します。同じ作業を再利用し、停止済みなら
再開します。エディタやシェルの終了だけでは停止しません。意図的に停止する場合は
`haco env list`で名前を確認し、`haco env stop <environment>`を実行します。
次回の`haco open`だけで再開でき、startは不要です。停止なら導入ツール・rootfs・編集・
設定済みOCIデータを保持します。削除とは異なります。

最初の構成はリポジトリ1〜8個です。後で登録を変更しても既存の作業を上書きしません。
構成変更には明示的なforkを使います。[通常の開発環境](../design/default-development-session.ja.md)を参照してください。

## 詳細な選択と設定

既存の明示的なコマンドも使えます。環境名を指定するopen、`haco open --select`による
既存環境の選択、`haco open .`による所有者を固定した参照の再開を維持します。
新しいディレクトリには`--repo`が必要で、内容を暗黙に取り込みません。
[Workspace準備・fork](../design/workspace-workflow.md)と[CLI参照](cli.ja.md)から
Workspace・Env・Base・ストレージ・通信・設定を明示的に操作できます。
通常のopenでも`--base`・`--oci`を指定できますが、既存の互換性検証に従います。

`haco ssh setup <environment>`は接続の準備だけを行います。
`haco open --client none --json`は通常の環境を準備・再開して識別情報を返し、
デスクトップ接続は準備しません。editorプロセスの起動だけでは接続やbuild/testの成功を
意味しません。[Windows SSH](windows-environment-ssh.md)も参照してください。

## 対象・script利用・キャンセル

明示的なlifecycle操作には名前を指定します。open --selectとssh setupは端末の番号一覧から選択でき、空入力なら接続変更前にキャンセルします。Envが一つだけなら自動選択できます。scriptでは常に名前を指定してください。非対話で選択が曖昧な場合は入力待ちにしません。オプションは対象より前に置き、各commandの`--help`で書式を確認します。

結果はstdout、進捗・診断はstderrです。scriptには`haco env list --json`、`haco env status --json <environment>`を使います。createの既存JSON結果も維持します。保持データの削除は端末確認か明示的な`--yes`が必要で、pipe/FIFOで入力待ちにしません。Ctrl+Cで観測が終わってもcontrollerの変更処理が終わったとは限らないため、再実行前にstatusを確認します。端末終了をcleanup成功と解釈しません。一時実行は別契約で時間制限付きcleanupを要求し、その確認結果を返します。

## 失敗後の操作

| 表示 | 次の操作 |
|---|---|
| setupのrunning / succeeded / failed | 観測した工程です。進捗率ではありません。固定stage/reasonとrequest IDを確認します。 |
| busy | 別操作が対象を使用中です。その操作を確認・待機します。承認待ちとは異なります。 |
| Capabilityの承認待ち | 別のtrusted端末で`haco approve`により一覧・内容を確認します。観測だけでは承認しません。 |
| create/start/stop/delete/SSH失敗 | `haco env status <environment>`と`haco doctor <environment>`で確認するまでresource状態は不明です。cleanup済みと推測しません。 |
| SSH準備後のeditor起動失敗 | Envと接続は残ります。`haco open --client ssh <environment>`を使うかdesktop editorを直します。 |
| setup customization失敗 | 基盤工程が成功している場合もあり、保存scriptと副作用が残ります。内容を確認・修正してから明示的に再実行します。`--clear-script`は保存recipeを除去するだけで副作用を戻しません。 |

setupでは`haco doctor`を実行します。**WSL/Linux Physical Hostの管理者**は次のjournalでsetupの`request_id`を探せます。Envにはこの管理情報を渡しません。

```bash
journalctl -u haco-controller.service --since '30 minutes ago' --no-pager
```

backendの生出力や秘密は診断フィールドに含めません。streamが切れた場合は最終結果が未確認で、controllerのsetupが続いている可能性があります。[setup診断](../design/trusted-host.ja.md#setupの進捗と失敗診断)を参照してください。

## 必要なものだけ削除する

**trusted haco-host内**の`haco env delete <environment>`は、明示対象と消える・残るデータを表示して既存の正規削除APIを呼びます。Env runtime/rootfs/接続を除去し、Workspace、OCI Store、独立snapshotは保持します。明示Env名を削除意思として扱う既存契約を維持し、新たなpromptや互換性を壊すcommand改名は追加しません。失敗時には不在を確認できておらず、所有権・leaseはlifecycle APIの規則で保持します。

`haco workspace list`、`haco plugin oci store list`、`haco snapshot list`で保持データを確認します。別のWorkspace/Store deleteは消える内容を表示し、確認または`--yes`を要求します。Workspace削除では未commit・未追跡・未pushの作業も消え得ます。必要なデータは独立snapshotやexportに保持してください。今回の変更で保持単位は再設計しません。
