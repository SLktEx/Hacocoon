# Environment管理network

Status: **canonical Environment providerはimplemented。install済みWindowsでproxy許可/拒否と直接egress拒否の受入が成功。**

現在のIncus SandboxProviderはLinux/WSLのEnvironmentごとに専用managed bridgeを作る。旧shared `haco-sandbox0` / ACL / profile helperはlegacy RuntimeとSeed経路に残るが、現在のEnvironment topologyやfallbackではない。

## 現在のtopologyと所有権

Environmentはprofileを継承せず、明示NICをIncus default resource project内の決定的な名前の `hbr*` bridgeへ接続する。production command adapterは作成時に `user.hacocoon.owner=environment-network-v1` を付け、接続・削除前に照合する。名前の一致だけでは所有権を認めない。

bridgeはIncusが選ぶIPv4 address・DHCP・routingを使い、`ipv4.nat=false`、`ipv4.firewall=true`、`ipv6.address=none`、`raw.dnsmasq=port=0` を要求する。DNS serviceは無効。Incus IPv4 firewallはDHCP/checksum処理のため有効のままとし、それより早いHacocoon inet hookで通信境界を守る。NICは固定managed MACとport isolationを持つ。

共有proxy endpointはPhysical Hostのloopback address `169.254.254.1:18080` であり、各bridge gatewayではない。adapterはupper/lowercase HTTP(S) proxy設定とlocal-only NO_PROXYを渡すが、これらの便利な環境変数は権限を与えない。

## 通信境界

adapterは共有nftables input/forward ruleを照合する。Environment起点のHost通信はDHCPと固定proxyだけに限定し、Host起点通信へのestablished replyとは区別する。外部や別Environmentへの直接転送はdropする。Environmentごとのprerouting guardがmanaged MACとIPv4 subnetを固定し、subnet検査から除くのはaddress取得前のDHCP tupleだけ。

各Environmentは別bridgeを持つため、旧shared L2の前提を適用しない。proxyは接続元をtrusted Incus runtime stateとcontrollerの永続Environment identityへ照合する。hostname承認・public address pinning・HTTPS SNI検証は置換可能なStandard proxyとCore Capability契約が担当する。[egress承認](../EGRESS_AUTHORIZATION.ja.md)を参照。

永続trusted `haco-host` は基盤疎通用の別owned NAT bridgeを使う。そのDNS/HTTPS成功はEnvironmentのproxy迂回が許可された証拠ではない。[trusted-host network](trusted-host.ja.md#専用trusted-host-network)を参照。

## 実装上の制約

canonical data planeはbridge方式だが、helper/constantの一部に移行時の `Routed` / `routed` 名が残る。名前からrouted NIC実装と推測しない。残存shared bridge helperとtestはlegacyの検証であり、現在のEnvironmentをそのNAT経路へ接続する許可ではない。

install済みunitは既存Physical Host controller内のStandard proxyを有効にする。adapterが共有guardを検証してから固定listenerへbindし、準備/bind失敗ならcontroller serviceは起動しない。片方のservice終了で他方も止め、通常HTTP socketとhijack済みCONNECT socketを閉じる。headless require-approvalはfail closedとなる。[ADR 0007](../adr/0007-controller-owned-standard-egress.ja.md) を参照。lifecycleとpackageのEnvironment allow/denyは、明示的な管理者Policy設定を用いてWindowsで受入済み。通常のPolicy管理UIは後続とする。

## 受入

repository testは所有権・network/guard設定・lifecycle・source identityを検査し、real-Incus gateはinstall済みWindows受入と別にproviderを検証する。正規Windows installer gateはtrusted-host基盤疎通と保持を証明する。別段階のinstall済みcontroller検証ではEnvironmentのproxy許可/拒否と直接TCP拒否も成功した。firewall再読込・起動順と実Docker共存は別の受入事項として残る。[実装status](../IMPLEMENTATION_STATUS.ja.md)を参照。

## Policy に従う名前解決

Standard listener は上限付き DNS query を専用の lookup Capability へ送ります。直接 DNS を有効にせず、返した address への接続権限も付与しません。installed Standard mode の guest stub 自動導入は実装済みです。[名前解決](name-resolution.ja.md)と [ADR 0021](../adr/0021-policy-bound-name-resolution.ja.md)を参照してください。

所有権を確認した停止中 Environment は起動前に欠落した volatile source guard を復元します。既存 guard の不一致と稼働中 guest の欠落は fail closed です。[ADR 0022](../adr/0022-resume-volatile-source-guards.md)を参照してください。


## インストール済み Windows の送信元 guard 観測

Windows SSH 受入 fixture に実装済みです。通常の Env 作成と SSH 準備後、読み取り専用 observer が instance の世代と起動状態、隔離 NIC、所有確認済みの非 NAT bridge、実 nftables の送信元 table を確認します。table には prerouting priority -300 で、MAC 拒否、狭い DHCP bootstrap 例外、IPv4 subnet 拒否の順に正確な 3 ルールが必要です。余分な rule／chain、識別違い、不完全な query は失敗とし、状態を修復しません。観測後にも世代と NIC の不変を確認します。

既存の HTTPS／proxy／直接 TCP の受入を補う、実 kernel 設定の確認です。偽装 packet の配送、別 Env の削除、再起動／同名再作成の一連の動作は対象外であり、この observer の成功から推測しません。Windows との統合は既存 SSH gate で実行し、その native 結果は observer の回帰テストと区別して記録します。

専用 Incus／WSL のローカル確認は、停止中の復元 Env を通常 start した後、同じ observer による世代と実 guard ルールの照合が成功しました。最初の起動試行は controller socket 準備前に失敗し、先行する単独 observer 実行も失敗しています。これらを成功扱いにはしません。配布 package を使う Windows SSH gate 全体と偽装 packet の動作は別の受入です。
