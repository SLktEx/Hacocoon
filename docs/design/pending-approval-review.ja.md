# 承認待ちの確認

[English](pending-approval-review.md) | 日本語

状態: **リポジトリの一段階を実装済み。VS Code 確認は実装済み、Windows 通知の起動導線は実装済み、D2 受け入れは部分実装です。**
リポジトリのテストと、導入済みのネットワーク・デスクトップ・GitHub 受け入れは区別します。

## 通常の使い方

信頼された Host で実行します。

```sh
haco approve
```

候補が一つなら直接詳細を表示し、複数なら番号で選択します。現在の capability、
action、対象、Environment 作成識別、権限に関わる属性と、再利用する範囲を
分けて表示します。今回だけの yes/no、または Environment／全体の allow・deny・ask
を選べます。ask の保存でも、今回の操作には別途 yes/no が必要です。

`haco approve --list` は承認せず、信頼された承認待ちの詳細を人向けに表示します。
スクリプトで一覧を解析する場合は `--json` を付けます。
任意の要求 ID で対象を指定することもできます。通常の Git・ネットワーク操作には
新しい引数は不要です。既存の Git approve/deny も使えます。保存した Policy の
確認・編集には [`haco config`](../reference/configuration.ja.md) を使います。

通常は Approved／Denied と保存した Policy を短い文で表示します。スクリプト用に
capability 処理記録が必要な場合だけ --json を指定します。実行状態・保存した選択・
監査の完了を区別します。拒否した要求は未実行で、プロバイダー成功の完了監査もありません。
通信断や失敗時は、再試行前に Policy と監査を確認してください。回答を送れただけで
保存や実行の成功とは扱いません。

network.egress の succeeded は通信の認可完了を意味し、相手の HTTP 操作の成功ではありません。接続・TLS・application の失敗は元の application が報告します。導入済み受け入れでは認可の処理記録と実際の HTTPS 応答を別々に確認します。

## Git と network に共通の判断

Git proposal と HTTPS/network 要求は、同じ `haco approve` の選択肢、読みやすい結果表示、
保存 Policy、監査、実行前の再評価を使います。

| 選択 | 今回の要求 | 以後の一致する要求 |
|---|---|---|
| 今回だけ許可／拒否 | 明示した回答 | 保存変更なし |
| Environment の許可／拒否／毎回確認 | 保存判断を適用。毎回確認は別途回答が必要 | 同じ Environment 作成 ID のみ |
| 全体の許可／拒否／毎回確認 | 保存判断を適用。毎回確認は別途回答が必要 | 将来作成するものも含め全 Environment |

同名で作り直した Environment に、以前の Environment 限定設定は引き継ぎません。
全体設定は意図どおり将来の Environment にも適用します。保存によって対象を広げず、
Git はリポジトリ・remote・ref・update kind、ネットワークは正規化した hostname・プロトコル・ポートを
保持します。変化する operation／old／new commit ID の一般化は、信頼済み Git プロバイダーが
検証した再利用範囲だけで行います。明示した拒否と必須の隔離は引き続き優先します。

HTTPS CONNECT の承認単位は接続であり、その内部の HTTP 要求一件ごとではありません。
暗号化された URL パス・method はこの境界では観測・強制できず、保存条件として表示しません。
実際の TLS／HTTP 成否は元のクライアントが報告します。domain 単位の承認を URL 単位とは扱いません。

共通の回帰テストは、両 capability の CLI 回答 10 通りと保存設定 6 通りについて、
監査範囲・再評価・対象変更・同名再作成を比較します。これは共通境界の検証であり、
Git push や実際のネットワーク接続を実行するテストではありません。

## 待機と権限

置き換え可能な Standard queue は、Policy が承認を要求したコントローラーの background
要求を待機させます。上限は実行中を含め 128 セッション、Environment 名ごとに 16、
要求の表示用スナップショットは 16 KiB です。選択期限は二分で、元の要求のキャンセルも
尊重します。選択済みセッションも実際の結果が確定するまで件数に含めます。
超過・期限切れ・選択前のキャンセルでは許可しません。コントローラーは暗黙に継承する stdin
を読みません。対話型 control セッションは従来どおり専用 callback を使います。

待機と完了は、受け付けた個別セッションが所有します。回答を受け付けるのは一度だけです。
古い・重複した完了通知では、置き換わったセッションを解放できません。結果は buffered
channel で渡し、reviewer の切断で解放を止めません。回答受付後のキャンセルは回答の
取り消しや再承認を意味しません。再起動時は待機を失い、操作を復元・再実行しません。

共通確認 application は queue と既存の Git proposal をまとめます。元の信頼された
要求を使い、要求 ID が曖昧なら拒否します。要約から Git 権限を再構築せず、
プロバイダーの実行も所有しません。capability サービスが保存範囲の確認、永続化、監査、
現在の Policy と Environment 識別の再確認、準備済み操作の実行を引き続き担います。

