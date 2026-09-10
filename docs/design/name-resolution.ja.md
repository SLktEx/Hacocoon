# Environment の名前解決

日本語 | [English](name-resolution.md)

状態: **partial**。Policy に従う resolver、controller HTTP route、UDP/TCP relay component は存在します。installed Standard mode の作成・再開で自動設定します。通常の名前解決と default 拒否は `c05528a` の Windows GHA で成功しました。VPN・変更反映・再起動の検証は未完了です。

## 通常の利用方法として目指すもの

既存の Environment 作成時に名前解決を自動設定します。nameserver 引数や新たに覚える command を追加せず、通常の getaddrinfo/getent を使います。設計判断は [ADR 0021](../adr/0021-policy-bound-name-resolution.ja.md) が所有します。

Environment と正規化 hostname に対する `network.resolve` / `lookup` の Policy 判断が必要です。HTTP、HTTPS、private network への接続許可は付与しません。既存の allow/deny/approval と監査を使い、この component は allow rule や Policy UI を追加しません。拒否した名前を platform resolver に渡しません。

## Resolver の所有者

Standard が Physical Host の platform resolver を使います。対応する Windows/WSL では Windows DNS tunneling を upstream とする設計で、WSL の resolv.conf 自動生成を有効に保つ必要があります。public resolver fallback は設けません。[Microsoft の説明](https://learn.microsoft.com/en-us/windows/wsl/troubleshooting#networking-considerations-with-dns-tunneling)を参照してください。

信頼しない guest stub は loopback port 53 と既存の固定 proxy endpoint を使い、Host credential や管理 handle を受け取りません。隔離された peer identity から送信元 ownership を確認します。上限付きの IN A/AAAA のみを扱い、relay は cache しません。trusted Host の infrastructure DNS は別途実機検証が必要です。

## 変更反映と残る検証

同じ public name と利用可能な VPN name を使い、Windows、Physical Host、trusted Host、Environment を比較します。Windows DNS 変更、VPN 接続・切断、WSL 再起動を確認し、platform/application cache と relay の挙動を分けます。現時点の component test では各 VPN の反映タイミングを証明できません。

Policy・監査拒否、送信元 header 偽装、接続許可を伴わない private address、異常 message、UDP/TCP、cancel を component test で確認します。自動導入は実装済みで、Windows・WSL・trusted Host・Environment の実際の getaddrinfo の一致と default DNS 拒否は、`c05528a` の Windows GHA run 34132173483 で成功しました。VPN/NRPT・OS 再起動は未実行で、適切な host/VPN fixture がない場合は SKIP として報告します。接続 allow/deny は別途検証し、proxy 内の解決だけを guest の検証成功としません。

`72096d8` の Ubuntu/Windows installer 検証は DNS service 自動設定中に失敗し、
getaddrinfo fixture には未到達です。この失敗を SKIP や成功として扱いません。
古い installed substrate 上の独立した local unit 起動 probe は成功して削除済みですが、
現在の installed create 経路の検証にはなりません。診断は許可した処理段階と数値の
service 終了コードに限定し、生の guest log や script 内容を転送しません。

`7eecbdf` の installer 診断で失敗箇所を `daemon-reload` に絞りました。
service の変更前に guest systemd manager を最大 30 秒待ちます。
reload 自体の失敗は再試行しません。起動遅延・timeout・reload 失敗を
shell 回帰テストで確認します。この修正は `c05528a` の Ubuntu・Incus で成功しました。Windows の DNS fixture と VS Code 実接続も成功しましたが、workflow はその後の project setup 検証スクリプトで、setup 実行前に失敗しました。

## 繰り返す setup と service 起動

39b5ce4 の Windows 受入で承認 setup の失敗が再現し、既存の鍵固定 SSH による照会で DNS service の
Result が start-limit-hit と確認できました。各 setup が Env を開始する際、検証済み companion と
unit が同じでも DNS service を毎回 restart していました。

companion と正規の unit を比較し、変更時は従来どおり restart、同一なら systemd start を使います。
稼働中 service は継続し、停止中なら起動します。manager の準備待ち、daemon-reload、enable、
稼働確認、resolver 設定は維持します。systemd の起動制限、接続元識別、network Policy、所有確認は
緩めません。起動失敗は失敗として返します。連続 setup の回帰は観測した起動制限を模擬しますが、
修正後の installed 受入は未完了です。
