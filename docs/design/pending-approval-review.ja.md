# 承認待ちの確認

[English](pending-approval-review.md) | 日本語

状態: **VS Code GUIとWindows通知内の確認を開発候補で実装済み。導入済み新規回答の受け入れはpartialです。**
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

`haco approve --list` は承認せず、信頼された承認待ちの詳細を JSON で表示します。
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

## VS Code GUIでの承認

状態: **開発候補で実装済み。導入済みGUIの受け入れは確認待ち**。
任意のローカルUI拡張で通知のReview、または **Hacocoon: Review Pending Approvals** を選ぶと、
Webviewが開きます。今回の操作全体と保存範囲を確認し、**今回は許可する** または **今回は拒否する**
を選びます。terminal入力は不要です。表示はVS Codeの日英設定に従い、command paletteは通知bridgeなしでも使えます。

初期値は設定を保存しない「今回のみ」です。選択肢は共通Policy builderが返した範囲だけです。
Gitのref／更新種別、networkのhostname／protocol／portなど、保存するruleを省略せず表示します。
Env単位は今回の作成identityに限定し、全Env共通は今後作成するEnvも含みます。毎回確認を保存する場合も、
今回の許可・拒否は明示的に回答します。通知を開く、選ぶ、更新するだけでは回答しません。

固定した導入済みローカルCLIのprivate子プロセスpipeから既存management APIを使用します。
WindowsのWSL distributionはローカル利用者設定だけで選び、LinuxはPhysical Hostを使います。
Web／remote extension Host、非信頼window、非対応platformは拒否します。workspace設定・provider出力・
公開eventは実行ファイル・認証・管理socketを指定できません。通常Envへ管理権限を投影しません。

privateな選択tokenを表示内容全体に束縛します。回答直前にEnv作成identityと保存範囲を含めて再取得・比較し、
共通decision serviceを呼びます。単一回答の取得、Policy検証・保存・監査・実行は既存serviceが担当します。
tokenはargv・log・URL・読み取り専用event bridgeへ出しません。既存management endpointの認可も必要です。

ローカル画面は一つを再利用し、待機中は5秒ごとに承認待ちを更新します。回答の選択は失敗し得る呼出し前に
消費します。閉じる操作と15分の表示期限はローカル子プロセスを終了させ、自動再送や取消しはしません。
queueのより短い要求期限は維持します。取得・protocol・通信失敗では選択を無効にし、送信後の不明な結果は
未確認と表示します。拒否、実行成功、保存設定、監査未完了をreceiptで区別し、不確かな結果の再試行前には
設定と監査を確認します。

Webviewはnetwork／command URI／local file resourceを許可せず、nonce CSPとtextContentを使います。
権限に関する値は省略せず文字列として表示します。snapshot、待機数、入出力、stderrには上限を設けます。
private protocolは表示用内部interfaceで、公開plugin APIではありません。
[ADR 0067](../adr/0067-local-gui-approval-session.md)を参照してください。

実rendererのclick、保存範囲、古い／変更された要求、trust取消し、重複回答、不正出力、取消しを回帰試験します。
導入済み検証は実Webviewの起動通知と実コントローラーの古い要求拒否を観測し、回答を注入しません。
新しい実機結果は確認待ちです。旧custom terminalの成功はhistoricalであり、GUIの成功へ読み替えません。
[検証証拠](../status/acceptance-evidence.ja.md)を参照してください。Windows通知内だけでの新規回答は
Issue #568の別の残件です。

## Windows通知内で回答する

状態: **開発候補で実装済み。導入済みの新規要求への回答と、表示の見切れ確認は未完了**。

Windowsインストーラーは自分のWSLディストリビューション用に、非表示helper、照合用protocol、
通知識別子とCOM受信を登録します。所有するHostのsetupは既存controllerから通知を購読します。
`-SkipDesktopReview`は引き続き登録を省略し、所有する通知サービスを無効にします。

承認待ちはOS通知内で確認します。「次へ」「前へ」で今回のすべての条件を読み、設定を保存するか
選び、保存する正確な条件を確認してから今回の許可・拒否を回答します。既定は保存しない選択です。
このEnv限定は今回の作成分だけ、全Envは今後作成するものも含みます。「毎回確認」の保存時も今回の
回答を明示します。本文クリック・表示・閉じる・開き直し・URLでは回答しません。実装上は端末・
ブラウザー・別管理画面の起動を必要としません。

長い条件は省略せず次のページへ続けます。外部値は文字列で表示し、制御文字・双方向表示文字を
可視化します。選択欄には共通処理が返した保存候補だけを使います。表示に成功したページごとに
一度だけ使えるnative nonceを発行します。最後の選択は別の非公開子プロセスで再取得し、表示した
要求全体・保存候補と照合してから共通処理へ渡します。controllerのtoken・回答はargv、URL、
公開イベントへ載せません。CoreにWindows固有のPolicy・実行処理を追加せず、通常Envの権限も増やしません。

helperはユーザー・ディストリビューション単位の登録所有権と固定起動先を検証します。二重起動は
既存COM受信へ読み取り専用の表示を依頼し、実際のShow後に応答します。最大16件を扱い、3秒ごとに
待機状態を確認して一度に1件まで新しい通知を追加します。消えた要求のボタンを撤去し、Windows通知
自身にも2分の期限を設定します。helperと非公開接続は最大15分、回答の子プロセスはその範囲で最大5分です。
終了時は所有する子プロセスを停止・回収し、所有する確認通知を削除します。削除失敗でも古いnonceは無効です。

表示・protocol・登録・接続の失敗で許可しません。回答は失敗し得る呼び出しの前に消費し、自動再送
しません。拒否、監査付きの操作成功、確認できた設定保存、操作失敗、結果未確認を分けて表示します。
結果不明時は再試行前に現在の設定と監査を確認します。native診断は固定の段階名・状態値に限定し、
生の子プロセス出力を露出しません。

[ADR 0071](../adr/0071-notification-contained-approval.ja.md)を参照してください。Windows実COM受信と
英日選択XMLの通知履歴には構成要素の検証がありますが、見切れ・人のクリック・導入済み新規回答の
証拠ではありません。以前のconsole/URI受け入れ、過去のchecksum失敗、実機残件は
[検証証拠](../status/acceptance-evidence.ja.md)に保持します。Linux native起動は計画のままです。
