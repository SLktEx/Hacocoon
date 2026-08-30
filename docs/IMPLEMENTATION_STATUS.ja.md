# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md) です。

Hacocoon は pre-1.0 です。現在のmilestone位置は **v0.25** です。milestoneは軽量なdevelopment checkpointとして扱い、v0.17のacceptance残件のようなpartial状態があっても、後続の実装済みcheckpointへ進めます。repository実装は、明示的に名前を付けたacceptance checkを除き、すべてのreal-host supportを意味しません。

| 領域 | 現在の状態 | Milestone |
|---|---|---:|
| Runtime / Workspace | Incus Environment lifecycle、Workspace identity、RO/RW lease | v0.1-v0.2 |
| Client access | status、loopback forwarding、SSH prepare/revoke | v0.3 |
| Policy / Capability | fail-closed policy、approval、audit | v0.4 |
| Git push | trusted Host がbrokerし、reusable Host credentialをEnvironmentへ渡さない | v0.5 |
| Agent integration | `haco run`、machine output、events。orchestrationはCore外 | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` がcapability auditを最小化済みeventへprojectionし、stable ID、resume cursor、bounded batch、recovery/attention flag、public corruption errorを提供。観測はcapabilityを承認・実行しない | v0.6 / cross-cutting |
| Environment routing | provider-neutral seamは維持。**具体的なcloud implementationは現在deferred**で、EC2/AWS/EBS実装はactive treeにない | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` がexact Environment ensure/reuse、status、loopback SSH/TCP、revoke/delete、`/workspace` discovery、`pkg/interaction` batchをpackage-owned DTOで公開。通常の `haco ssh` がnon-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`、per-agent binding、`haco-agent-host` | v0.8-v0.10 |
| Base | `haco base list` / `inspect`、immutable Base revision | v0.11 |
| Resource budget | CPU / memory / PID / root storage | v0.12 |
| Managed Sandbox Network | `haco-sandbox0`、proxy-only ACL transport guard、`haco-sandbox` profile。DHCPを残してbridge DNSを停止し、driftはfail closed | v0.13 / cross-cutting |
| Git Fetch Plugin | `haco plugin git fetch`、Host `gh auth git-credential` | v0.14 |
| OCI Seed Recommendation | `haco plugin oci seed sample` / `recommend`、top 10%を `auto_promote=true` | v0.15 implemented |
| OCI Image Deletion | `haco plugin oci image delete`、deletion tombstone、exact immutable identityの明示reenable | v0.16 implemented |
| OCI Seed Builder / Btrfs COW | `seed build/current`、Base単位pin、保守的GC/recover、trusted Host acquisition、managed Environmentからのcredential-free exact-image harvest、offline no-NIC build、immutable publish/current pointer、exact-parent resolutionを実装。real-host/authenticated-registry/COW acceptanceはpending | v0.17 partial |
| Docker Compatibility | `haco plugin oci docker status/prepare`。Base提供profileとpinned systemd unitを検証し、active vendor daemonを勝手に停止せずEnvironment-local socket activationだけを有効化 | v0.18 implemented |
| Domain-aware egress authorization | Core `network.egress/connect`、Standard HTTP/HTTPS proxy、Host DNS pinning、private-address reject、CONNECT/SNI検証、trusted Incus source-IP mapping、`haco egress serve` を実装 | v0.19 implemented |
| Managed Btrfs rootfs storage | storage poolごとにsparse-raw Btrfs filesystemをlazyに用意し、Hacocoon所有のBase/Tooling/Seed/Environment rootfsを対応する `haco-<storage-id>` Incus poolへ固定 | v0.20 implemented |
| Managed Btrfs transparent compression | managed Btrfs mountは標準で `compress=zstd:3`。非準拠mountはremountし、`compress-force`はdesired stateとして採用せず、既存dataの自動recompressionもしない | v0.21 implemented |
| Interaction notification clients | `haco-notify` がloopback interaction deliveryをbrowser/native OS向けに提供し、optional VS Code notification extensionも同じinteraction streamを利用。replay/dedup behaviorをtest済み | v0.22 implemented |
| Real Incus E2E acceptance | GitHub-hosted Ubuntu 26.04でstandalone real Incus system-containerを先に検証し、その後fresh runnerでHacocoon Core lifecycle E2Eを実行。systemd/exec、network、hotplug、storage/snapshot、diagnostics、guarded cleanupをphased gateで検証 | v0.23 implemented |
| Structured logging | shared `log/slog` foundation、INFO-default text/JSON output、Environment lifecycle operation field、sanitize済みDEBUG Host-command trace、egress authorization trace、secret redactionをmaintained executableへ実装 | v0.24 implemented |
| Managed Btrfs Host privilege broker | ordinary `haco` は非rootのまま、root-owned `haco-storage-helper` にtypedなloop/Btrfs/mount operationだけを委譲。helperはmanaged path、hardlink/symlink、loop backing file/inode、filesystem signature、exact mount identityを再検証し、real Ubuntu 26.04でordinary-user `haco create` / `exec` / `delete` / `run` まで自動検証 | v0.25 implemented |
| OCI plugin boundary | `HACO_PLUGIN_OCI=nerdctl|docker` の明示opt-in。未設定でもCoreは動作する | cross-cutting |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Domain-aware egress境界

ordinary HTTP/HTTPS egressはDNS-to-IP ACL近似ではなくStandard proxyでenforceします。Incus NICはdefault denyを維持し、managed bridge gatewayのStandard proxy portへのTCPだけをallowします。bridgeはDHCPを残しつつ `raw.dnsmasq=port=0` でDNS listenerを停止し、unmanaged DNS/ACL configはfail closedです。

managed profileがEnvironmentへHTTP(S) proxy discoveryを提供します。proxyはtrusted Incus source-IP stateからEnvironment identityを導出し、hostname / port / protocolごとに既存Policy / Approval / Capability / audit経路を通し、authorization後だけHost DNSを解決してpublic answer setをconnection単位でpinします。HTTPS CONNECTはTLS bytesをupstreamへ流す前にClientHello SNIとauthorized hostnameの一致を検証します。`haco egress serve` はtrusted Host foregroundの起動経路です。詳細は [`EGRESS_AUTHORIZATION.ja.md`](EGRESS_AUTHORIZATION.ja.md) を参照してください。

## Notification clients

v0.22ではclient-neutral interaction streamをuser-visible notification adapterへ接続しますが、approval authorityはclientへ移しません。`haco-notify` がbrowser/native向けloopback bridgeを提供し、`clients/vscode-notify` がoptional VS Code consumerを提供します。cursor persistence、replay、dedup、corruption handling、browser behavior、VS Code behaviorをrepository testで検証します。notificationの表示は観測だけであり、Capabilityを承認・実行しません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Real Incus E2E acceptance

v0.23は新しいCore APIではなくsupport-confidence checkpointです。GitHub ActionsのUbuntu 26.04上でまずIncus substrateだけを独立に検証し、その後Hacocoon Core lifecycleをfresh runner上のreal Incusで検証します。standalone stageはreal system container、systemd/exec、network、device hotplug、storage/snapshot、diagnostics、exact cleanupを確認し、Core stageはその成立済みsubstrateに対してHacocoon lifecycleを確認します。これによりIncus側のfailureとHacocoon regressionを切り分けやすくし、fake-only E2Eを十分なacceptanceとは扱いません。

## Structured logging

v0.24ではstructured loggingを独立したmilestoneとして扱います。maintained executableは `HACO_LOG_LEVEL` / `HACO_LOG_FORMAT` から1つのshared `log/slog` rootをconfigureします。defaultはINFO/textで、JSONへ切り替えてもstdoutのcommand resultは変えません。Environment create/exec/shell/deleteは `operation`、`environment_id`、duration、result/error fieldをcontext経由で持ち回ります。trusted Host runnerはsanitize済みcommand metadataをDEBUGで追加し、Incus/network/storage/Git/OCIをcomponent分類しますが、subprocess stdout/stderrを自動logしません。

shared handlerはpassword/token/API key、authorization/cookie、credential-bearing URL、secret assignmentのknown patternをDEBUGを含めてdefense-in-depthでredactします。ただしcall site側でもarbitrary header、environment、config object、private key、request body、untrusted outputを渡してはいけません。詳細は [`reference/logging.ja.md`](reference/logging.ja.md) を参照してください。

## Client adapter境界

`pkg/clientadapter` がVS Codeに依存しないreusable adapter-facing contractです。canonical Host Workspaceとrequested access modeが完全一致する場合だけEnvironmentをensure/reuseし、guest内Workspaceは `/workspace` として公開します。connection metadataのreconcileとpublic `pkg/interaction` event contractも同じ境界から利用できます。

SSH prepareが受け取るのはpublic-key materialだけで、private keyとIDE configはclientが保持します。返却/reconcileされたSSH/TCP connectionはloopback-onlyか再検証し、provider outputがcontract違反ならrejectします。既存の `haco create` + `haco ssh` + 通常の `ssh` がnon-VS-Code proofです。詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## OCI Seed / storage

v0.17はbuild/publish、operations-hardening、credential-free managed-Environment harvestのrepository sliceを実装済みです。trusted Host acquisition/cache → offline no-NIC Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver clone の経路を維持し、複数Environmentで一つのwritable `/var/lib/containerd` を共有しません。

v0.20ではSeed固有のCOWだけでなく、local rootfs全体のstorage boundaryをHacocoon管理Btrfsへ統一します。Environment、Tooling Base builder、Seed builderがroot storageを必要とした時点でlazy providerがmanaged attachmentを解決し、対応する `haco-<storage-id>` Incus poolを選びます。

v0.21ではmanaged Btrfsのtransparent compressionを標準化します。初回mountと非準拠の既存managed mountに `compress=zstd:3` を適用し、`compress-force` はdesired stateとして使いません。既存extentの自動defrag/recompressはreflink/COW sharingを減らす可能性があるため行いません。

v0.25ではHost storage privilegeをordinary CLIから分離します。sparse file/state処理はinvoking userのまま行い、必要なHost特権だけをroot-owned `haco-storage-helper` のtyped requestへ変換します。helperは任意command/path/device/mount optionを拒否し、caller-owned managed directory/image、hardlink/symlink state、loop `BACK-FILE` / `BACK-INO`、format前filesystem signature、mount identity/postconditionを再検証します。Hacocoonはpasswordless sudo ruleをinstallしません。GitHub-hosted Ubuntu 26.04 acceptanceではreal helper lifecycleに加え、fresh runnerのreal Incusでactual ordinary-user `haco create` / `exec` / `delete` / `run`、managed pool reuse、ephemeral cleanup、exact pool/mount/loop cleanupまで確認します。詳細は [`design/btrfs-storage-layout.ja.md`](design/btrfs-storage-layout.ja.md) を参照してください。

Local Registryはprerequisiteではなくroadmap versionも予約しません。残件はauthenticated/private-registry combination、physical Btrfs compression ratio / CPU cost / COW measurement、compaction/sparse-hole behavior、broader real-host failure injection、Windows/WSL behaviorなどです。

## Docker compatibility

v0.18のrepository gateは実装済みです。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求し、unit driftやactive vendor Docker daemonではfail closedします。

## Cloud status

v0.7のprovider-neutral Environment routing seamは維持します。以前のconcrete EC2/AWS/EBS implementationはactive treeから削除済みで、**cloud implementationは現在deferred**です。

## Acceptance gaps

v0.23でGitHub-hosted Ubuntu 26.04上のphased real-Incus substrate + Core lifecycleを、v0.25でordinary-user managed-Btrfs CLI + privileged helper boundaryを自動証明するようになりました。ただしproxy-only bridge ACL/dnsmasqを含む全network/resource behavior、Windows/WSL + VS Code、private-registry credential、Docker compatibility、physical Btrfs compression/COW/compaction、broader storage failure injection、desktop notification delivery、future cloud adapterなどは引き続きenvironment-dependentです。前のmilestoneにacceptance残件があっても、後続minor checkpointへ進むことは妨げません。
