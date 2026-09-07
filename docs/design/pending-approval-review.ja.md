# 承認待ちの確認

[English](pending-approval-review.md) | 日本語

状態: **repository の一段階を実装済み。通知から開く操作は planned です。**
repository のテストと、installed の network・desktop・GitHub 受け入れは区別します。

## 通常の使い方

信頼された Host で実行します。

```sh
haco approve
```

候補が一つなら直接詳細を表示し、複数なら番号で選択します。現在の capability、
action、対象、Environment 作成 identity、権限に関わる属性と、再利用する範囲を
分けて表示します。今回だけの yes/no、または Environment／全体の allow・deny・ask
を選べます。ask の保存でも、今回の操作には別途 yes/no が必要です。

`haco approve --list` は承認せず、信頼された承認待ちの詳細を JSON で表示します。
任意の request ID で対象を指定することもできます。通常の Git・network 操作には
新しい引数は不要です。既存の Git approve/deny も使えます。保存した Policy の
確認・編集には [haco config](../reference/configuration.ja.md) を使います。

通常は Approved／Denied と保存した Policy を短い文で表示します。スクリプト用に
capability receipt が必要な場合だけ --json を指定します。実行状態・保存した選択・
監査の完了を区別します。拒否した要求は未実行で、provider 成功の完了監査もありません。
通信断や失敗時は、再試行前に Policy と監査を確認してください。回答を送れただけで
保存や実行の成功とは扱いません。

network.egress の succeeded は通信の認可完了を意味し、相手の HTTP 操作の成功ではありません。接続・TLS・application の失敗は元の application が報告します。installed 受け入れでは認可の receipt と実際の HTTPS 応答を別々に確認します。

## 待機と権限

置き換え可能な Standard queue は、Policy が承認を要求した controller の background
要求を待機させます。上限は実行中を含め 128 session、Environment 名ごとに 16、
要求の表示用 snapshot は 16 KiB です。選択期限は二分で、元の要求のキャンセルも
尊重します。選択済み session も実際の結果が確定するまで件数に含めます。
超過・期限切れ・選択前のキャンセルでは許可しません。controller は ambient stdin
を読みません。対話型 control session は従来どおり専用 callback を使います。

待機と完了は、受け付けた個別 session が所有します。回答を受け付けるのは一度だけです。
古い・重複した完了通知では、置き換わった session を解放できません。結果は buffered
channel で渡し、reviewer の切断で解放を止めません。回答受付後のキャンセルは回答の
取り消しや再承認を意味しません。再起動時は待機を失い、操作を復元・再実行しません。

共通 review application は queue と既存の Git proposal をまとめます。元の信頼された
要求を使い、request ID が曖昧なら拒否します。summary から Git 権限を再構築せず、
provider の実行も所有しません。capability service が保存範囲の確認、永続化、監査、
現在の Policy と Environment identity の再確認、準備済み操作の実行を引き続き担います。

`approval.pending`／`approval.decide` は既存の管理 Unix socket だけに登録します。
Physical Host では root/hacocoon group、trusted Host への projection は root 専用です。
Environment には管理 endpoint を渡しません。回答は ID、明示的な boolean、任意の
保存選択だけで、対象・属性・保存範囲を差し替えられません。失敗時も実行結果は返しますが、
provider 出力は除外し、error は固定の分類だけを返します。

## 通知

[Interaction Event](../INTERACTION_EVENTS.ja.md) は read-only・最小限のままです。
ID は照合用で、承認 token ではありません。この段階では native／VS Code の通知から
review を開く操作は未実装です。将来の client も信頼された review 経路で新たに人の
回答を求める必要があります。event bridge 経由で承認したり、信頼されない remote
terminal で Host の承認コマンドを実行したりしてはいけません。

[ADR 0028](../adr/0028-pending-approval-sessions.ja.md) を参照してください。
