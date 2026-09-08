# ADR 0026: Git の保存範囲と固定された今回の実行を分ける

状態: accepted。eb16300 の installed 保存 ask／GitHub 検証は成功しました。[正確な範囲](../reference/managed-repository-workflow.md#installed-saved-approval-acceptance)を参照してください。他の保存方針は repository 内検証です。

trusted provider が準備済み要求の再利用範囲を定義します。controller は capability、
action、resource、Environment ID、属性名の全集合を維持し、明示された可変値だけを
wildcard にします。属性の追加・削除、別値、opaque parameter、呼び出し元が与えた
literal wildcard は拒否します。一般 client は保存範囲を指定できません。

Git は context に結び付いた準備済み操作を確認します。repository・登録 remote・ref・
`update_kind=fast-forward` を固定し、old/new OID と operation ID だけを wildcard に
します。fetch は別操作です。属性名完全一致を維持し、将来の権限属性が増えた場合は
見直すまで古いルールを一致させません。既存 push ルールにも update_kind が必要です。

実行は exact request、固定 OID、現在の Environment ID、最新 Policy を照合します。
Git は ancestry と remote old OID を確認します。branch の許可を history rewrite・
branch 削除・作成に広げません。

pending は今回の commit・summary と saved_scope を分けます。既存 approve／deny に
任意で env／全 env の allow・deny・ask を保存し、ask の今回の回答は別に扱います。
表示用 snapshot はコピーし、改ざん・再送で範囲や別要求を操作できなくします。

client は pending で保存対応を確認してから送信し、古い server に保存指定を無視されて
今回だけ実行されることを防ぎます。saved_choice receipt は永続化と監査成功後だけ
返します。その後の Git・通信の失敗でも Policy は rollback しません。再試行前に
Policy と remote を確認します。管理者 deny／ask は保存 allow より優先します。

監査は今回の exact 属性と policy-saved の saved_scope を分けます。どちらも Policy
に見せる権限情報だけで、pack・credential・opaque parameter は記録しません。

属性削除による照合緩和、commit に固定した保存ルール、client の wildcard 指定、
queue 成功を永続化成功と扱う方式、曖昧な push の自動再試行を却下します。

実ローカル Git、bare remote、通常 helper のテストは、GitHub credential や実 provider
受け入れの証拠ではありません。機能の push 受け入れは Hacocoon-test に通常承認で行います。
