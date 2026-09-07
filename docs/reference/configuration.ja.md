# 承認方針の設定

状態: **repository の一段階を実装済み。installed 受け入れは未確認です。**

trusted Linux／WSL Host で実行します。

```bash
haco config
haco config --edit
```

最初の command は現在の revision と Policy を JSON で表示します。
`--edit` は同じ文書を `VISUAL`、`EDITOR`、未設定なら `vi` で開きます。
`policy` を編集し、`revision` は変えません。通常の承認や Git 操作に
新しい必須引数はありません。

ファイルで編集する場合:

```bash
haco config > configuration.json
# configuration.json の policy を編集し、revision は保持する。
haco config --file configuration.json
```

`policy.rules` は管理者 rule、`policy.saved_decisions` は通常の承認から保存した
方針です。同じ照合に参加し、deny、require-approval、allow の順に優先します。
保存方針を消しても管理者の制約は消えません。env 単位の保存は作成 ID に結び付き、
全 env の方針は明示的に `environment: "*"` を使います。
Git は repository・remote・ref・fast-forward を固定します。
[Policy の意味](../design/policy-and-capability-foundation.md)と
[Git の手順](managed-repository-workflow.md)を参照してください。

controller を再起動せず次の要求へ反映します。既存接続は取り消しません。
実行中の操作には、通常の実行直前の Policy 再確認が適用されます。

別の承認保存や設定編集があると古い snapshot は競合になります。最新の設定と差分を
確認し、意図した編集をやり直します。自動 merge・再試行は行いません。
editor の失敗時は編集ファイルを残して場所を表示します。
成功応答は永続化と完了監査の成功を意味します。不明確なエラーの後は、保存済みの可能性が
あるため現在の設定を確認してから再試行します。

Physical Host の保護された `/var/lib/hacocoon/policy.json` が唯一の設定元です。
直接編集は `.policy-save.lock` と協調するか controller を停止して行います。
通常 command は自動で協調します。既に壊れた Policy や安全でないファイルは拒否し、
管理者による修復が必要です。読めない rule を初期値に置き換えません。
Policy の詳細は通知 payload に載せません。[ADR 0027](../adr/0027-revision-bound-policy-editing.ja.md)
を参照してください。

repository 内で競合、承認保存との同時更新、危険なファイル、不正入力、保存前後の監査失敗、
次要求への反映を確認します。製品 CLI／controller E2E は確認・editor・ファイル反映・
競合拒否を扱います。installed Windows／WSL の受け入れを示すものではありません。

## ローカル installed での観測

通常インストーラで導入した `71dbb4fc4f3e` snapshot の Host doctor は全 6 項目が成功しました。
通常の trusted Host から `haco config` で取得・反映し、返された revision を Physical Host の
ファイルと変更前後の監査に照合しました。default deny・管理者 8 ルール・保存方針 0 件は維持されています。
Windows package の SHA-256 は
`d658fe9947146a23b168f9de333443302576b339aefe8a30d58ed4562931dae5` です。

最初の JSON 表示の一致確認は **FAIL** でした。明示的な空の saved_decisions 配列が
保存時に省略されたためです。初回の snapshot 表示から同じ形式へ正規化する修正を行いました。
revision は引き続き実ファイルの正確な bytes を hash 化します。
同じ条件の component 回帰テストと CLI E2E は成功しました。表示修正の installed 受け入れは未確認です。

`71dbb4f` は GHA 全 4 系統が成功しました。test 34151576434、Ubuntu 34151576429、
Incus 34151576447、Windows 34151576493 です。Windows は通常 config の往復、
実際の VS Code・project setup・Edge preview・Environment doctor を含みます。

別のローカル `71dbb4f` 検証では、`haco config --file` で `preview-71dbb4f` だけに
Ubuntu archive の一時ルール 4 件を追加・削除しました。通常の project setup で
loopback HTTP server を起動し、Windows が port 36059 で正確な Workspace marker を取得しました。
preview の再利用・閉鎖後の拒否と、runtime／Workspace／DNS の doctor が成功しました。
このローカル probe には SSH 接続を用意していません。marker・recipe・Environment・
listener を削除し、default deny・元の 8 ルール・保存方針 0 件を確認しました。
既存 Workspace `git-save-eb16300` は保持しています。過去の preview／doctor 失敗は
この実行では再現せず、元の原因は未解明のままです。
