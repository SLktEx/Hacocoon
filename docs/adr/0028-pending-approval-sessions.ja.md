# ADR 0028: 承認待ち session

[English](0028-pending-approval-sessions.md) | 日本語

状態: accepted。repository 実装済み、installed 受け入れは未確認です。

background の通信要求にも、daemon stdin や暗黙の許可を使わず人の確認が必要です。
置き換え可能な Standard queue が待機を制限します。Core の capability application
が Policy 評価後の受付、回答、永続化、監査、最新 Policy／identity 確認、実行を担います。

受け付けた session の object identity で待機と完了を結び付けます。capability service
の実際の戻り値が決まった後だけ完了を渡します。重複して拒否した要求には session を
渡さず、古い完了で置き換わった session を解放しません。bounded channel により、
reviewer が切断しても処理を終えられます。再起動後に要求を再実行しません。
回答送信後のキャンセルは結果が不明な場合であり、取り消しではありません。

共通 review application は小さな interface で独立した source をまとめます。Git は
準備済みの厳密な操作と既存の承認所有権を保ち、scope を再構築せず元の承認要求を
渡します。source 間の重複 ID は拒否します。Core に provider ごとの条件分岐や
新しい Environment／Workspace lifecycle mutation を追加しません。

review 操作を受け付けるのは既存の認可された管理 socket だけです。その principal は
既に Hacocoon を管理できますが、Environment と read-only event bridge にはこの
権限を渡しません。通知の request ID は操作の識別子であり bearer token ではありません。
browser credential、localhost の無認証 POST、guest の管理 socket、通知だけによる
自動決定は追加しません。

不採用: 無制限の待機、background での stdin 読み取り、queue 受付を保存成功と扱う設計、
ID だけの完了所有権、再起動後の未完了承認の復元、UI の値からの Git 権限再構築、
生の provider 出力を review に渡す設計、公開の表示用 stream を通じた承認。
[機能の契約](../design/pending-approval-review.ja.md)を参照してください。
