# Environment の名前解決

日本語 | [English](name-resolution.md)

状態: **partial**。Policy に従う resolver、controller HTTP route、UDP/TCP relay component は存在します。Environment への自動設定と Windows/VPN 実機検証は未実装・未検証です。

## 通常の利用方法として目指すもの

既存の Environment 作成時に名前解決を自動設定します。nameserver 引数や新たに覚える command を追加せず、通常の getaddrinfo/getent を使います。設計判断は [ADR 0021](../adr/0021-policy-bound-name-resolution.ja.md) が所有します。

Environment と正規化 hostname に対する `network.resolve` / `lookup` の Policy 判断が必要です。HTTP、HTTPS、private network への接続許可は付与しません。既存の allow/deny/approval と監査を使い、この component は allow rule や Policy UI を追加しません。拒否した名前を platform resolver に渡しません。

## Resolver の所有者

Standard が Physical Host の platform resolver を使います。対応する Windows/WSL では Windows DNS tunneling を upstream とする設計で、WSL の resolv.conf 自動生成を有効に保つ必要があります。public resolver fallback は設けません。[Microsoft の説明](https://learn.microsoft.com/en-us/windows/wsl/troubleshooting#networking-considerations-with-dns-tunneling)を参照してください。

信頼しない guest stub は loopback port 53 と既存の固定 proxy endpoint を使い、Host credential や管理 handle を受け取りません。隔離された peer identity から送信元 ownership を確認します。上限付きの IN A/AAAA のみを扱い、relay は cache しません。trusted Host の infrastructure DNS は別途実機検証が必要です。

## 変更反映と残る検証

同じ public name と利用可能な VPN name を使い、Windows、Physical Host、trusted Host、Environment を比較します。Windows DNS 変更、VPN 接続・切断、WSL 再起動を確認し、platform/application cache と relay の挙動を分けます。現時点の component test では各 VPN の反映タイミングを証明できません。

Policy・監査拒否、送信元 header 偽装、接続許可を伴わない private address、異常 message、UDP/TCP、cancel を component test で確認します。自動導入と実際の getaddrinfo は未完了です。VPN/NRPT・OS 再起動は未実行で、適切な host/VPN fixture がない場合は SKIP として報告します。接続 allow/deny は別途検証し、proxy 内の解決だけを guest の検証成功としません。
