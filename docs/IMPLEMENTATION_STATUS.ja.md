# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> **現在のcode realityを確認するための日本語companion**
>
> 厳密な正本は [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) です。

Hacocoonはまだ **pre-1.0** です。現在はlocal Incus側のfoundationを優先し、以前のv0.7 EC2/AWS/EBS implementationはdeferredとしてactive treeから外しています。一方、v0.13のOCI機能はCore必須ではなく**任意plugin**として分離しています。

## 現在のrepository reality

| 領域 | 現在の状態 | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `haco exec` / `haco shell` / `haco delete` | v0.1 | repository coverage。real Incus acceptance pending |
| Workspace / Lease | canonical Workspace identity、RO/RW lease、conflict prevention | v0.1-v0.2 | unit/concurrency/process coverage |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 | repository coverage。real Incus SSH pending |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 | unit/process/CLI coverage |
| Git / GitHub | host-side brokered authority。Host credentialをEnvironmentへexportしない | v0.5 | unit/adversarial/real-git coverage |
| Agent / Orchestrator | `haco run`、machine JSON、security event export | v0.6 | unit/race/process coverage |
| Environment routing | provider-neutral seamを維持。current buildが登録するProviderはIncusのみ | v0.7 | router coverage |
| Remote / Cloud | 以前のEC2 runtime、AWS capability、EBS helperはactive treeから削除 | v0.7 historical slice | deferred。Git history/designは将来用に維持 |
| VS Code Client Adapter | `haco-vscode` → loopback SSH → Remote-SSH | v0.8 | helper coverage。real acceptance pending |
| Per-agent sandbox broker | `internal/agenthost`がopaque sessionをdedicated Environmentへbind | v0.9 | persistence/release coverage |
| VS Code Agent Host Adapter | `haco-agent-host prepare/release` | v0.10 | repository coverage。real Agent Host pending |
| Base identity / CLI | `haco base list` / `haco base inspect` / `haco create --base`、immutable revision pinning | v0.11 | unit/routing/fake-Incus coverage |
| Resource budget | CPU / memory / PID / root storageのfinite/unlimited budget、Incus pre-start verify | v0.12 | fake-Incus coverage。real enforcement pending |
| Optional OCI plugin boundary | OCI/container固有実装を`modules/plugin/oci`へ分離。Coreはtelemetry / deletion state / Docker compatibility packagingを所有しない | v0.13 | module boundary + focused unit coverage |
| OCI plugin composition | `HACO_PLUGIN_OCI=nerdctl` / `HACO_PLUGIN_OCI=docker`で明示enable。未設定ならpluginなし | v0.13 | driver parse/selection coverage |
| OCI plugin namespace | OCI/container固有操作は `haco plugin oci ...`。Base lifecycleは`haco base ...` | v0.13 | CLI namespace coverage |
| OCI telemetry | `haco plugin oci seed sample` / `haco plugin oci seed recommend` | v0.13 | `modules/plugin/oci` unit coverage。real-host pending |
| OCI auto-selection | eligible identity上位10%を`auto_promote=true` | v0.13B | deterministic coverage。Seed Builder consume pending |
| OCI image deletion | `haco plugin oci image delete`、tombstone、`--all-environments` | v0.13C | focused plugin coverage。replacement Seed/GC pending |
| Docker compatibility | unitは`modules/plugin/oci/packaging/systemd`所有。Docker driver選択だけでHost Docker authorityを得ない | v0.13 | systemd validation。Base/Seed bake-in pending |
| OCI Seed build/publish | offline builder、immutable Seed publish、real Btrfs/COW | v0.13A | planned |
| Local OCI Registry | optional plugin infrastructure。通常pull/telemetry/Seedの必須依存ではない | v0.13 | design only |

## 実装の流れ

```text
Workspace
  -> Environment lifecycle
  -> local Incus
  -> Workspace leases / client access
  -> Policy / Approval / Capability
  -> Git/GitHub broker
  -> machine/orchestrator access
  -> provider-neutral routing seam (cloudはdeferred)
  -> VS Code Client Adapter
  -> trusted session -> Environment binding
  -> Agent Host adapter
  -> Base -> immutable revision
  -> ResourceBudget -> provider enforcement
  -> optional OCI plugin -> explicit nerdctl/Docker driver
       -> telemetry / recommend / deletion state
```

## Cloud Runtimeのdeferred

以前のv0.7 EC2 runtime、host-side AWS capability、EBS helper、そのcloud専用E2Eはcurrent treeから意図的に外しています。local側のcontractが落ち着いた後にProvider adapterとして戻せるよう、routing seamとGit history/designは維持します。

以前のEC2実装はexperimental/default-offで、**real AWS acceptance pending**でした。production acceptance済みだったわけではありません。

## v0.11 Base

```text
haco base list
haco base inspect <base>
haco create --base <base> --workspace <path> <environment>
```

custom mappingには`HACO_INCUS_BASES_JSON`を使えます。

## v0.12 Resource Limits

```bash
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

## v0.13 Optional OCI plugin

```text
HACO_PLUGIN_OCI=nerdctl haco plugin oci status
HACO_PLUGIN_OCI=docker haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete docker.io/library/node:24
```

`HACO_PLUGIN_OCI`未設定ならOCI pluginはcompositionされず、Coreは`containerd` / `nerdctl` / Docker CLI / Docker Engineをprobe・要求しません。

`containerd + nerdctl`はHacocoonが用意するoptional profileであって、Core要件ではありません。

## Compatibility status

pre-1.0の間はCLI、state、provider、Base/image lifecycle、Capability/Policy、client/agent integration、resource-budget behavior、optional plugin profileをBreaking Changeで修正できます。ただしunsafe authority boundary、ambiguous ownership、silent data lossは許容しません。
