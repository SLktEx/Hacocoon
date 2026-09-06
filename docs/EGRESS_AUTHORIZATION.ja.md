# Domain-aware egress authorization

状態: **authorization/enforcement componentはimplemented。install済みproxy serviceの稼働とWindows Environment egress受入はpartial。**

Hacocoonの外向き通信は、sandboxが要求したhostnameそのものをauthorityとして扱います。hostnameを一度DNS解決して得たIPをallowする方式では、shared CDN、DNS変更、direct-IP accessにより別destinationへauthorityが横滑りするため採用しません。

## 境界

egress request / authorization contractはCore、project-maintainedなHTTP/HTTPS enforcement proxyはStandard、Incus固有のbridge / ACL / profile / source identity plumbingはIncus adapterの責務です。

```text
sandbox
  -> 専用Environment bridge + Host traffic guard
  -> application egressは169.254.254.1:18080だけallow
  -> Standard egress proxy
  -> trusted source-IP -> Environment resolution
  -> network.egress/connect Capability
  -> Policy: allow / require-approval / deny
  -> Host DNS resolution + pinned public address
  -> upstream
```

proxyはapproval tokenを発行せず、approvalをIP allowlistとしてcacheしません。grantは1 Environment・canonical hostname・protocol・port・1 connection attemptだけにscopeされます。

## 実装済みauthorization engine

- `internal/core` にprovider-neutralな `EgressRequest` / `EgressGrant` を定義。
- `internal/egress` がDNS hostnameをcanonicalizeし、`network.egress/connect` を既存Policy / Approval / Capability / audit境界へ通す。
- IP literalはpolicy evaluation前にreject。
- `modules/standard/egressproxy` がexplicit HTTP / HTTPS proxy enforcementを実装。
- HTTP absolute-targetと `Host` は同じhostname/portを指す必要がある。
- DNSはhostname authorization後にtrusted Host側だけで解決。
- resolved address setはそのconnectionへpinし、dial時にhostnameを再解決しない。
- private / loopback / link-local / CGNAT / benchmark / documentation / multicast等のunsafe addressをreject。public/private混在answerもresolver順序に依存せず全体をfail closed。
- HTTPS `CONNECT` だけをhostname proofとして信用しない。TLS bytesをupstreamへ送る前にbounded TLS ClientHelloをparseし、SNIがauthorized CONNECT hostnameと同じcanonical hostnameになることを要求。
- provider/audit failureは既存Capability serviceを通じてfail closed。

## 実装済みIncus enforcement

canonical Environment providerはEnvironmentごとのowned bridgeを使い、NAT無効・DHCP有効・DNS無効を要求する。照合済みHost inet ruleとEnvironmentごとのsource guardがproxy経由通信だけを許可する。trusted `haco-host` のNAT bridgeは別の基盤経路であり、proxy環境変数で下位境界を弱めない。topologyと残存legacy経路の正本は[Environment管理network](design/managed-sandbox-network.ja.md)。

proxyはguestの自己申告Environment名を受け取らず、接続元をtrusted Incus runtime stateとcontrollerの永続Environment storeで解決する。固定Physical Host endpoint `169.254.254.1:18080` だけでlistenし、不在・曖昧・unmanaged identityをfail closedにする。restartをまたいでconnection grantを保持せず、hostname grantをIP allowlistへ変換しない。

## Policy例

特定HTTPS hostnameを常時allowする例:

```json
{
  "default": "deny",
  "rules": [
    {
      "capability": "network.egress",
      "action": "connect",
      "resource": "api.example.com",
      "environment": "env-a",
      "attributes": {"protocol": "https", "port": "443"},
      "decision": "allow",
      "reason": "approved development API"
    }
  ]
}
```

connectionごとに既存approval providerの承認を要求する場合は `allow` の代わりに `require-approval` を指定します。Environment / hostname / protocol / portはauditされるauthority scopeに残ります。

## 起動経路

install済みPhysical Host controllerはStandard proxyを構築するが、listenerを起動していない。そのためinstall済みEnvironmentの許可された外向き通信は未受入である。新しい製品 `haco` にegress起動commandはない。移行用binaryのforeground `hacoq egress serve` はlegacy機能であり、installerのserviceや `haco-host` 内へ追加する第二controllerではない。

install済みservice lifecycleの完成では、既存controllerのPolicy・永続source resolver・Standard proxyを再利用し、停止時のfail closedと、対話approval provider不在時の `require-approval` を扱う必要がある。broker不在をNAT/直接通信の開放で補わない。

Git pushは引き続きGit境界の別privileged operationであり、reusable Host Git credentialをEnvironmentへ渡して有効化しない。

## Acceptance boundary

repository testsはallow / deny / require-approval、direct-IP reject、shared-IP / alternate-hostname耐性、mixed/private DNS、SNI mismatch、legacy network migration、unmanaged DNS/ACL drift、trusted source-IP mappingをcoverします。real supported-Incusのbridge / nftables / dnsmasq動作はhost acceptanceとして別に確認します。
