# Environment管理network

Status: **正規の Environment プロバイダーは実装済み。install済みWindowsでproxy許可/拒否と直接egress拒否の受入が成功。**

現在のIncus SandboxProviderはLinux/WSLのEnvironmentごとに専用管理対象の bridgeを作る。旧shared `haco-sandbox0` / ACL / プロファイル helperは旧実装 RuntimeとSeed経路に残るが、現在のEnvironment 構成や代替経路ではない。

## 現在のtopologyと所有権

Environmentはプロファイルを継承せず、明示NICをIncus 既定 resource project内の決定的な名前の `hbr*` bridgeへ接続する。production コマンドアダプターは作成時に `user.hacocoon.owner=environment-network-v1` を付け、接続・削除前に照合する。名前の一致だけでは所有権を認めない。

bridgeはIncusが選ぶIPv4 アドレス・DHCP・経路選択を使い、`ipv4.nat=false`、`ipv4.firewall=true`、`ipv6.address=none`、`raw.dnsmasq=port=0` を要求する。DNS サービスは無効。Incus IPv4 firewallはDHCP/checksum処理のため有効のままとし、それより早いHacocoon inet hookで通信境界を守る。NICは固定管理対象の MACとポート isolationを持つ。

共有proxy 接続先はPhysical Hostのループバックアドレス `169.254.254.1:18080` であり、各bridge gatewayではない。アダプターはupper/lowercase HTTP(S) proxy設定とlocal-only NO_PROXYを渡すが、これらの便利な環境変数は権限を与えない。

## 通信境界

アダプターは共有nftables input/forward ルールを照合する。Environment起点のHost通信はDHCPと固定proxyだけに限定し、Host起点通信へのestablished replyとは区別する。外部や別Environmentへの直接転送はdropする。Environmentごとのprerouting 保護処理が管理対象の MACとIPv4 subnetを固定し、subnet検査から除くのはアドレス取得前のDHCP tupleだけ。

各Environmentは別bridgeを持つため、旧shared L2の前提を適用しない。proxyは接続元を信頼された Incus 実行基盤状態とコントローラーの永続Environment 識別へ照合する。hostname承認・公開アドレス pinning・HTTPS SNI検証は置換可能なStandard proxyとCore Capability契約が担当する。[egress承認](egress-authorization.ja.md)を参照。

永続信頼された `haco-host` は基盤疎通用の別owned NAT bridgeを使う。そのDNS/HTTPS成功はEnvironmentのproxy迂回が許可された証拠ではない。[trusted-host ネットワーク](trusted-host.ja.md#専用trusted-host-network)を参照。

## 実装上の制約

正規のデータ planeはbridge方式だが、helper/constantの一部に移行時の `Routed` / `routed` 名が残る。名前からrouted NIC実装と推測しない。残存shared bridge helperとテストは旧実装の検証であり、現在のEnvironmentをそのNAT経路へ接続する許可ではない。

install済みunitは既存Physical Host コントローラー内のStandard proxyを有効にする。アダプターが共有保護処理を検証してから固定listenerへbindし、準備/bind失敗ならコントローラーサービスは起動しない。片方のサービス終了で他方も止め、通常HTTP ソケットとhijack済みCONNECT ソケットを閉じる。headless require-approvalは安全側で拒否となる。[ADR 0007](../adr/0007-controller-owned-standard-egress.ja.md) を参照。ライフサイクルとパッケージのEnvironment allow/denyは、明示的な管理者Policy設定を用いてWindowsで受入済み。通常の Policy 確認・編集は `haco config`、承認待ちの確認は `haco approve` で行います。

## 受入

リポジトリ内のテストは所有権・network/guard設定・ライフサイクル・送信元の識別を検査し、実際の Incus gateはinstall済みWindows受入と別にプロバイダーを検証する。正規Windows インストーラー gateはtrusted-host基盤疎通と保持を証明する。別段階のinstall済みコントローラー検証ではEnvironmentのproxy許可/拒否と直接TCP拒否も成功した。firewall再読込・起動順と実Docker共存は別の受入事項として残る。[実装status](../IMPLEMENTATION_STATUS.ja.md)を参照。

## Policy に従う名前解決

Standard listener は上限付き DNS query を専用の lookup Capability へ送ります。直接 DNS を有効にせず、返したアドレスへの接続権限も付与しません。導入済み Standard モードの guest stub 自動導入は実装済みです。[名前解決](name-resolution.ja.md)と [ADR 0021](../adr/0021-policy-bound-name-resolution.ja.md)を参照してください。

所有権を確認した停止中 Environment は起動前に欠落した volatile 送信元保護処理を復元します。既存保護処理の不一致と稼働中 guest の欠落は安全側で拒否です。[ADR 0022](../adr/0022-resume-volatile-source-guards.md)を参照してください。

## 導入済みWindowsでの送信元保護の観測

WindowsのSSH検証には、通常のEnv作成とSSH準備後に動く読み取り専用の観測処理があります。Envの世代を固定し、稼働状態、分離したNIC、所有確認済みの非NATブリッジ、カーネルのnftablesテーブルを検査します。テーブルには、MAC不一致の拒否、狭いDHCP初期化例外、IPv4サブネット不一致の拒否がこの順序で必要です。preroutingの優先度は-300です。余分なルールやチェーン、ID不一致、照会不足は失敗とし、修復しません。観測後に世代とNICのIDを再確認します。

これはHTTPS・プロキシ・直接TCPの検証を補います。実際のカーネル設定を調べますが、偽装パケットの送信、別Envの削除、再起動・再作成の一連の動作は検証しません。観測処理の回帰試験と、パッケージ導入からのWindows SSH検証は別の証拠です。限定したローカル成功、準備待ちでの失敗、残る未確認範囲は[検証証拠](../status/acceptance-evidence.ja.md#development)を参照してください。

## Policy に結び付いた開発用 relay

Standard endpoint は [明示的な TCP/UDP 開発接続](network-connections.md)にも
対応する。通常クライアントはゲスト内の loopback listener を選択する。
既存 HTTP/SNI、bridge 送信元検証、既定の packet 拒否を維持し、管理 API や
Incus socket をゲスト endpoint へ公開しない。
