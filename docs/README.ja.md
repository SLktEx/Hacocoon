# Hacocoon ドキュメント

[English](README.md) | **日本語**

Hacocoonはpre-1.0です。architecture intent、現在のrepository reality、development checkpoint番号、公開release、real-host acceptanceを分けて扱います。

## まず読む

- 現在の実装: [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md)
- Development checkpoint正本: [`status/checkpoints.yaml`](status/checkpoints.yaml)
- Development checkpoint policy/status/履歴: [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md)
- 設計原則: [`DESIGN_PRINCIPLES.ja.md`](DESIGN_PRINCIPLES.ja.md)
- Architecture / Roadmap: [`status/architecture-and-roadmap.md`](status/architecture-and-roadmap.md)
- Security: [`security/security-architecture.md`](security/security-architecture.md)
- Trusted logical Host: [`design/trusted-host.ja.md`](design/trusted-host.ja.md)
- Windows / WSL bootstrapとdefault `haco-host` entry: [`WINDOWS_WSL_BOOTSTRAP.ja.md`](WINDOWS_WSL_BOOTSTRAP.ja.md)
- Core / Standard / Plugin境界: [`design/plugin-architecture.md`](design/plugin-architecture.md)
- 用語と境界: [`reference/terminology-and-boundaries.md`](reference/terminology-and-boundaries.md)
- Logging policy: [`reference/logging.ja.md`](reference/logging.ja.md)
- Domain-aware egress: [`EGRESS_AUTHORIZATION.ja.md`](EGRESS_AUTHORIZATION.ja.md)
- Managed Btrfs storage: [`design/btrfs-storage-layout.ja.md`](design/btrfs-storage-layout.ja.md)
- Controller/client transport: [`design/controller-client-transport.ja.md`](design/controller-client-transport.ja.md)
- Reusable client adapter: [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md)
- Client interaction event / notification delivery: [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md)
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
2. `status/checkpoints.yaml` — development checkpoint番号・current checkpoint・Gate identity
3. `status/versioning-and-release-status.md` — checkpoint policy・人間向けstatus・履歴/context
4. `DESIGN_PRINCIPLES.md` — cross-cuttingなproduct / architecture constraint
5. `status/architecture-and-roadmap.md` — product boundaryとfuture direction
6. `reference/terminology-and-boundaries.md` / `security/security-architecture.md`
7. `design/` 配下の該当feature specification
8. 個別のoperational/reference doc
9. README / index

README/indexはcheckpoint tableを意図的に複製しません。現在の `v0.N` とversion/Gate mappingは [`status/checkpoints.yaml`](status/checkpoints.yaml)、人間向けstatusは [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md)、実装詳細は [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) に置きます。

## Core / Standard / Plugin

- **Core**: Environment、Policy / Approval / Capability、interaction、境界制御に必要な安定contractを定義する。
- **Standard**: 通常配布で多くの利用者が使うproject-maintainedな交換可能default implementation。現在のIncus backendとdefault hostname-aware HTTP/HTTPS egress proxy/enforcerを含む。
- **Plugin**: 無くても一般的なHacocoonとして成立するoptional / specialized integration。nerdctl / Docker / OCI toolingなど。

外向き通信ではegress request / policy / controller contractはCore、具体的なdefault proxy / enforcement implementationはStandardに置きます。Incus adapterがproxy-onlyなlower-layer transport guard、bridge DNS disablement、trusted source mappingを提供します。repository実装は完了しており、real supported-Incus acceptanceはhost-dependentです。

`haco-host` はlocal Incus integrationが提供するtrusted infrastructureで、EnvironmentでもOCI pluginの必須要件でもありません。現在のlifecycle / default-entry sliceは実装済みで、Git / OCI / credential / control-channelの全面移行はfollow-upです。

## 現在のcheckpoint

個別feature page、README本文、commit messageから現在のcheckpointを推測しません。次を使います。

- [`status/checkpoints.yaml`](status/checkpoints.yaml) — authoritativeな現在の `v0.N` とordered version/Gate mapping
- [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md) — policyと人間向けcheckpoint status/履歴
- [`IMPLEMENTATION_STATUS.ja.md`](IMPLEMENTATION_STATUS.ja.md) — 実際に何が実装済みか、どのacceptance gapが残るか

