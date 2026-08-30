# Hacocoon ドキュメント

[English](README.md) | **日本語**

Hacocoonはpre-1.0です。architecture intent、現在のrepository reality、real-host acceptanceを分けて扱います。

## まず読む

- 現在の実装: [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- 設計原則: [`DESIGN_PRINCIPLES.ja.md`](DESIGN_PRINCIPLES.ja.md)
- Architecture / Roadmap: [`status/architecture-and-roadmap.md`](status/architecture-and-roadmap.md)
- Milestone番号: [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md)
- Security: [`security/security-architecture.md`](security/security-architecture.md)
- Core / Standard / Plugin境界: [`design/plugin-architecture.md`](design/plugin-architecture.md)
- 用語と境界: [`reference/terminology-and-boundaries.md`](reference/terminology-and-boundaries.md)
- Reusable client adapter: [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md)
- Client interaction event: [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md)
- README / docs の書き方: [`DOCUMENTATION_STYLE_GUIDE.md`](DOCUMENTATION_STYLE_GUIDE.md)

## ドキュメントの配置

長く使うpathは「どのversionで入ったか」ではなく「何についての文書か」で決めます。

```text
docs/design/      feature / architecture contract
docs/security/    security / trust boundary
docs/reference/   terminology / reference
docs/status/      roadmap / version / status authority
docs/adr/         architecture decision record。ADR番号はidentityなので例外
```

通常のdoc filenameへmilestone/versionや読む順番のprefixを入れません。

## 正本の優先順

1. `IMPLEMENTATION_STATUS.md` — 現在のcode reality
2. `status/versioning-and-release-status.md` — milestone番号とstatus
3. `DESIGN_PRINCIPLES.md` — cross-cuttingなproduct / architecture constraint
4. `status/architecture-and-roadmap.md` — product boundaryとroadmap intent
5. `reference/terminology-and-boundaries.md` / `security/security-architecture.md`
6. `design/` 配下の該当feature specification
7. 個別のoperational/reference doc
8. README / index

## Core / Standard / Plugin

- **Core**: Environment、Policy / Approval / Capability、interaction、境界制御に必要な安定contractを定義する。
- **Standard**: 通常配布で多くの利用者が使うproject-maintainedな交換可能default implementation。現在のIncus backendや将来のdefault egress proxy/enforcerなど。
- **Plugin**: 無くても一般的なHacocoonとして成立するoptional / specialized integration。nerdctl / Docker / OCI toolingなど。

外向き通信ではegress request / policy / controller contractはCore、具体的なdefault proxy / enforcement implementationはStandardに置きます。現在のmanaged sandbox networkはdefault-deny substrateであり、domain-aware egress authorizationが完成済みという意味ではありません。

## 現在のfeature gate

番号の正本は [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md) です。

| Version | Gate | State |
|---|---|---|
| v0.13 | Managed Sandbox Network | 実装済み |
| v0.14 | Git Fetch Plugin | 実装済み |
| v0.15 | OCI Seed Recommendation | 実装済み |
| v0.16 | OCI Image Deletion | first slice実装済み |
| v0.17 | OCI Seed Builder & Btrfs/COW | first repository slice / partial |
| v0.18 | Docker Compatibility Plugin | repository実装完了。real-host acceptanceは別 |

v0.17がpartialなので、完全実装済みのproduct progressionは **v0.16まで連続**しています。Local OCI Registryはdeferredなoptional infrastructureで、roadmap milestoneを予約しません。

現在のdesign specification:

- [`design/managed-sandbox-network.ja.md`](design/managed-sandbox-network.ja.md)
- [`design/git-fetch-plugin.ja.md`](design/git-fetch-plugin.ja.md)
- [`design/oci-seed-recommendation.ja.md`](design/oci-seed-recommendation.ja.md)
- [`design/oci-image-deletion.ja.md`](design/oci-image-deletion.ja.md)
- [`design/oci-seed-and-cow.ja.md`](design/oci-seed-and-cow.ja.md)
- [`design/docker-compatibility-plugin.ja.md`](design/docker-compatibility-plugin.ja.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](OPTIONAL_LOCAL_OCI_REGISTRY.ja.md) — deferred optional direction

## Reusable client adapter境界

`pkg/clientadapter` はVS Codeに依存しないclient contractで、exact Environment ensure/reuse、status、`/workspace` discovery、loopback SSH/TCP、revoke/delete、`pkg/interaction` batchを提供します。

private keyとIDE configはclient自身が保持し、Hacocoonが受け取るのはSSH public-key materialだけです。返却connectionはloopback-onlyか再検証し、canonical Workspaceやrequested access modeが違うEnvironmentはreuseしません。通常の `haco create` + `haco ssh` + `ssh` がnon-VS-Code proofで、code-server、JetBrains、将来のclientも同じ境界を再利用できます。

詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Client interaction境界

`pkg/interaction` はcapability audit streamをclient-neutralなread-only eventへprojectionします。stable ID、resume可能なbyte cursor、bounded batch、attention/recovery flagを提供し、raw resource、authority attributes、provider output、approval token、free-form audit reasonはclient schemaへ出しません。

eventの観測自体はCapabilityを承認・実行しません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Base と OCI

```text
haco base list
haco base inspect <base>

HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed sample
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed recommend
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed build
HACO_PLUGIN_OCI=nerdctl  haco plugin oci seed current
HACO_PLUGIN_OCI=docker   haco plugin oci docker status <environment>
HACO_PLUGIN_OCI=docker   haco plugin oci docker prepare <environment>
```

BaseはEnvironmentのstarting identity、OCIはoptionalなdeveloper workload toolingです。Coreはcontainerd / nerdctl / Dockerを必須にしません。

## Cloud

provider-neutralなremote/cloud routing seamは維持しますが、concrete EC2/AWS/EBS implementationはactive treeから削除済みで、cloud implementationは現在deferredです。詳細は [`design/remote-and-cloud-runtime.md`](design/remote-and-cloud-runtime.md) を参照してください。

## Versioning

独立して使える1機能 ≒ 1 minor milestone。fix / hardening / refactor / CLI namespace cleanup / CI / docsだけでは通常versionを消費しません。version mappingはstatus docや本文に書き、通常のfilenameには入れません。

## 編集ルール

[`DOCUMENTATION_STYLE_GUIDE.md`](DOCUMENTATION_STYLE_GUIDE.md) に従います。owner docを先に更新し、その後 `IMPLEMENTATION_STATUS.md`、独立した新機能がmilestoneを消費・変更する時だけ `status/versioning-and-release-status.md` を更新します。英語/日本語のcompanionは同じ変更で揃え、最後に実行します。

```bash
python tools/check_docs.py
```