`approval.pending`／`approval.decide` は既存の管理 Unix ソケットだけに登録します。
Physical Host では root/hacocoon group、信頼された Host への projection は root 専用です。
Environment には管理接続先を渡しません。回答は ID、明示的な boolean、任意の
保存選択だけで、対象・属性・保存範囲を差し替えられません。失敗時も実行結果は返しますが、
プロバイダー出力は除外し、error は固定の分類だけを返します。

## 通知

[Interaction ストリーム](../reference/interaction-events.ja.md) は読み取り専用の最小化された表示経路です。
ID は照合用で、承認 token ではありません。VS Code は下記のローカル確認を使用し、
native Windows 通知の起動導線は実装済み、D2 受け入れは部分実装です。event bridge へ回答を送ったり、
信頼しない remote terminal で管理コマンドを動かしたりしません。
[ADR 0028](../adr/0028-pending-approval-sessions.ja.md) を参照してください。

## VS Code からのローカル承認

状態: **リポジトリ実装済み、導入済み受け入れは未確認**。
任意のデスクトップ VS Code 拡張で通知の Review、または Hacocoon: Review Pending Approvals を選ぶと、
ローカル UI extension Host が通常の `haco approve` を専用 terminal で開きます。
クリック自体では回答しません。信頼された現在の要求と再利用範囲を確認し、単発回答または保存を選び、
ask には別途今回の yes/no を入力します。

Windows はローカルの導入済み Hacocoon WSL distribution と既定の利用者を使用し、
Linux はローカル Physical Host の導入済み CLI を使用します。実行ファイルは絶対パス固定、
引数は分離し、環境変数は許可リストに限定します。workspace の実行設定やコントローラー override、
remote シェルは使いません。別の WSL distribution はローカル利用者設定だけで指定できます。
Web、remote extension Host、信頼されていない window、非対応 platform は承認を開けません。

同じ要求の画面は再利用し、入力と出力を制限します。子プロセスの制御文字は terminal を操作できません。
閉じる操作・Ctrl-C/D・15 分の期限はローカルの子プロセスを終了させますが、送信済み回答の取消しや
再実行はしません。失敗・不明な結果は未確認として示します。Windows の起動導線は後述し、Linux デスクトップの起動は未実装の計画です。
[ADR 0029](../adr/0029-local-desktop-approval-review.ja.md) を参照してください。

JavaScript テストは実行先、入力、終了処理、失敗、通知クリックを確認します。
実 VS Code GHA には予測不能な古い要求 ID で local terminal から導入済みコントローラーの拒否を確認する
検査を追加しましたが、結果は未確認です。新しい要求への人間の実回答や OS 通知クリックを証明するものではありません。

実機検証では VS Code API 全体の列挙が確認起動前に失敗したため、observer は必要な安定 API だけを明示的に渡します。失敗時も作成確認済みの editor/terminal 検証ファイルを削除し、生の subprocess 出力を含まない固定段階と真偽値だけを記録します。実機で通常 CLI の HTTPS 承認は別途成功し、修正 observer は実機 VS Code 1.136.1 で編集・terminal・古い要求拒否を確認し、検証用構成後始末も成功しました（導入済み 6771f2f、observer 05c8206）。

## Windows 通知から開く

Windows インストーラーは対象 WSL 専用の native 確認アダプターを登録し、信頼済み
`haco-host` の所有する通知サービスを有効にします。`-SkipDesktopReview` は登録を省略し、
サービスを無効にします。通常の Host setup が通知バイナリを配置し、コントローラー経由で購読します。
監査ファイルは投影しません。通知からその要求の既存 `haco approve` console を開き、
範囲を確認して通常の回答を入力します。開くだけで回答・Policy 保存・再実行はしません。
Windows のインストール済み自動起動の受け入れは未完了です。
[通知の配送](../reference/interaction-events.ja.md)を参照してください。

distribution ごとにユーザー単位のプロトコルと通知識別を分けるため、検証 instance
の導入で別 instance の通知先を変えません。helper は正規の要求 URI だけを受け取り、
実行ファイルと設定済み distribution を固定し、環境の上書きを除去します。
不正リンク・余分な引数・古い要求は拒否します。登録がない場合は承認起動のない通知です。
Linux デスクトップの起動は未実装の計画で、VS Code は引き続き任意です。

実機 Windows では、正しいプロトコル URI を持つ通知履歴、対応 helper の Windows
プロトコル起動、導入済みコントローラーの古い要求拒否を確認しました。
終了済みの専用 HTTPS 検証要求を使った結果であり、OS 通知からの新規回答と人間による
画面上の toast click は未確認です。[ADR 0030](../adr/0030-windows-notification-review.ja.md)を参照してください。

`4bb8dad` の Windows run 34176272125 は、native アダプターの checksum 検証が
使用できない `Get-FileHash` に依存していたため、デスクトップ受け入れ前に失敗しました。
.NET で直接 hash を計算するよう修正し、`Get-FileHash` を使えない条件の PowerShell 5.1
構成要素回帰は成功しました。修正後の実 Windows インストールと通知サービス自動起動は
未完了です。失敗 run の後続検査は SKIP であり、成功ではありません。
