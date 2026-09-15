# 開発環境ごとの名前解決先

状態: 設計合意済み、実装中。[English](0094-environment-resolver-selection.md)

## 決定

CoreはEnvごとに`host`・`backend`・`disabled`を定義します。既定の`host`はPhysical Hostの通常resolverとWSL DNS tunnelingであり、信頼された論理Hostの設定とは区別します。Standardは既存Policy・監査の後、現在の作成実体に結び付いた設定で解決先を選びます。無効時はどちらの上流も呼ばず、不正モードや所有権不一致は拒否します。

backendの処理はprovider境界に置きます。Incus adapterは所有権を確認した信頼済みツール用インスタンスの通常resolverを、固定コードとデータとしての問い合わせで使います。共有bridge DNSや通常Envの直接DNS通信を開放しません。`host`をこの論理Hostのresolverへ置き換えません。

guestの既存の保護されたDNS経路を維持します。disabled時のローカルサービス停止は操作上の措置であり、guestが独自stubを起動してもcontrollerの拒否が権限境界です。DNS成功はHTTP/TCP/UDP/Hostサービス/別Envへの接続許可になりません。同名再作成で別実体の設定を引き継がず、copy・snapshot・転送では承認を複製せずこの設定を保持します。

## 不採用案

resolv.confのコピーだけではplatformの意味を保てません。全guestのIncus dnsmasqを有効にする案、guestが任意上流やHostコマンドを選ぶ案は既存の境界を広げます。基盤固有の処理をCoreやStandardの条件分岐へ入れません。

[名前解決](../design/name-resolution.ja.md)が契約の所有先です。設定・転送・導入済み受入の実績が揃うまで完成とは記録しません。
