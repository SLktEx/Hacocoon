# Environment管理network

Status: **canonical Environment providerはimplemented。install済みproxyの稼働とWindows Environment egress受入はpartial。**

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

install済みunitは既存Physical Host controller内のStandard proxyを有効にする。adapterが共有guardを検証してから固定listenerへbindし、準備/bind失敗ならcontroller serviceは起動しない。片方のservice終了で他方も止め、通常HTTP socketとhijack済みCONNECT socketを閉じる。headless require-approvalはfail closedとなる。[ADR 0007](../adr/0007-controller-owned-standard-egress.ja.md) を参照。lifecycleはrepository-implementedであり、packageのEnvironment allow/denyと通常のPolicy操作はpending。

## 受入

repository testは所有権・network/guard設定・lifecycle・source identityを検査し、real-Incus gateはinstall済みWindows受入と別にproviderを検証する。正規Windows installer gateが現在証明するのはtrusted-host基盤疎通と保持である。install済み製品のEnvironment許可/拒否、firewall再読込・起動順、実Docker共存には、それぞれ記録した受入が必要。[実装status](../IMPLEMENTATION_STATUS.ja.md)を参照。
