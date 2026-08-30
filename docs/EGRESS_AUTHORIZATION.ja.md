# Domain-aware egress authorization

状態: **authorization / proxy engine はrepository実装済み。Incus routing/profile integrationは #177 の残件です。**

Hacocoonの外向き通信は、sandboxが要求したhostnameそのものをauthorityとして扱います。hostnameを一度DNS解決して得たIPをallowする方式では、shared CDN、DNS変更、direct-IP accessにより別destinationへauthorityが横滑りするため採用しません。

## 境界

egress request / authorization contractはCore、project-maintainedなHTTP/HTTPS enforcement proxyはStandard、Incus固有のrouting / ACL / profile plumbingはIncus adapterの責務です。

```text
sandbox
  -> default-deny network substrate
  -> Standard egress proxy
  -> network.egress/connect Capability
  -> Policy: allow / require-approval / deny
  -> Host DNS resolution + pinned public address
  -> upstream
```

proxyはapproval tokenを発行せず、approvalをIP allowlistとしてcacheしません。grantは1 Environment・canonical hostname・protocol・port・1 connection attemptだけにscopeされます。

## 実装済みengine

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

## Policy例

最初のengineは既存のexact-resource policy modelを使います。特定HTTPS hostnameを常時allowする例:

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

## 意図的なnon-goalと残るintegration

このrepository sliceだけではsandbox trafficはまだproxyへroutingされません。#177では次が残ります。

- proxyだけをsandboxから到達可能なordinary outbound transportにする。
- arbitrary direct-IP trafficをapplication layerより下でrejectし続ける。
- Incus bridgeのrecursive DNS serviceをexfiltration pathにしない。
- Environment identityをcaller labelではなくtrusted network/runtime stateから導出する。
- Host credentialを渡さずsandbox workloadへstandard proxy discoveryを設定する。
- restart/recoveryをdeterministic/fail-closedにし、real supported-Incus acceptanceを行う。

Git pushは引き続きGit pluginの別privileged operationです。ordinary pushを動かすためにreusable Host Git credentialをsandboxへ渡してはいけません。
