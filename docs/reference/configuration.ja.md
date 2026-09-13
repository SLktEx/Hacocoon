# 承認方針の設定

状態: **実装済み。導入済み環境の設定往復は 2584ec6 で成功しました。**

信頼された Linux／WSL Host で実行します。

```bash
haco config
haco config --edit
```

最初のコマンドは現在の revision と Policy を人向けの表示で示します。
機械処理用の snapshot は `haco config --json` で取得します。
`--edit` は同じ文書を `VISUAL`、`EDITOR`、未設定なら `vi` で開きます。
`policy` を編集し、`revision` は変えません。通常の承認や Git 操作に
新しい必須引数はありません。

ファイルで編集する場合:

```bash
haco config --json > configuration.json
# configuration.json の policy を編集し、revision は保持する。
haco config --file configuration.json
```

保存結果もスクリプトで解析する場合は、適用コマンドにも `--json` を付けます。

`policy.rules` は管理者ルール、`policy.saved_decisions` は通常の承認から保存した
方針です。同じ照合に参加し、deny、require-approval、allow の順に優先します。
保存方針を消しても管理者の制約は消えません。env 単位の保存は作成 ID に結び付き、
全 env の方針は明示的に `environment: "*"` を使います。
Git はリポジトリ・remote・ref・fast-forward を固定します。
[Policy の意味](../design/policy-and-capability-foundation.md)と
[Git の手順](../guides/git-workflow.ja.md)を参照してください。

コントローラーを再起動せず次の要求へ反映します。既存接続は取り消しません。
実行中の操作には、通常の実行直前の Policy 再確認が適用されます。

別の承認保存や設定編集があると古いスナップショットは競合になります。最新の設定と差分を
確認し、意図した編集をやり直します。自動統合・再試行は行いません。
エディター の失敗時は編集ファイルを残して場所を表示します。
成功応答は永続化と完了監査の成功を意味します。不明確なエラーの後は、保存済みの可能性が
あるため現在の設定を確認してから再試行します。

Physical Host の保護された `/var/lib/hacocoon/policy.json` が唯一の設定元です。
直接編集は `.policy-save.lock` と協調するかコントローラーを停止して行います。
通常コマンドは自動で協調します。既に壊れた Policy や安全でないファイルは拒否し、
管理者による修復が必要です。読めないルールを初期値に置き換えません。
Policy の詳細は通知内容 に載せません。[ADR 0027](../adr/0027-revision-bound-policy-editing.ja.md)
を参照してください。

リポジトリ内で競合、承認保存との同時更新、危険なファイル、不正入力、保存前後の監査失敗、
次要求への反映を確認します。製品 CLI／コントローラー E2E は確認・エディター・ファイル反映・
競合拒否を扱います。導入済み Windows／WSL の受け入れを示すものではありません。


## 検証範囲

導入済み環境の設定往復は `2584ec6` と `71dbb4f` で成功しました。空配列の表示が一致しなかった失敗と、後の成功だけでは原因が分からないプレビュー・診断の失敗は[検証証拠](../status/acceptance-evidence.ja.md#development)に残します。リポジトリ内の検証と実機結果は区別します。

規則には任意のRFC 3339形式の`expires_at`を指定できます。期限以降はその規則を
適用せず、残る規則と既定ポリシーで判定します。不正な値は読込みを拒否します。
[規則の期限](../design/policy-and-capability-foundation.md#rule-lifetime)と
[接続中の失効](../design/network-connections.md)を参照してください。
