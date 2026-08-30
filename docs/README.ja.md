# Hacocoon ドキュメント

[English](README.md) | **日本語**

Hacocoonはpre-1.0です。architecture intent、現在のrepository reality、real-host acceptanceを分けて扱います。

## まず読む

- 現在の実装: [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- 設計原則: [`DESIGN_PRINCIPLES.ja.md`](DESIGN_PRINCIPLES.ja.md)
- Architecture / Roadmap: [`00_REBASELINE_AND_ROADMAP.md`](00_REBASELINE_AND_ROADMAP.md)
- Milestone番号: [`00D_VERSIONING_AND_RELEASE_STATUS.ja.md`](00D_VERSIONING_AND_RELEASE_STATUS.ja.md)
- Security: [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- Core / Standard / Plugin境界: [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)
- Reusable client adapter: [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md)
- Client interaction event: [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md)

## Core / Standard / Pluginのルール

Hacocoonでは、product semanticsとdefault implementation、optional integrationを分けます。

- **Core**: Environment、Policy / Approval / Capability、境界制御に必要な安定contractを定義する。
- **Standard**: 通常配布で多くの利用者が使うproject-maintainedなdefault implementation。現在のIncus backendや、将来のdefault egress proxy/enforcerなど。Core contractを満たす交換可能な実装であり、Coreそのものではない。
- **Plugin**: 無くても一般的なHacocoonとして成立するoptional / specialized integration。nerdctl / Docker / OCI toolingなど。

外向き通信では、egress request / policy / controller contractはCore、具体的なdefault proxy / enforcement implementationはStandardに置きます。この分類はarchitecture intentであり、現在のv0.13はdefault-deny network substrateまでです。domain-awareなallow / approval enforcementが実装済みという意味ではありません。

## 番号ルール

独立して使える1機能 ≒ 1 minor milestone。fix / hardening / refactor / CLI namespace cleanup / CI / docsだけでは通常versionを消費しません。

| Version | Gate | State |
|---|---|---|
| v0.13 | Managed Sandbox Network | 実装済み |
| v0.14 | Git Fetch Plugin | 実装済み |
| v0.15 | OCI Seed Recommendation | 実装済み |
| v0.16 | OCI Image Deletion | first slice実装済み |
| v0.17 | Docker Compatibility Plugin | 実装済み。host acceptanceは別 |
| v0.18 | OCI Seed Builder & Btrfs/COW | first repository slice / partial |

完全実装済みのproduct progressionは **v0.17まで連続**しています。v0.18はfirst repository sliceまで実装済みで、real-host/COW acceptanceが残っています。

Local OCI Registryはdeferredなoptional infrastructureで、roadmap milestoneを予約しません。

## Reusable client adapter境界

`pkg/clientadapter` はVS Codeに依存しないclient contractとして、exact Environment ensure/reuse、state、`/workspace` discovery、loopback SSH/TCP、revoke/delete、`pkg/interaction` batchを公開します。

private keyとIDE configはclientが保持します。Hacocoonへ渡すのはSSH public-key materialだけです。返されたconnectionはloopback-onlyか再検証し、canonical Workspaceやaccess modeが異なるEnvironmentをsilentに再利用しません。

通常の `haco create` + `haco ssh` + `ssh` がnon-VS-Code proof pathです。code-server、JetBrains、将来clientもCoreにclient固有conditionalを増やさず同じcontract上へadapterを作れます。

詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Client interaction境界

`pkg/interaction` は capability audit stream を client-neutral な read-only event へprojectionします。stable ID、resume可能なbyte cursor、bounded batch、attention/recovery flagを提供し、raw resource、attributes、provider output、approval token、free-form audit reasonはclient schemaへ出しません。

これは観測専用の境界です。approval / execution はtrusted Policy/Capability経路に残るため、browser、VS Code、code-server、JetBrains等の複数adapterが同じeventを観測しても、その観測自体がauthorizationにはなりません。

詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Base と OCI

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed sample
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed recommend
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed build
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed current
HACO_PLUGIN_OCI=nerdctl  haco plugin oci image delete <reference>
HACO_PLUGIN_OCI=docker   haco plugin oci docker status <environment>
HACO_PLUGIN_OCI=docker   haco plugin oci docker prepare <environment>
```

BaseはEnvironmentのstarting identity、OCIはoptionalなdeveloper workload toolingです。Coreはcontainerd / nerdctl / Dockerを必須にしません。

Dockerの `prepare` はBase提供のcompatibility profileとHacocoon-pinned systemd unitを検証し、Environment-local socket activationだけを有効化します。Docker packageのinstallや、activeなvendor daemonのsilent stopは行いません。

## OCI storage

v0.18はtrusted Host acquisition/cache、offline no-NIC immutable Seed build/publish、current-Seed resolution、normal Incus/storage-driver cloneのfirst repository sliceを実装済みです。physical Btrfs COW measurementとbroader real-host acceptanceはpendingです。Local Registryはprerequisiteではなく、policyが許せばnormal direct upstream pullを使えます。

- [`18_v0.18_OCI_SEED_AND_COW.ja.md`](18_v0.18_OCI_SEED_AND_COW.ja.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](OPTIONAL_LOCAL_OCI_REGISTRY.ja.md)

## Cloud

v0.7のprovider-neutral routing seamは維持しますが、concrete EC2/AWS/EBS implementationはactive treeから削除済みで、cloud implementationは現在deferredです。