pre-1.0のcheckpoint番号は意図的に安く扱います。意味のあるproduct、implementation、operator experience、observability、acceptanceのsliceは、前のreal-host acceptanceが残っていても次minorへ進めて構いません。README/indexは変化するtableを複製せず正本へlinkします。

現在のdesign/reference:

- [`design/trusted-host.ja.md`](design/trusted-host.ja.md)
- [`design/managed-sandbox-network.ja.md`](design/managed-sandbox-network.ja.md)
- [`design/git-fetch-plugin.ja.md`](design/git-fetch-plugin.ja.md)
- [`design/oci-seed-recommendation.ja.md`](design/oci-seed-recommendation.ja.md)
- [`design/oci-image-deletion.ja.md`](design/oci-image-deletion.ja.md)
- [`design/oci-seed-and-cow.ja.md`](design/oci-seed-and-cow.ja.md)
- [`design/docker-compatibility-plugin.ja.md`](design/docker-compatibility-plugin.ja.md)
- [`EGRESS_AUTHORIZATION.ja.md`](EGRESS_AUTHORIZATION.ja.md)
- [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md)
- [`reference/logging.ja.md`](reference/logging.ja.md)
- [`design/btrfs-storage-layout.ja.md`](design/btrfs-storage-layout.ja.md)
- [`design/controller-client-transport.ja.md`](design/controller-client-transport.ja.md)
- [`OPTIONAL_LOCAL_OCI_REGISTRY.ja.md`](OPTIONAL_LOCAL_OCI_REGISTRY.ja.md) — deferred optional direction

## Trusted Host / WSL entry

Supported local Incus pathでは、実際のLinux/WSL substrateである **Physical Host** と、永続的なtrusted logical **`haco-host`** を分けます。Physical HostにはIncus、loop/Btrfs、その他platform authorityを残し、`haco-host` はTCBの一部として扱います。通常のuntrusted Environmentとは別物です。

現在のrepository sliceでは 移行用の `hacoq host ensure` / `hacoq host shell`、exact ownership marker、name collision拒否、managed storage配置、専用WSL login entryを実装しています。Windows install完了後は `wsl -d Hacocoon` を「Hacocoon Hostを開く」入口として扱えます。Raw Incus controlは `haco-host` にmountせず、Physical Hostのroot shellを明示的なrecovery pathとして残します。詳細は [`design/trusted-host.ja.md`](design/trusted-host.ja.md) と [`WINDOWS_WSL_BOOTSTRAP.ja.md`](WINDOWS_WSL_BOOTSTRAP.ja.md) を参照してください。

## Reusable client adapter境界

`pkg/clientadapter` はVS Codeに依存しないclient contractで、exact Environment ensure/reuse、status、`/workspace` discovery、loopback SSH/TCP、revoke/delete、`pkg/interaction` batchを提供します。

private keyとIDE configはclient自身が保持し、Hacocoonが受け取るのはSSH public-key materialだけです。返却connectionはloopback-onlyか再検証し、canonical Workspaceやrequested access modeが違うEnvironmentはreuseしません。通常の `haco create` + `haco ssh` + `ssh` がnon-VS-Code proofで、code-server、JetBrains、将来のclientも同じ境界を再利用できます。

詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Client interaction と notification delivery

`pkg/interaction` はcapability audit streamをclient-neutralなread-only eventへprojectionします。stable ID、resume可能なbyte cursor、bounded batch、attention/recovery flagを提供し、raw resource、authority attributes、provider output、approval token、free-form audit reasonはclient schemaへ出しません。

`haco-notify` とoptional VS Code notification extensionはこのclient-safe streamをconsumeします。eventの観測や通知自体はCapabilityを承認・実行しません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

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

## 編集ルール

[`DOCUMENTATION_STYLE_GUIDE.md`](DOCUMENTATION_STYLE_GUIDE.md) に従います。owner docを先に更新し、その後 `IMPLEMENTATION_STATUS.md` を更新します。development checkpointがminor番号を消費・変更する時は `tools/bump-milestone` を使い、`status/checkpoints.yaml`、status mirror、generated build identityを同期します。英語/日本語のcompanionは同じ変更で揃え、最後に実行します。

```bash
python tools/check_docs.py
```
