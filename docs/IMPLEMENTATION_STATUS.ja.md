# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> **現在の `main` のcode realityを確認するための日本語companion**
>
> 厳密な正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。Roadmapの希望、planned specification、compatibility guaranteeとは分けて扱います。

Hacocoonはまだ **pre-1.0** です。実装済みであっても、CLI/API/state/configが固定されたこと、production support済みであること、real-provider/client acceptanceが完了したことを意味しません。

v0.1〜v0.12の既存gateに加え、v0.13 OCIではusage telemetry / recommendation / top-10% auto-selection / image deletion+tombstoneのsliceが実装されています。Seed build/publishとreal Btrfs acceptanceは未完です。以前のv0.7 EC2/AWS/EBS implementationは、local側のcontractが安定するまでdeferredとしてcurrent treeから外しています。

## 現在のrepository reality

| 領域 | 現在の状態 | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `haco exec` / `haco shell` / `haco delete` | v0.1 | repository coverageあり。real Incus acceptance pending |
| Workspace model / Lease | canonical Workspace identity、RO/RW lease、RW conflict prevention、process serialization | v0.1-v0.2 | unit / persistence / concurrency coverage |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 | unit/process coverage。real Incus SSH acceptance pending |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 | unit/process/CLI coverage |
| Git / GitHub Capability | host-side brokered push。host credentialをEnvironmentへexportしない | v0.5 | unit / adversarial / real-git coverage |
| Agent / Orchestrator | `haco run`、stable machine JSON、security event export | v0.6 | unit/race/process coverage |
| Environment routing | provider-neutral routerを維持し、pre-v0.7 bare runtime refはIncusとして互換処理。別Providerのseamはgeneric fake providerでtest | v0.7 | router unit coverage |
| Remote / Cloud Environment | current buildが登録・実装するEnvironment ProviderはIncusのみ。以前のexperimental EC2 runtimeはcurrent treeから削除 | v0.7 historical slice | deferred。Git history / designは将来のadapter再導入用に維持 |
| AWS Capability / EBS helper | 以前の`aws.api` capability、EBS replacement helper、cloud専用E2EもEC2と一緒にactive treeから削除 | v0.7 historical slice | deferred。以前はreal AWS acceptance pendingでproduction acceptance前だった |
| VS Code Client Adapter | `haco-vscode` → loopback SSH → standard Remote-SSH `/workspace` | v0.8 | helper coverage。real Windows/WSL + Incus + VS Code acceptance pending |
| Per-agent sandbox broker | `internal/agenthost` がopaque session identityをdedicated Environmentへbind | v0.9 | ownership / persistence / collision / release proof coverage |
| Agent binding state | `agent-bindings.json` にownership proofをtrusted stateとして保存。raw session IDはhash化 | v0.9 | lock + atomic/fsync-backed writes |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release`。hashed alias、loopback SSH、client-side private key、`code --agents` | v0.10 | PR #137で実装済み。real Agent Host acceptance pending |
| Base identity | `BaseName` / `BaseRevision` / `BaseRef` をprovider-neutralに保持しEnvironmentへ保存 | v0.11 | unit / routing / fake-Incus coverage |
| Incus Base pinning | logical Base sourceをcreate時にimmutable fingerprintへ解決し、pinned fingerprintからinit | v0.11 | alias movement等のcoverage。real Incus image acceptance pending |
| Base CLI | `haco base list` / `haco base inspect <base>` / `haco create --base <base> ...`。曖昧な `haco image ...` は削除 | v0.11 | namespace routing +既存Base coverage |
| Custom Base mapping | `HACO_INCUS_BASES_JSON` でlogical mappingを追加。`haco/` namespaceは予約 | v0.11 | adversarial coverage。build/import/history/rollback/GCは未実装 |
| Resource budget model | CPU / memory bytes / PID / root bytesをfiniteまたはexplicit `unlimited`として保持 | v0.12 | normalization / bounds / persistence coverage |
| Resource CLI | `haco create` / `haco run` に `--cpu` / `--memory` / `--pids` / `--root-size` | v0.12 | parser + fake-Incus coverage |
| Incus resource enforcement | finite limitを`start`前に設定してread-back verify | v0.12 | unit/fake-Incus coverage。real enforcement pending |
| Unsupported provider behavior | requested finite budgetをenforceできないproviderはside effect前にfail closed。cloud固有実装には依存しない | v0.12 | generic wrapped-provider coverage |
| OCI plugin namespace | OCI/containerd/nerdctl固有操作を `haco plugin oci ...` に分離 | v0.13 | CLI namespace routing coverage |
| OCI usage telemetry | `haco plugin oci seed sample` / `haco plugin oci seed recommend`。Environmentごとのlatest image snapshot、6h refresh、30日ranking | v0.13 | `internal/seedstats` unit coverage。real-host acceptance pending |
| OCI auto-selection | eligible immutable identityの上位10%を切り上げ、最低1件 `auto_promote=true` | v0.13 | deterministic unit coverage。Seed buildへの投入は未実装 |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>`。Host cache+tombstone、`--all-environments`で全Environmentまで拡張、`--force`なし | v0.13 | focused unit coverage。replacement Seed publish/GC pending |
| OCI deletion override | tombstoneは明示overrideまでrecommend/auto-promoteより優先 | v0.13 | persisted state/recommendation coverage |
| OCI Seed build/publish | offline Seed Builder、Environment harvesting、immutable Seed publish、Btrfs COW実測 | v0.13 | 未完 / acceptance pending |
| Local OCI Registry | optional optimization。Seedや通常pullの必須依存ではない | v0.13 | design only |
| CI / release hardening | Go matrix、vet、race、docs consistency、bootstrap/release/workflow trust checksを構成 | cross-cutting | 現在GitHub Actionsはrepository-wideにstep実行前failureが発生中。real acceptanceは別 |

