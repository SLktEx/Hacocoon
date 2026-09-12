# 管理対象Gitの操作

[English](git-workflow.md) | 日本語

[はじめての開発環境](getting-started.ja.md)を終えた方向けに、Gitの権限設定、
複数リポジトリ、承認の保存を説明します。認証は信頼された `haco-host` に残し、
EnvironmentにはGit専用の仲介接続だけを渡します。

<a id="configure-git-policy"></a>
## Gitの権限設定

信頼された管理側で `haco config --edit` を開き、次の**ルール項目**を
`policy.rules` に追加します。revision、既存ルール、保存済みの選択、既定値を保持してください。
設定全体を置き換えるJSONではありません。
登録したURL・リポジトリID・Environment・既存ブランチに置き換えます。
公開リポジトリの例は読み取りに使えますが、GitHubのpush権限を与えるものではありません。

```json
[
  {
    "capability": "git.repository", "action": "fetch",
    "environment": "sample-dev", "resource": "https://github.com/SLktEx/Hacocoon.git",
    "attributes": {
      "repository": "sample", "remote": "https://github.com/SLktEx/Hacocoon.git",
      "target_ref": "refs/heads/*", "old_oid": "*", "new_oid": "*", "operation_id": "*"
    },
    "decision": "allow", "reason": "Read all branches of this registered repository"
  },
  {
    "capability": "git.repository", "action": "push",
    "environment": "sample-dev", "resource": "https://github.com/SLktEx/Hacocoon.git",
    "attributes": {
      "repository": "sample", "remote": "https://github.com/SLktEx/Hacocoon.git",
      "target_ref": "refs/heads/main", "old_oid": "*", "new_oid": "*", "operation_id": "*",
      "update_kind": "fast-forward"
    },
    "decision": "require-approval", "reason": "Review the exact push"
  }
]
```

