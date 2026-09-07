# 承認待ちの確認

[English](pending-approval-review.md) | 日本語

状態: **repository の一段階を実装済み。VS Code review は実装済み、OS 通知からの起動は planned です。**
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

[Interaction stream](../INTERACTION_EVENTS.ja.md) は read-only の最小化された表示経路です。
ID は照合用で、承認 token ではありません。VS Code は下記のローカル review を使用し、
native OS 通知からの起動は planned です。event bridge へ回答を送ったり、
信頼しない remote terminal で管理コマンドを動かしたりしません。
[ADR 0028](../adr/0028-pending-approval-sessions.ja.md) を参照してください。

## VS Code からのローカル承認

状態: **repository 実装済み、installed 受け入れは未確認**。
任意の desktop VS Code 拡張で通知の Review、または Hacocoon: Review Pending Approvals を選ぶと、
ローカル UI extension host が通常の `haco approve` を専用 terminal で開きます。
クリック自体では回答しません。信頼された現在の要求と再利用範囲を確認し、単発回答または保存を選び、
ask には別途今回の yes/no を入力します。

Windows はローカルの installed Hacocoon WSL distribution と既定の利用者を使用し、
Linux はローカル Physical Host の installed CLI を使用します。実行ファイルは絶対パス固定、
引数は分離し、環境変数は許可リストに限定します。workspace の実行設定や controller override、
remote shell は使いません。別の WSL distribution はローカル user 設定だけで指定できます。
Web、remote extension host、信頼されていない window、非対応 platform は承認を開けません。

同じ要求の画面は再利用し、入力と出力を制限します。子プロセスの制御文字は terminal を操作できません。
閉じる操作・Ctrl-C/D・15 分の期限はローカルの子プロセスを終了させますが、送信済み回答の取消しや
再実行はしません。失敗・不明な結果は未確認として示します。OS 通知からの起動は planned のままです。
[ADR 0029](../adr/0029-local-desktop-approval-review.ja.md) を参照してください。

JavaScript テストは実行先、入力、終了処理、失敗、通知クリックを確認します。
実 VS Code GHA には予測不能な古い要求 ID で local terminal から installed controller の拒否を確認する
probe を追加しましたが、結果は未確認です。新しい要求への人間の実回答や OS 通知クリックを証明するものではありません。

実機検証では VS Code API 全体の列挙が review 起動前に失敗したため、observer は必要な安定 API だけを明示的に渡します。失敗時も作成確認済みの editor/terminal 検証ファイルを削除し、生の subprocess 出力を含まない固定段階と真偽値だけを記録します。実機で通常 CLI の HTTPS 承認は別途成功し、修正した desktop observer は再検証中です。