## 実装の流れ

```text
Workspace
  -> Environment lifecycle
  -> Workspace leases / client access
  -> Policy / Approval / Capability
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> provider-neutral routing seam (cloud implementationはdeferred)
  -> VS Code Client Adapter
  -> trusted session -> Environment binding broker
  -> VS Code Remote Agent Host adapter
  -> logical Base -> immutable revision -> Environment
  -> ResourceBudget -> provider enforcement
  -> optional OCI plugin -> telemetry / recommend / deletion state
```

## Cloud Runtimeのdeferred

以前のv0.7 EC2 runtime、host-side AWS capability、EBS replacement helper、そのcloud専用E2Eはcurrent implementation treeから意図的に外しています。

Hacocoon本体のCore/state/resource/network/image/client contractがまだ頻繁に変わっており、real cloudで継続的にacceptanceできない状態でcloud-specific implementationだけ追従させると、検証価値よりmaintenance costの方が大きいためです。

これは「cloud providerを今後サポートしない」という意味ではありません。provider-neutral routing seamは残しており、local側のcontractが落ち着いた後にadapterとして復活できます。Git historyとv0.7設計資料も参照用として残しますが、current implementationとして読んではいけません。

## v0.10 Agent Host Adapter

```text
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

Coding agentに`haco` / Incus management authorityを渡しません。real Windows/WSL + Incus + VS Code Agents window acceptanceはhost-dependentです。

## v0.11 Base Images

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

logical Baseはcreate時にimmutable revisionへresolveされ、そのidentityをEnvironmentにpersistします。`haco base` はEnvironment starting point専用で、OCI/container imageは `haco plugin oci` に分離します。

## v0.12 Resource Limits

```bash
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

requested finite limitをproviderがenforceできない場合はsilent ignoreせずfail closedします。

## v0.13 OCI plugin

```text
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
haco plugin oci image delete docker.io/library/node:24 --all-environments
```

usage telemetry、recommendation、上位10% auto-selection、manual deletion tombstoneは実装済みです。一方、Host cacheへのEnvironment image harvesting、offline Seed Builder、immutable Seed publish/current pointer、old Seed GC、real Btrfs COW acceptanceは未完です。

Local Registryはoptionalであり、通常のEnvironment `nerdctl pull` やSeed設計の必須依存ではありません。

## Acceptanceの境界

unit test、fake-provider E2E、race、vet、build、repository CIはreal-provider/client acceptanceの代替ではありません。

Real Incus、managed networking/resource enforcement、Windows/WSL + VS Code、Agent Host routing、real image sources、OCI Seed/Btrfsは対応環境で別途確認が必要です。Cloud acceptanceはcloud implementationを再導入した時点で改めて定義します。

## Compatibility status

pre-1.0の間はCLI、helper binary、state、provider、Base/image lifecycle、Capability/Policy、client/agent integration、resource-budget behavior、host bootstrap、runtimeをBreaking Changeで修正できます。

ただしcompatibility freedomを理由にunsafe authority boundary、ambiguous ownership、silent data lossを許容しません。
