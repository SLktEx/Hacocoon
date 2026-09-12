# ADR 0028: 承認待ち session

[English](0028-pending-approval-sessions.md) | 日本語

状態: accepted。リポジトリ実装済み、導入済み受け入れは未確認です。

background の通信要求にも、daemon stdin や暗黙の許可を使わず人の確認が必要です。
置き換え可能な Standard queue が待機を制限します。Core の capability application
が Policy 評価後の受付、回答、永続化、監査、最新 Policy／識別確認、実行を担います。

受け付けたセッションの object 識別で待機と完了を結び付けます。capability サービス
の実際の戻り値が決まった後だけ完了を渡します。重複して拒否した要求にはセッションを
渡さず、古い完了で置き換わったセッションを解放しません。上限付きの channel により、
reviewer が切断しても処理を終えられます。再起動後に要求を再実行しません。
回答送信後のキャンセルは結果が不明な場合であり、取り消しではありません。

共通確認 application は小さな interface で独立した元データをまとめます。Git は
準備済みの厳密な操作と既存の承認所有権を保ち、範囲を再構築せず元の承認要求を
渡します。元データ間の重複 ID は拒否します。Core にプロバイダーごとの条件分岐や
新しい Environment／Workspace ライフサイクル mutation を追加しません。

確認操作を受け付けるのは既存の認可された管理ソケットだけです。その principal は
既に Hacocoon を管理できますが、Environment と読み取り専用 event bridge にはこの
権限を渡しません。通知の要求 ID は操作の識別子であり bearer token ではありません。
ブラウザー認証情報、localhost の無認証 POST、guest の管理ソケット、通知だけによる
自動決定は追加しません。

不採用: 無制限の待機、background での stdin 読み取り、queue 受付を保存成功と扱う設計、
ID だけの完了所有権、再起動後の未完了承認の復元、UI の値からの Git 権限再構築、
生のプロバイダー出力を確認に渡す設計、公開の表示用ストリームを通じた承認。
[機能の契約](../design/pending-approval-review.ja.md)を参照してください。
