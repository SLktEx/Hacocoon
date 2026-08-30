# Domain-aware egress authorization

状態: **repository実装完了。real supported-Incus acceptanceはhost-dependentです。**

Hacocoonの外向き通信は、sandboxが要求したhostnameそのものをauthorityとして扱います。hostnameを一度DNS解決して得たIPをallowする方式では、shared CDN、DNS変更、direct-IP accessにより別destinationへauthorityが横滑りするため採用しません。

## 境界

egress request / authorization contractはCore、project-maintainedなHTTP/HTTPS enforcement proxyはStandard、Incus固有のbridge / ACL / profile / source identity plumbingはIncus adapterの責務です。

```text
sandbox
  -> Incus NIC default deny
  -> bridge gateway:18080だけtransport allow
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

- managed bridgeはDHCPを残しつつ `raw.dnsmasq=port=0` を設定し、bridge DNS serviceを停止する。guest DNSを別exfiltration pathにしない。
- Hacocoon NICのunmatched ingress / egressは引き続き `reject`。
- managed ACLのordinary outbound allowは、Hacocoon bridge gatewayのStandard proxy port (`18080`) へのTCP 1本だけ。旧v0.13のempty ACLはこのruleへmigrationし、それ以外のunmanaged ruleはfail closed。
- managed bridgeの `raw.dnsmasq` がemptyなら `port=0` へmigrationする。他のoperator/custom値が入っている場合は勝手に上書きせずreject。
- managed `haco-sandbox` profileがupper/lowercaseの `HTTP_PROXY` / `HTTPS_PROXY` とlocal-only `NO_PROXY` を注入する。env varを無視するmalicious processでも、NIC ACLより下のdirect trafficは通らない。
- proxyはconnection source IPをtrusted Incus runtime stateへ問い合わせてEnvironment identityを導出する。exactly oneの `haco-*` instanceに一致しないsourceはdeny。
- `haco egress serve` はmanaged networkをverifyしてHacocoon bridge gatewayだけでlistenする。trusted Host foregroundで動かすため、既存の同期stdio `require-approval` providerをそのまま利用できる。
- brokerはrestartをまたぐauthority cacheを持たない。停止・restart中はACLが許可する唯一のtransportにlistenerがいないためfail closed。

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

trusted HostでStandard brokerをforeground起動します。

```text
haco egress serve
```

listen addressはcallerから指定できません。Hacocoonがmanaged Incus bridgeから導出し、bridge / ACL / profileをverifyしてからtrafficを受けます。

Git pushは引き続きGit pluginの別privileged operationです。ordinary pushを動かすためにreusable Host Git credentialをsandboxへ渡してはいけません。

## Acceptance boundary

repository testsはallow / deny / require-approval、direct-IP reject、shared-IP / alternate-hostname耐性、mixed/private DNS、SNI mismatch、legacy network migration、unmanaged DNS/ACL drift、trusted source-IP mappingをcoverします。real supported-Incusのbridge / nftables / dnsmasq動作はhost acceptanceとして別に確認します。
