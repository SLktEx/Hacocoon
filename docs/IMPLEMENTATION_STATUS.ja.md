# 実装状況

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md) です。

Hacocoon は pre-1.0 です。v0.17 OCI Seed Builder & Btrfs/COW がpartialなfeature gateなので、完全実装済みの product progression は **v0.16まで連続**しています。一方、v0.18 Docker Compatibilityのrepository実装は番号入れ替えより先にland済みで、その実装はそのまま有効です。

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
| Domain-aware egress authorization | Core `network.egress/connect`、Standard HTTP/HTTPS proxy、Host DNS pinning、private-address reject、CONNECT/SNI検証、trusted Incus source-IP mapping、`haco egress serve` を実装。real supported-Incus acceptanceはhost-dependent | cross-cutting |
| Git Fetch Plugin | `haco plugin git fetch`、Host `gh auth git-credential` | v0.14 |
| OCI plugin boundary | `HACO_PLUGIN_OCI=nerdctl|docker` の明示opt-in。未設定でもCoreは動作する | cross-cutting |
| OCI Seed Recommendation | `haco plugin oci seed sample` / `recommend`、top 10%を `auto_promote=true` | v0.15 |
| OCI Image Deletion | `haco plugin oci image delete`、deletion tombstone、optional all-environments | v0.16 |
| OCI deletion override | tombstoneはrecommendationと既存pinより優先し、`haco plugin oci image reenable <reference@sha256:...>` でexact immutable identityだけ明示復活できる | v0.16 / v0.17 integration |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build` / `current`、Base単位の `pin` / `unpin` / `pins`、保守的な `seed gc` / `recover`、trusted Host acquisition、明示marker付きrunning managed Environmentからのcredential-free exact-image harvest、offline no-NIC build、immutable publish/current pointer、exact-parent resolution、build前のinterrupted-builder recoveryを実装。real-host/authenticated-registry/COW acceptanceはpending | v0.17 partial |
| Docker Compatibility | `haco plugin oci docker status/prepare`。Base提供profileとpinned systemd unitを検証し、active vendor daemonを勝手に停止せずEnvironment-local socket activationだけを有効化 | v0.18 implemented |
| Optional Local OCI Registry | optional。通常pullやSeed constructionの必須経路ではない | unversioned optional / deferred |

## Domain-aware egress境界

ordinary HTTP/HTTPS egressはDNS-to-IP ACL近似ではなくStandard proxyでenforceします。Incus NICはdefault denyを維持し、managed bridge gatewayのStandard proxy portへのTCPだけをallowします。bridgeはDHCPを残しつつ `raw.dnsmasq=port=0` でDNS listenerを停止し、unmanaged DNS/ACL configはfail closedです。

managed profileがEnvironmentへHTTP(S) proxy discoveryを提供します。proxyはtrusted Incus source-IP stateからEnvironment identityを導出し、hostname / port / protocolごとに既存Policy / Approval / Capability / audit経路を通し、authorization後だけHost DNSを解決してpublic answer setをconnection単位でpinします。HTTPS CONNECTはTLS bytesをupstreamへ流す前にClientHello SNIとauthorized hostnameの一致を検証します。`haco egress serve` はtrusted Host foregroundの起動経路なので、現在のstdio approval providerをそのまま使えます。詳細は [`EGRESS_AUTHORIZATION.ja.md`](EGRESS_AUTHORIZATION.ja.md) を参照してください。

## Client adapter境界

`pkg/clientadapter` がVS Codeに依存しないreusable adapter-facing contractです。exported signatureは `internal/core` typeではなくpackage-owned DTOとpublic error sentinelだけを使います。canonical Host Workspaceとrequested access modeが完全一致する場合だけEnvironmentをensure/reuseし、guest内Workspaceは `/workspace` として公開します。connection metadataのreconcileとpublic `pkg/interaction` event contractも同じ境界から利用できます。

SSH prepareが受け取るのはpublic-key materialだけで、private keyとIDE configはclientが保持します。返却/reconcileされたSSH/TCP connectionはloopback-onlyか再検証し、provider outputがcontract違反ならrejectします。新規作成したinvalid connectionはrevokeし、cleanupを証明できなければrecovery-requiredです。既存の `haco create` + `haco ssh` + 通常の `ssh` がnon-VS-Code proofです。詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Client interaction境界

`pkg/interaction` が reusable なclient-facing event contractです。既存のtrusted capability audit streamを読み、schema/event/request identity、UTC時刻、event kind、Environment/capability/action、attention/recovery flag、closed failure code、次回resume cursorだけを公開します。

raw capability resource、authority attributes、opaque parameters、provider output、approval token、credential、free-form audit reasonはclient schemaへ出しません。browser、VS Code、code-server、JetBrains等のadapterは独立にeventを観測・dedupできますが、eventを読むこと自体には副作用がなく、trusted Policy/Capability approval / execution boundaryの代わりにはなりません。詳細は [`INTERACTION_EVENTS.ja.md`](INTERACTION_EVENTS.ja.md) を参照してください。

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。

Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## OCI Seed / storage

v0.17はbuild/publish、operations-hardeningに加え、credential-free managed-Environment harvestのrepository sliceも実装済みです。trusted Host acquisition/cache → offline no-NIC Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver clone の経路を維持し、複数Environmentで一つのwritable `/var/lib/containerd` を共有しません。

Base単位の明示pinはimmutable OCI identityとしてpersistします。deletion tombstoneはrecommendationと既存pinの両方より優先し、exact identityを明示reenableするまで復活しません。`seed recover` はHacocoon-owned temporary builderをreconcileしてから保守的GCを行い、`seed build` もbuild lock保持中に事前recoveryします。GCはIncus-owned Btrfs internalsを直接触らず、current/in-use/instance-base/external-aliasのimageを保持します。publish後にもdeletion stateを再確認するため、長いbuild中にoperator deleteがraceしてもcurrentへ昇格しません。

exact immutable identityが明示marker付きrunning Hacocoon-managed Environmentに既に存在する場合、temporary `nerdctl save` OCI archiveだけをtrusted Host cacheへcopyし、guest archiveを削除して再利用できます。registry credential、credential-helper output、workspace data、任意のEnvironment file、live containerd stateはcopyしません。legacy/unmarked Environmentはharvest対象にせず、harvest不能時は従来のtrusted Host pullへfallbackします。

Local Registryはprerequisiteではなくroadmap versionも予約しません。残件はreal supported-host Incus/containerd/Docker acceptance、Host-owned credentialを使うauthenticated/private-registry combinationのcredential leakなし検証、physical Btrfs COW measurement、broader real-host failure injectionです。

## Docker compatibility

v0.18のrepository gateは実装済みです。このcodeはfeatureが一時的にv0.17と呼ばれていた時点でlandしましたが、rollbackせずv0.18の実装として扱います。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求します。unit driftや既にactiveなvendor Docker daemonがあればfail closedします。

real Incus/systemd acceptanceはrepository実装とは分離したhost-dependent項目です。

## Cloud

v0.7 の provider-neutral routing seam は残しますが、以前の concrete EC2/AWS/EBS implementation は active tree から意図的に外しています。local/provider contract が安定するまで cloud support は deferred です。
