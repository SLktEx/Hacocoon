# v0.13 — Managed Sandbox Network

Status: **`main` に実装済み。real Incus networking acceptance は host-dependent。**

v0.13 は Incus-backed Hacocoon Environment に、Incus `default` profileへ暗黙に依存しない Hacocoon 管理の sandbox network substrate を提供します。

## 実装済み

- managed bridge `haco-sandbox0` の作成・検証
- default-deny substrate `haco-sandbox-egress` ACL の作成・検証
- `haco-sandbox` profile の作成・検証
- 新規 local sandbox Environment の default profile として `haco-sandbox` を使用
- profile/network/ACL drift 時は broad network へfallbackせず fail closed
- anti-spoofing / port isolation は Incus adapter 側で管理
- v0.12 root-disk ResourceBudget と両立

## 境界

このmilestoneはnetwork substrateまでです。Incus ACLはIP/CIDR/address-setベースであり、domain-name authorizationを提供するものとして扱いません。

Domain-aware allow/ask policyは上位のproxy/broker/pluginの責務です。

## Security requirements

- Incus `default` profileへsilent fallbackしない
- managed objectを使用前に検証する
- drift時にsecurityを弱めない
- coding EnvironmentへIncus/Hacocoon control-plane authorityを渡さない
- higher-level egress authorizationは別の明示的境界として扱う

## Acceptance

repository unit/static coverageでは作成・選択・drift rejectionを確認済みです。実環境のbridge/profile/ACL挙動とisolationはsupported Incus hostで別途確認します。