要求の全属性がルールに必要です。この例ではリポジトリを固定して全ブランチの読み取りを許可し、
pushはmainだけを毎回確認します。以前のmain限定の読み取りルールは全ブランチfetchを許可しません。
範囲を確認して明示的に更新してください。cloneではpush許可を保存しません。
個別refの読み取り拒否がある場合は一覧全体を拒否し、オブジェクト取得時にも各refのPolicyを再確認します。
コミットID・操作IDは変動可能です。pushの`update_kind`は更新なら`fast-forward`、新規なら`create`で、
この項目のない古いルールは拒否されます。開発ブランチを確認付きでpushするには、その正確な
`target_ref`と対象update kindに`decision: require-approval`のpushルールを追加し、mainのルールは
維持してください。既定denyなら対応ルールのないブランチ／種別は拒否、既定require-approvalなら
共通reviewで確認されます。
[Policyの優先順位](../design/policy-and-capability-foundation.md#matching-rule-precedence)は
記載順によらず、拒否、承認要求、許可の順です。

信頼された側が上流Gitに接続するため、EnvironmentにGitHub認証情報や認証付きプロキシは不要です。
パッケージ取得やアプリのDNSには別途ネットワーク権限が必要です。

## 取得・コミット・push

Environmentでは通常の `git status`、`git fetch origin`、`git pull --ff-only`、
`git add`、`git commit`、`git push` を使います。
この開発候補では1回のbatchで最大1024個のSHA-1ブランチ、合計32 MiBまでのpackを取得できます。
refごとの転送で共有履歴が重複する場合があり、大容量転送の最適化は未完了です。
`git branch -r`で一覧を見て、たとえば`git switch --track origin/feature/example`で
既存ブランチへ切り替えます。開発候補のpushは新規ブランチ一つ、または既存ブランチ一つの
fast-forwardに対応します。たとえば`git switch -c feature/work`で作成・commit後に
`git push -u origin feature/work`を使います。新規作成も固定commitを示して個別に承認し、後続更新とは
別に判断します。force push、ブランチ削除、複数refのpush、LFS、submoduleは延期されています。
移動・削除されたheadは拒否するため、再fetchで最新状態を確認してください。巨大レポの受入を意味しません。

新規Workspaceは全ブランチのfetch設定を持ちます。既存の独立Workspaceを全ブランチへ広げる場合は、
読み取りPolicyを確認後、**そのEnvironment内で**次を実行します。変更は手元のGit設定だけです。

```bash
git config --replace-all remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'
git fetch origin
```

pushの待機中、別の信頼されたHostターミナルで確認します。

```bash
haco git pending
haco git approve <id>
# 同じ要求を拒否する場合:
haco git deny <id>
```

実行する判断は一つです。リポジトリ、URL・ref、Environmentと作成時の識別子、
変更前後のコミット、差分の概要を確認してください。
承認は固定された提案に適用され、後で移動したブランチには適用されません。
拒否すると上流は変わらず、再度pushすると新しい要求になります。

タイムアウトや通信・監査の失敗時は、再試行前に認証済みの信頼された側から上流refを確認します。
外部への書き込みは完了している可能性があります。応答がないことは未実行の証拠になりません。

## 確認した選択を保存する

```bash
haco git approve --save env <id>
haco git approve --save all <id>
haco git deny --save env <id>
haco git approve --save ask-env <id>
```

各行は選択肢です。`env` は今回作成したEnvironmentに限り、
`all` は他のEnvironmentや将来作成するものも明示的に対象にします。
`deny --save all` は全体への拒否を保存します。
`ask-env` と `ask-all` は今後の承認要求を保存し、今回の判断はapprove／denyで別に指定します。
`--save` を省略するとPolicyを変更しません。

保存した選択は対象ブランチを固定し、新規作成とfast-forward更新を区別します。
`feature/work`の作成を許可しても、その次の更新やmainへのpushは許可されません。
承認待ち中に同名refが作られた場合も、新規作成で上書きしません。競合時は上流を確認・fetchし、
新しく判断してください。承認の自動再試行は行いません。

保存した許可は管理者の拒否・承認要求を上書きしません。
上の例は毎回確認する設定です。繰り返し許可する必要があれば、管理者ルール自体を確認します。
取り消すときは[`haco config`](../reference/configuration.ja.md)で対応する保存項目を削除します。
保存結果の成功には永続化と監査が必要ですが、その後の実行が失敗しても設定が残る場合があります。

## 複数リポジトリ

接続先とブランチを個別に登録して、一つの集合を作ります。

```bash
haco repo clone --branch first-branch first https://github.com/OWNER/REPO.git
haco repo clone --branch second-branch second https://github.com/OWNER/REPO.git
haco workspace create --repo first,second both
haco env create --workspace managed:both both-dev
haco git connect both-dev
```

`/workspace/first` と `/workspace/second` で作業し、それぞれ独立した `.git` を使います。
両方の取得元にPolicyを設定します。一つの利用権の予約が集合全体を所有し、
メンバーを別々に貸し出すことはできません。各マウントの外に書いたデータはEnvironmentだけに属します。
メンバー構成の変更と、中断した集合の復旧は延期されています。

## 保持するデータとインポート

停止では作業と利用権の予約が残ります。Environmentを削除してもWorkspaceは残りますが、
Workspace自体の削除ではGit情報も失われます。
取得元・Workspace・Storeの整理は[データの寿命](data-lifetime.ja.md)を参照してください。
Workspaceの記録が接続先として使っている取得元は削除できません。

インポート済みのGitHub接続先は、同じID・URL・ブランチを明示的に登録した取得元だけに再接続できます。
ファイル接続先と旧形式のインポートはオフラインのままで、同名の取得元を作っても有効になりません。
[インポート後のGit再接続](../design/git-and-github-capability.md#reconnect-an-imported-github-workspace)を参照してください。
インポートした実機でのfetch／pushは未確認です。

権限境界は[Git設計](../design/git-and-github-capability.md)、
コミットごとの成功・失敗・保持した検証データは[受入記録](../status/acceptance-evidence.ja.md)にあります。

パスからの再開と独立したデータ分岐は[Workspace workflow](../design/workspace-workflow.md)を参照してください。
