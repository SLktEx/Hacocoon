# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> **現在の `main` のcode realityを確認するための日本語companion**
>
> 厳密な正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。Roadmapの希望、planned specification、compatibility guaranteeとは分けて扱います。

Hacocoonはまだ **pre-1.0** です。実装済みであっても、CLI/API/state/configが固定されたこと、production support済みであること、real-provider/client acceptanceが完了したことを意味しません。

現在、**実装済みmilestoneは v0.1〜v0.12 まで連続**しています。v0.13 Local OCI Registry / OCI Seed+COWはplannedであり、`main` implementationではありません。

## 現在のrepository reality

| 領域 | 現在の状態 | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `haco exec` / `haco shell` / `haco delete` | v0.1 | unit / process-boundary integration pass。real Incus acceptance pending |
| Workspace model / Lease | canonical Workspace identity、RO/RW lease、RW conflict prevention、process serialization | v0.1-v0.2 | unit / persistence / concurrency / integration pass |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 | unit/process pass。real Incus SSH acceptance pending |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 | unit/process + CLI E2E pass |
| Git / GitHub Capability | host-side brokered push。host credentialをEnvironmentへexportしない | v0.5 | unit / adversarial / real-git / CLI E2E pass |
| Agent / Orchestrator | `haco run`、stable machine JSON、security event export | v0.6 | unit/race/process + CLI E2E pass |
| EC2 Environment / AWS | experimental / disabled by default | v0.7 | fake-AWS integration/E2E pass。real AWS acceptance pending |
| VS Code Client Adapter | `haco-vscode` → loopback SSH → standard Remote-SSH `/workspace` | v0.8 | helper unit。real Windows/WSL + Incus + VS Code acceptance pending |
| Per-agent sandbox broker | `internal/agenthost` がopaque session identityをdedicated Environmentへbind | v0.9 | ownership / persistence / collision / release proof unit coverage |
| Agent binding state | `agent-bindings.json` にownership proofをtrusted stateとして保存。raw session IDはhash化 | v0.9 | lock + atomic/fsync-backed writes |
| VS Code Remote Agent Host Adapter | `haco-agent-host prepare/release`。hashed alias、loopback SSH、client-side private key、`code --agents` | v0.10 | PR #137で`main`実装済み。real Agent Host acceptance pending |
| Base identity | `BaseName` / `BaseRevision` / `BaseRef` をprovider-neutralに保持しEnvironmentへ保存 | v0.11 | unit / routing / fake-Incus E2E |
| Incus Base pinning | logical Base sourceをcreate時にimmutable fingerprintへ解決し、pinned fingerprintからinit | v0.11 | alias movement / malformed fingerprint / injection系unit coverage。real Incus image acceptance pending |
| Base CLI | `haco image list` / `haco image inspect <base>` / `haco create --base <base> ...` | v0.11 | CLI parse + fake-Incus E2E。statusにpersisted Base revisionを出力 |
| Custom Base mapping | `HACO_INCUS_BASES_JSON` でhost/operatorがlogical mappingを追加。`haco/` namespaceは予約 | v0.11 | adversarial input test。build/import/history/rollback/GCは未実装 |
| Resource budget model | CPU / memory bytes / PID / root bytesをfiniteまたはexplicit `unlimited`としてprovider-neutralに保持 | v0.12 | normalization / bounds / invalid inputのunit coverage |
| Resource CLI | `haco create` / `haco run` に `--cpu` / `--memory` / `--pids` / `--root-size` | v0.12 | strict parser unit + fake-Incus E2E |
| Incus resource enforcement | finite limitを`start`前に設定してread-back verify。失敗・不一致ならcreate成功扱いにしない | v0.12 | ordering / mismatch / cleanup unit + fake-Incus E2E。real Incus enforcement pending |
| Managed Incus sandbox network | Hacocoon-managed `haco-sandbox` profile/networkをlocal sandbox pathのdefaultとして使用し、drift/broad fallbackをfail closed | cross-cutting | unit/static integrationあり。real Incus networking acceptanceは別 |
| Unsupported provider behavior | finite budgetをenforceできないproviderはside effect前にfail closed。experimental EC2は現在この経路 | v0.12 | wrapped providerが呼ばれないことをunit test。real AWS pending |
| CI / release hardening | Go matrix、vet、race、docs consistency、bootstrap/release/workflow trust checks | cross-cutting | real provider/client acceptanceは別 |

## 実装の流れ

```text
Workspace
  -> Environment lifecycle
  -> Workspace leases / client access
  -> Policy / Approval / Capability
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> experimental EC2 / AWS
  -> VS Code Client Adapter
  -> trusted session -> Environment binding broker
  -> VS Code Remote Agent Host adapter
  -> logical Base -> immutable revision -> Environment
  -> ResourceBudget -> provider enforcement before Environment access
  -> managed Incus sandbox networking
```

## v0.9 Per-Agent Sandbox

Deterministic Environment nameだけではownership proofになりません。persisted bindingが一致しなければAcquire/Releaseはfail closedします。

Parallel RW agentは別canonical Workspace、通常は別Git worktreeを使います。

## v0.10 Agent Host Adapter

```text
haco-agent-host prepare --session <opaque-id> [workspace]
haco-agent-host release --session <opaque-id>
```

VS CodeがAgent Host/AHPを所有し、HacocoonはEnvironment ownershipと安全なconnection preparationを所有します。Coding agentに`haco` / Incus management authorityを渡しません。

real Windows/WSL + Incus + VS Code Agents window acceptanceはhost-dependentです。

## v0.11 Base Images

実装されたfirst slice:

```text
haco image list
haco image inspect <base>
haco create --base <base> --workspace <path> <environment>
```

logical Baseはcreate時にimmutable revisionへresolveされ、そのidentityをEnvironmentにpersistします。Custom build/import、revision history、rollback、GCはfirst sliceでは未実装です。

## v0.12 Resource Limits

実装されたfirst slice:

```bash
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

未指定dimensionはprovider defaultに放置せず、Hacocoonがexplicit `unlimited` effective budgetに解決してEnvironment metadataへ保存します。

Incusではfinite CPU / memory / PID / root disk limitをEnvironment start前に設定し、read-backで一致を確認します。requested finite limitをproviderがenforceできない場合はsilent ignoreせずfail closedします。

## v0.13はplanned

`13_v0.13_LOCAL_OCI_REGISTRY.md` と `13A_v0.13_OCI_SEED_AND_COW.md` はdesign contractです。

- Local OCI Registry/cache gateway: planned
- transparent Environment-side OCI routing: planned
- OCI Seed + Btrfs/COW optimization: planned second slice

**これらを実装済みとして扱ってはいけません。** `IMPLEMENTATION_STATUS.md` が更新されるまではcurrent code realityではありません。

## Acceptanceの境界

unit test、fake-provider E2E、race、vet、build、repository CIはreal-provider/client acceptanceの代替ではありません。

Real Incus、managed networking/resource enforcement、Windows/WSL + VS Code、Agent Host routing、real image sources、AWS/EC2/SSM/EBSは対応環境で別途確認が必要です。

## Compatibility status

pre-1.0の間はCLI、helper binary、state、provider、Base/image lifecycle、Capability/Policy、client/agent integration、resource-budget behavior、host bootstrap、experimental runtimeをBreaking Changeで修正できます。

ただしcompatibility freedomを理由にunsafe authority boundary、ambiguous ownership、silent data lossを許容しません。
