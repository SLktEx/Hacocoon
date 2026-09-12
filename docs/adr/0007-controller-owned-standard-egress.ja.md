# ADR 0007: install済みStandard egressをcontroller内で動かす

状態: accepted  
日付: 2026-09-06

## 背景

正規の Environment プロバイダーは直接通信を禁止するが、install済みコントローラーは
Standard proxyを構築するだけで起動していなかった。旧実装 foreground コマンドを
別起動するとPolicy/stateの別構成になる。HTTP serverの通常のshutdownだけでは、
hijackされたHTTPS CONNECT ソケットも終了できない。

## 決定

install済みPhysical Host コントローラー unitは `--standard-egress` を有効にする。
同じ構成がPolicy・監査・永続送信元の識別・Standard proxyを所有する。
Incus アダプターが既存のproxy-only 保護処理を準備・検証してから、固定IPv4 接続先へbindする。
準備の期限は30秒。準備やbindに失敗した場合はcontrol サービスを起動しない。
インストーラーはnftablesを明示的に導入し、コントローラーソケットの準備を待つ。

引数なしコントローラーはStandard listenerを持たない独立したcontrol-transport入口として残る。
これは明示的なdeployment選択であり、Environmentの代替経路ではない。
install済みunitは必ずStandard egressを有効にする。クライアントフラグ、guest サービス、
第二コントローラー、任意listen アドレス、NAT開放、firewall無効化は追加しない。

両listenerはキャンセル範囲を共有する。片方だけが終了した場合は他方も止め、
systemdが再起動するプロセス失敗とする。停止時にはhijack済みを含む全proxy接続を閉じ、
要求 contextもキャンセルする。CONNECTは要求キャンセル時にクライアントと上流を閉じ、
ClientHello待ちやprefix送信中も終了する。同時に保持する接続は256、headerは16 KiBを上限とし、
既存ClientHello/SNI・公開 DNS pinning検査を維持する。

daemon 構成は暗黙に継承する承認プロバイダーを持たない。Policy不在はdeny。
正確な allowは範囲と監査を維持し、require-approvalはstdinを読まず、要求詳細を
journalへ出さずに安全側で拒否とする。既存の対話control セッションは範囲付き承認 callbackを
供給できるが、この変更ではproxy承認UIを追加しない。

HTTP serverのerror sinkは固定のstructured 失敗 messageだけを記録する。
任意のpanic文・header・stackはlogへコピーしない。

## 却下した代替案

- 旧実装 foreground brokerやhaco-host内の第二構成。
- proxy不在を直接通信/NATの開放で補うこと。
- daemon stdinや別接続の回答を承認とみなすこと。
- hijack済みCONNECT ソケットを閉じないHTTP Shutdownだけの利用。
- install済みproxy終了後もコントローラー要求を受け続けること。

## 検証と制約

構成要素回帰はサービスの同時終了、CONNECTの各停止段階での実ソケット終了、
暗黙に継承する承認拒否、正確な Policy判断、選択したerror log、
インストーラーの実unit生成関数を扱う。これらはリポジトリ検証である。
install済みWindows Environmentのallow/deny通信、firewall reload/起動順、
通常のPolicy管理には個別の受入が必要であり、trusted-host疎通では証明しない。

[egress authorization](../design/egress-authorization.ja.md)と
[実装status](../IMPLEMENTATION_STATUS.ja.md)を参照。

