# ADR 0071: Windows通知内での承認

状態: 開発候補で採用。新規要求への導入済み回答と表示の実機受け入れは未完了。

日付: 2026-09-13

## 背景

Issue #568はWindows通知内だけでの回答を要求します。[ADR 0030](0030-windows-notification-review.ja.md)
のコンソール起動では要件を満たしません。公開の要求IDとプロトコルURLは照合用であり、
回答権限ではありません。非パッケージ型クライアントの通知選択欄にはCOM受信を使います。

## 決定

コンソールhandlerを非表示のWindows helperへ置き換えます。インストーラーはWSLの
ディストリビューションごとに、ユーザー単位の通知識別子・COM class・固定起動先を所有します。
置換前に既存所有者を検証し、COM起動先の公開前に所有権を記録します。通常Envの権限は増やしません。

[非公開の承認セッション](0067-local-gui-approval-session.md)、既存の管理接続、共通の保存規則
builderを再利用します。一つのメモリー上の表示処理で最大16件を扱い、回答ごとに別の
非公開子プロセスを使います。Policy判断・公開回答API・provider処理を複製しません。

今回の条件と実際に保存する規則を、上限付きの通知ページですべて確認してから回答します。
既定は設定を保存しない選択です。同じEnvの今回の作成分と、今後の作成分を含む全Envを
区別します。「毎回確認」を保存する場合も、今回の許可・拒否を別に明示します。

表示ページごとに新しい乱数nonceを発行し、一度しか受理しません。選択はネイティブCOMだけで
受け取り、本文クリック・閉じる・URL・起動引数では回答しません。送信直前に別の非公開
セッションで選び直し、要求全体と保存候補が一致するか照合します。共通処理もsnapshotを再確認し、
要求の取得・Policy保存・監査・実行を引き続き所有します。

セッションtokenと回答は匿名pipe内に限定します。通知XMLは固定の表示scriptへstdinで渡し、
argvへ載せません。外部値はXMLの文字列とし、制御文字・双方向表示文字を可視化します。
二重起動への応答は実際のShow後に返します。通知が利用者や管理設定で無効なら配送を失敗とし、
設定の変更や許可への置換はしません。

古い通知を期限切れ・削除し、再起動時はすべてのページnonceを無効化します。プロセス・メッセージ・
期限に上限を置き、終了時は所有する子プロセスを停止・回収します。不明な回答の再接続・再送はせず、
確認できた設定保存と操作の失敗・結果未確認を区別します。削除失敗はエラーとして残します。
Windows上に古い通知が残っても、nonceの無効化は取り消しません。

## 不採用案と検証

端末・ブラウザー・別管理画面への遷移を要件達成とは扱いません。URLや公開イベント内の回答、
回答tokenの復元、自動再送、権限条件の省略、通知選択文字列からのPolicy再構築も採用しません。

実WindowsでのCOM ABIと通知履歴は構成要素の証拠です。表示の見切れや導入済みの新規要求への
人の回答とは区別し、[検証証拠](../status/acceptance-evidence.ja.md)に未確認を残します。

ネイティブ契約はMicrosoftの
[受信インターフェース](https://learn.microsoft.com/en-us/windows/win32/api/notificationactivationcallback/nf-notificationactivationcallback-inotificationactivationcallback-activate)と
[非パッケージ型の登録実装](https://github.com/microsoft/WindowsAppSDK/blob/main/dev/AppNotifications/AppNotificationUtility.cpp)に従います。
