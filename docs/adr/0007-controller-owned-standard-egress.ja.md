# ADR 0007: install済みStandard egressをcontroller内で動かす

状態: accepted  
日付: 2026-09-06

## 背景

canonical Environment providerは直接通信を禁止するが、install済みcontrollerは
Standard proxyを構築するだけで起動していなかった。legacy foreground commandを
別起動するとPolicy/stateの別compositionになる。HTTP serverの通常のshutdownだけでは、
hijackされたHTTPS CONNECT socketも終了できない。

## 決定

install済みPhysical Host controller unitは `--standard-egress` を有効にする。
同じcompositionがPolicy・audit・永続source identity・Standard proxyを所有する。
Incus adapterが既存のproxy-only guardを準備・検証してから、固定IPv4 endpointへbindする。
準備の期限は30秒。準備やbindに失敗した場合はcontrol serviceを起動しない。
installerはnftablesを明示的に導入し、controller socketの準備を待つ。

引数なしcontrollerはStandard listenerを持たない独立したcontrol-transport入口として残る。
これは明示的なdeployment選択であり、Environmentのfallbackではない。
install済みunitは必ずStandard egressを有効にする。client flag、guest service、
第二controller、任意listen address、NAT開放、firewall無効化は追加しない。

両listenerはcancel scopeを共有する。片方だけが終了した場合は他方も止め、
systemdが再起動するprocess failureとする。停止時にはhijack済みを含む全proxy接続を閉じ、
request contextもcancelする。CONNECTはrequest cancel時にclientとupstreamを閉じ、
ClientHello待ちやprefix送信中も終了する。同時に保持する接続は256、headerは16 KiBを上限とし、
既存ClientHello/SNI・public DNS pinning検査を維持する。

daemon compositionはambient approval providerを持たない。Policy不在はdeny。
exact allowはscopeとauditを維持し、require-approvalはstdinを読まず、request詳細を
journalへ出さずにfail closedとする。既存の対話control sessionはscope付きapproval callbackを
供給できるが、この変更ではproxy承認UIを追加しない。

HTTP serverのerror sinkは固定のstructured failure messageだけを記録する。
任意のpanic文・header・stackはlogへコピーしない。

## 却下した代替案

- legacy foreground brokerやhaco-host内の第二composition。
- proxy不在を直接通信/NATの開放で補うこと。
- daemon stdinや別接続の回答を承認とみなすこと。
- hijack済みCONNECT socketを閉じないHTTP Shutdownだけの利用。
- install済みproxy終了後もcontroller requestを受け続けること。

## 検証と制約

component回帰はserviceの同時終了、CONNECTの各停止段階での実socket終了、
ambient approval拒否、exact Policy判断、選択したerror log、
installerの実unit生成関数を扱う。これらはrepository検証である。
install済みWindows Environmentのallow/deny通信、firewall reload/起動順、
通常のPolicy管理には個別の受入が必要であり、trusted-host疎通では証明しない。

[egress authorization](../EGRESS_AUTHORIZATION.ja.md)と
[実装status](../IMPLEMENTATION_STATUS.ja.md)を参照。

