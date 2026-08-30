# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> **現在の `main` のcode realityを確認するための日本語companion**

Hacocoonはまだ **pre-1.0** です。実装済みであっても、CLI/API/state/configが固定されたこと、production support済みであること、real-provider/client acceptanceが完了したことを意味しません。

現在、**実装済みmilestoneは v0.1〜v0.17 まで連続**しています。v0.18 / v0.19はplannedです。

## 現在のrepository reality

| 領域 | 現在の状態 | Release | 検証状況 |
|---|---|---:|---|
| Secure Workspace Runtime | `haco create --workspace` / `haco exec` / `haco shell` / `haco delete` | v0.1 | unit / process integration。real Incus acceptanceはhost-dependent |
| Workspace model / Lease | canonical Workspace identity、RO/RW lease、conflict prevention、process serialization | v0.1-v0.2 | unit / persistence / concurrency |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 | unit/process。real SSH acceptance pending |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 | unit/process + CLI E2E |
| Git push plugin | `haco plugin git push`。Host credentialをEnvironmentへexportせずexact SHA/refをbroker | v0.5 | unit/adversarial/real-git/CLI E2E |
| Agent / Orchestrator | `haco run`、stable JSON、security event export | v0.6 | unit/race/process/CLI E2E |
| EC2 provider | experimental / explicit opt-in | v0.7 | fake-AWS。real AWS acceptance pending |
| VS Code adapter | `haco-vscode` → loopback SSH → Remote-SSH | v0.8 | helper test。real Windows/WSL pending |
| Per-agent sandbox | opaque sessionをdedicated Environmentへbindしownership proofをpersist | v0.9 | ownership/persistence/collision coverage |
| Agent Host adapter | `haco-agent-host prepare/release` | v0.10 | repository coverage。real Agent Host pending |
| Base identity | `BaseName` / immutable `BaseRevision`、`haco image list` / `inspect` / `create --base` | v0.11 | unit/fake-Incus |
| Resource budget | CPU / memory / PID / root-storage。finite limitはstart前に設定しread-back | v0.12 | unit/fake-Incus。real enforcement pending |
| Managed sandbox network | Hacocoon-managed Incus network/profile。broad/default fallbackやdriftはfail closed | v0.13 | unit/static integration。real Incus network pending |
| Git fetch plugin | `haco plugin git fetch`。検証済みURL/refspecを使い、HTTPS認証はHostの`gh auth git-credential` | v0.14 | unit/CLI/real-git coverage |
| Optional OCI plugin | `HACO_PLUGIN_OCI=nerdctl|docker`。未設定ならpluginをcomposeしない | v0.15+ | driver/service test。Coreはcontainer CLIなしでも成立 |
| OCI usage / Seed recommendation | `haco plugin oci seed sample|recommend`。Environmentごとのlatest snapshot、immutable digest、top-10% selection | v0.15 | unit/persistence。real tool acceptance pending |
| OCI image deletion | `haco plugin oci image delete`。reference+digest、tombstone、optional all-Environment削除、force削除なし | v0.16 | adversarial/deletion tests |
| Docker compatibility | optional plugin所有のsystemd socket/service。genuine Docker CLI / on-demand Engine compatibility | v0.17 | packaging verification。Base/Seed組み込みとreal-host lifecycle pending |
| Optional Local OCI Registry | normal pullやSeed constructionには必須でない | v0.18 | planned |
| OCI Seed Builder / Btrfs COW | trusted Host acquisition → offline builder → immutable Seed → normal clone/COW | v0.19 | planned |
| CI / release hardening | Go test/race/vet、docs/workflow/release check、`tools/ci-local.sh` | cross-cutting | provider acceptanceとは別 |

## Coreとoptional pluginの境界

Coreが所有するのはWorkspace、Environment lifecycle、execution、client access primitives、Policy/Capability/Audit、Base identity、genericなresource/network safetyです。

**nerdctl、Docker CLI、dockerd、Host OCI cache、Local RegistryはCoreの必須dependencyではありません。**

OCI workload機能が必要なinstallationだけ明示的に有効化します。

```text
HACO_PLUGIN_OCI=nerdctl
# または
HACO_PLUGIN_OCI=docker

haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

Top-levelの `haco image list|inspect` はHacocoon **Base image identity** のcommandです。workload container image管理ではありません。

## Git credential boundary

`haco plugin git fetch` / `push` はtrusted Host側でprivileged Git operationをbrokerします。GitHub HTTPSではHost所有の `gh auth git-credential` を明示利用します。PAT、credential helperのplaintext、SSH private key、authorization headerをEnvironmentやaudit stateへコピーしません。

## Planned

### v0.18 — Optional Local OCI Registry

同一imageの大量再download、rate limit、centralized policy pointが必要なinstallation向けのoptional infrastructureです。ordinary Environment pullにもSeed constructionにも必須ではありません。

### v0.19 — OCI Seed Builder & Btrfs/COW

```text
trusted Host image acquisition
  -> immutable digest
  -> OCI export / stream
  -> offline Seed Builder
  -> containerd clean stop
  -> immutable Incus Seed
  -> normal Incus clone
```

複数Environmentでwritable `/var/lib/containerd`を共有しません。physical block sharingはIncus/storage driverのCOW semanticsに任せます。

## Future client interaction

Browser/Web NotificationやよりrichなInteraction APIはfuture client/adapter workです。VS Code extensionで通知を出す場合もoptionalで、Core transportにはしません。

## Acceptanceの境界

unit、fake-provider E2E、race/vet/build、repository/local CIはreal Incus、Windows/WSL、VS Code、AWS、containerd/nerdctl/Docker、Btrfs acceptanceの代替ではありません。host-dependent acceptanceは別に記録します。
