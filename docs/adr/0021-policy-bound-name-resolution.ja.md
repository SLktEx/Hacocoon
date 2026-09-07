# Policy に従う Environment の名前解決

状態: 設計は accepted、実装は partial。

## 背景と決定

Environment の通常の getaddrinfo を Physical Host の resolver へ中継します。Windows/WSL では DNS tunneling と VPN の名前解決規則を保つことを意図します。HTTP proxy 内の名前解決だけでは、この利用者要件を満たしません。

Core は既存の Capability service を使い、`network.resolve`、action `lookup`、正規化した hostname、送信元 Environment を Policy と監査に渡します。許可と永続監査の後でのみ解決します。private address を返しても接続権限は付与しません。

置換可能な Standard adapter が Physical Host の platform resolver を使います。guest に upstream を選ばせず、固定 public DNS への fallback も設けません。guest の loopback UDP/TCP stub は既存の隔離された HTTP endpoint `169.254.254.1:18080/_haco/dns-query` へ中継します。送信元は永続化された provider ownership から識別し、guest header は信用しません。直接通信の制限は維持します。

通常の Environment 作成時に canonical lifecycle 内で自動設定する設計です。管理 socket、Windows executable、drive、Host credential は公開しません。trusted Host は infrastructure network を維持し、その名前解決経路を別途実機検証します。

## 制限と失敗

IN A/AAAA の単一 question、wire 4096 bytes、最大 32 addresses、同時 32 requests、Host lookup 5 秒が上限です。未対応 request は resolver に渡しません。Policy 拒否は REFUSED、承認・監査・resolver の利用不可は fail closed です。relay cache は持たず TTL は 0 としますが、platform/application cache は別に存在します。

default allow は追加しません。名前解決の許可と接続の許可は別です。拒否や headless approval 時に upstream resolver を呼びません。大きい UDP 応答は truncate して TCP retry を可能にします。cancel 時に listener と処理中の TCP 接続を閉じます。

## 採用しない案

任意 DNS への直接通信は Policy を迂回します。nameserver の手動転記は古くなり VPN 規則を迂回し得ます。Windows 実行権限や Host 管理権限を guest へ渡す必要はありません。DNS 応答から接続許可を自動生成しません。

## 現在の範囲

broker、Standard provider、HTTP handler、guest stub は component として実装・検証済みです。controller の既存 Standard listener に route を接続済みですが、installed Standard mode の作成・再開で guest stub を自動導入します。getaddrinfo、Windows/VPN の変更反映、接続 Policy の実機検証は未完了です。[設計](../design/name-resolution.ja.md)を参照してください。
