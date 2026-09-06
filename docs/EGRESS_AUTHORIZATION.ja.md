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

永続runtime参照にはprovider routeが含まれる。接続元照合はEnvironment routerの参照decoderを使い、設定した接続元providerとそのnative runtime参照の両方の一致を要求する。別providerの同一native参照には権限を与えない。

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

install済みunitは `haco-controller --standard-egress` を実行する。Incus adapterがguardを検証してから、既存compositionのStandard proxy・Policy・audit・永続source resolverを固定endpointで動かす。引数なしcontrollerは独立したcontrol-transport用に残るが、installerは必ずStandard serviceを有効にする。新しい `haco` にegress起動commandは不要で、残る `hacoq egress serve` はlegacy機能である。

controllerとproxyの停止は連動する。hijack済みCONNECTを含む全proxy接続を閉じ、ClientHello待ち・upstream送信・通信中のrequestをcancelする。header上限は16 KiB、header読取期限は10秒、保持接続上限は256。HTTP transport失敗は任意panic出力を含まない固定structured messageで記録する。

daemonはstdinをambient approvalとして使わない。Policy不在はdeny。exact allowは既存の保護されたPhysical Host policy fileとaudit契約を使い、対話provider不在の `require-approval` は拒否する。承認UIや自動allow policyは追加しない。通常のPolicy管理とinstall済みEnvironment通信受入はpartial。[ADR 0007](adr/0007-controller-owned-standard-egress.ja.md) を参照。

Git pushは引き続きGit境界の別privileged operationであり、reusable Host Git credentialをEnvironmentへ渡して有効化しない。

## Acceptance boundary

Windows workflowは正規BATのjourney成功後、install済みcontrollerのpacket検証を別のstepで行います。Physical Hostの通常userによるAPI clientが読み取り専用Workspace/Environmentを一つ作成し、そのWorkspaceのstatic HTTPS probeを実行して、同じcontroller経由で削除します。第二のcontroller、旧CLI、製品環境変数のoverrideは使いません。文書化済みの管理者 `policy.json` 設定で、そのEnvironmentの `github.com` HTTPS port 443だけを許可します。既存Policyは上書きせず、cleanupは内容が変わっていない検証用Policyだけを削除します。これは明示的なPolicy設定であり、installerやnetworkの修復ではありません。

Probeはinstall済みproxy経由での証明書検証付きHTTPS成功、未許可hostnameのproxy 403、Physical Hostで到達を確認したpublic endpointへの直接TCP接続拒否を要求します。管理socket pathがないことも確認します。Guestのroute起動は観測だけで、package、NAT例外、firewall変更、service override、mount修復は注入しません。controller/providerのpacket受入であり、plannedの製品Environment CLIや通常Policy UIが実装済みという主張ではありません。対象commitごとの結果は実装statusに記録します。

repository testsはallow / deny / require-approval、direct-IP reject、shared-IP / alternate-hostname耐性、mixed/private DNS、SNI mismatch、legacy network migration、unmanaged DNS/ACL drift、trusted source-IP mappingをcoverします。real supported-Incusのbridge / nftables / dnsmasq動作はhost acceptanceとして別に確認します。
