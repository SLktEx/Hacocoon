# 実装状況

## WSL向け更新 — 2026-09-06

- 更新mainには #441・#442・#453・#456 と #458/#459 のimage cacheが入った。merge済みPR本文や過去のgreenは、この組合せ候補の受入証拠ではない。
- **専用trusted networkはimplemented:** Incus adapterが `haco-host0` を所有・検証する。fresh hostはprofileを継承せず、installerはstorageからnetwork初期化を推測しない。現在のhostの限定NIC移行はroot disk/UUIDを保持する。Docker共存はowned bridgeと戻り通信だけに限定。installerのDNS/route/HTTPS検証、所有権・移行回帰、隔離Linux packet gateを実装した。[ADR 0005](adr/0005-trusted-host-network-ownership.md)を参照。
- **現在のpackage `af3065a`（2026-09-06）:** 未変更のcached BAT適用・同じ版の再実行とも終了0。所有hostを継承 `incusbr0` NICからprofileなしの明示 `haco-host0` へ移行した。通常入口、root/service事前診断なしの停止状態からの入口、controller protocol 1、製品version、実際のshell/process終了0、DNS・経路・HTTPS 200を確認。保持file・UUID・account状態・全sudo policy hashは移行と再実行の前後で一致した。Btrfs root配置とmount policyを保ち、無関係な旧default pool/profile/bridgeも保持した。fresh作成の成功は以前の `57b6ee2` のもので、この候補では未実行。
- **製品CLI境界:** 隠れた `haco host` の `hacoq` subprocess委譲を削除した。WSL login aliasはcontrollerを直接使い、installer bootstrapの `hacoq host ensure` 依存は明示的な移行残件として残る。未提供の製品commandが明確に失敗する回帰テストを追加。
- **implemented:** Incus-only storage配布とmount policy。残存 `driver`/`source` attachmentを拒否し、検査失敗はfail closed。policy照合中の既存rootfs/Workspace保持をreal-Incus CI契約にも加えた。
- **installer修正はimplemented:** 既定は固定管理account `hacocoon` でpassword入力不要。`-InteractiveUserSetup` はopt-in。root側common準備は検証済み通常UID/GIDを保持し、sudo policyを書かず、管理済みWSL初回設定を完了させる。PS5.1引数転送にも回帰テストがある。[ADR 0004](adr/0004-wsl-installer-authority.md)を参照。
- **cache回帰を修正（2026-09-06）:** `831e5f0` のpackaged BATは `Get-FileHash` が利用できずdistro作成前に失敗した。#441統合時に落ちた #459のmodule非依存SHA-256 helperを復元し、PS5.1回帰テストを追加。修正後のdownloadを検証し、installに成功した `57b6ee2` で再利用した。
- **cache性能修正はimplemented:** image download中のPS5.1のchunkごとの進捗描画を関数内だけ抑制。component testは実際のcache昇格・再利用、hash不一致時のcleanup、呼び出し元の設定保持を検証する。Windows受入では変更していないpackaged BATを使う。
- **Windows受入を実測（2026-09-06）:** `9d459be` の明示的昇格時の失敗後、`57b6ee2` はsystem WSLを元のconsole内で呼び出し、未変更のcached BATが終了0で完了した。入口、Btrfs rootfs、controller疎通、同じ現在版BAT完了は成功。その後、以下の通りshell正常終了の不具合が判明し、待機sessionは再起動・再実行で切断された。trusted-host識別子・file・全sudo policy hashを保持し、product overrideや準備fixtureは使っていない。
- **製品CLIはpartial:** 新 `haco` はhelp/version・controller経由の `doctor` とWSL login aliasを持つ。以下の旧lifecycle・Base・SSH command表記は一時的な `hacoq` の機能。#456の再利用可能なcontroller adapterは実装済みだが、製品commandを `hacoq` へ委譲してはいけない。
- **Seed撤去はplanned:** codeは残り、Base/任意OCIとの依存は[Seed設計](design/oci-seed-and-cow.ja.md)に記録した。Base選択と任意Pluginは保持する。
- **以前のpackage受入（`3f67845`）:** installer生成 `incusbr0` はtrusted-hostのDNS・経路を提供した。`57b6ee2` ではDocker FORWARD policy DROP下でHTTPSがtimeout。packaged `3f67845` ではPhysical Hostとtrusted-hostのHTTPSが200となり、同じ現在版BAT再実行後もtrusted-hostは200だった。その際の読み取り規則はDocker FORWARD policy ACCEPTだった。firewall修復は加えておらず、policy変化の原因・起動順を変えた共存は未検証。Windows gateは両層を要求する。その候補は未使用directory poolも作成した。現在の修正はminimal初期化を撤去し、最終fresh受入はpending。
- **対話終了を修正:** `57b6ee2` の最終観測でshellの `exit` はlogoutを表示してもWSL processが戻らず、後の再起動・再実行で待機connectionが切れた。入口とfile保持は成功したが、正常終了は失敗。adapterはprocess終了をclient入力の開閉から分離し、login aliasはterminalではないcharacter deviceを拒否する。入力を開いたまま正常・非zero終了するprocess回帰を追加。後のpackaged `8a44f17` は停止状態からのWSL入口、controller ping、保持fileのchecksum、shell `exit` 後のWSL process終了0を確認した。
- **WSL起動を修正:** `5b3d36d` のpackaged BAT適用は完了したが、続く通常loginがcontroller socket作成より先に進んだ。その後の読み取り診断では、有効なcontrollerがWSL起動約19秒で正常に開始した。loginは期限付きの読み取り専用pingで準備を待ち、第二のcontrollerを作らない。起動待ち・拒否・timeout回帰は成功。packaged `8a44f17` でHacocoonだけを明示的に停止し、root/service事前診断なしで通常入口とprocess終了0を確認した。account、trusted-host UUID、全sudo policy hashも前のinstallと一致した。
- **実機受入はpending:** `af3065a` のfresh作成、Windows再起動、firewall再読込・起動順変更時のnetwork、Environmentの許可proxy通信と直接通信拒否、SSH、Environment/Workspace作業保持。更新したWindows gate全体の成功は未確認。今回のWSL実機はFORWARD ACCEPTでDOCKER-USER chainはなかった。別のLinux kernel gateではDocker相当DROP下でtrusted往復通信を許可し、未要求inboundと別NICを拒否し、global policy保持を確認した。これはWindows上の実Docker共存やEnvironment proxyの受入ではない。
- **controller診断はimplemented:** `haco doctor [--json]` は期限付き読み取り専用controller APIでruntime・Btrfs storage設定・trusted-host/network所有権・固定対象のtrusted疎通を検査する。修復・`hacoq` 呼出し・guest-local state作成は行わない。failed/skippedや不正応答を成功扱いしない。repository回帰は成功し、packaged受入はpending。[診断契約](design/controller-client-transport.ja.md#host診断)を参照。
- **次の具体的実装:** installerの明示的な `hacoq host ensure` 依存をcontroller経由のbootstrapへ置き換える。Environmentの拒否・proxy、SSH、製品lifecycle導線は別途follow-up。

以下の表は元のcheckpoint時点の履歴文脈を保持する。

[English](IMPLEMENTATION_STATUS.md) | **日本語**

> 現在の `main` の code reality を示す companion です。番号の正本は [`status/versioning-and-release-status.ja.md`](status/versioning-and-release-status.ja.md) です。

Hacocoon は pre-1.0 です。現在のmilestone位置は **v0.26** です。milestoneは軽量なdevelopment checkpointとして扱い、v0.17のacceptance残件のようなpartial状態があっても、後続の実装済みcheckpointへ進めます。repository実装は、明示的に名前を付けたacceptance checkを除き、すべてのreal-host supportを意味しません。

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
| Managed Btrfs rootfs storage | local compositionが `haco-local-default` Incus-owned loop-backed Btrfs poolをlazyにensureし、Hacocoon所有のBase/Tooling/Seed/Environment/trusted-host rootfsをそのpoolへ配置 | v0.20 implemented |
| Managed Btrfs transparent compression | default Incus pool作成時に `compress=zstd:3` を要求する。`compress-force` と `autodefrag` はdesired defaultにせず、mount lifecycleはIncusが所有 | v0.21 implemented |
| Interaction notification clients | `haco-notify` がloopback interaction deliveryをbrowser/native OS向けに提供し、optional VS Code notification extensionも同じinteraction streamを利用。replay/dedup behaviorをtest済み | v0.22 implemented |
| Real Incus E2E acceptance | GitHub-hosted Ubuntu 26.04でstandalone real Incus system-containerを先に検証し、その後fresh runnerでHacocoon Core lifecycle E2Eを実行。systemd/exec、network、hotplug、storage/snapshot、diagnostics、guarded cleanupをphased gateで検証 | v0.23 implemented |
| Structured logging | shared `log/slog` foundation、INFO-default text/JSON output、Environment lifecycle operation field、sanitize済みDEBUG Host-command trace、egress authorization trace、secret redactionをmaintained executableへ実装 | v0.24 implemented |
| Incus-owned Btrfs storage acceptance | actual ordinary-user `haco` をreal Incusへ接続し、lazy pool creation、Incus-owned sparse backing image、loop attach、Btrfs mount、zstd policy、writable Workspace、pool reuse、guarded cleanupまで自動検証 | v0.25 implemented |
| Trusted `haco-host` / default WSL entry | local Incus runtimeがpersistent trusted logical `haco-host` をensure/shellでき、exact ownership markerとreserved-name collision拒否で境界を守る。managed storageを使い、WSL interactive entryはdefaultで`haco-host`へ入り、Physical Host rootは明示recovery pathとして残す。raw Incus controlは`haco-host`へ公開しない | v0.26 implemented |
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

## Trusted `haco-host` / WSL entry

v0.26ではlocal Incus pathにpersistent trusted logical Hostを導入します。`haco host ensure` が `haco-host` を作成・reconcileし、`haco host shell` がensure後に入ります。Hacocoonはexact ownershipをmarkし、非owned instanceとのname collisionを拒否し、managed storageへ配置し、raw Incus control socketを `haco-host` の外に残します。WSL login shimにより通常のinteractive distro entryは `haco-host` を開き、明示的なPhysical Host root entryはrecovery escape hatchとして残ります。

real Incus acceptanceはtrusted-host creation、ownership、idempotent ensure、stopped-state recovery、managed-storage behavior、control-socket non-exposureをcoverします。real Windows/WSL interactive-login acceptanceはhost-dependentです。現在のsliceはlifecycle/default-entryまでで、Git/OCI/credential/control-channelの全面移行はfollow-upです。詳細は [`design/trusted-host.ja.md`](design/trusted-host.ja.md) と [`WINDOWS_WSL_BOOTSTRAP.ja.md`](WINDOWS_WSL_BOOTSTRAP.ja.md) を参照してください。

## Client adapter境界

`pkg/clientadapter` がVS Codeに依存しないreusable adapter-facing contractです。canonical Host Workspaceとrequested access modeが完全一致する場合だけEnvironmentをensure/reuseし、guest内Workspaceは `/workspace` として公開します。connection metadataのreconcileとpublic `pkg/interaction` event contractも同じ境界から利用できます。

SSH prepareが受け取るのはpublic-key materialだけで、private keyとIDE configはclientが保持します。返却/reconcileされたSSH/TCP connectionはloopback-onlyか再検証し、provider outputがcontract違反ならrejectします。既存の `haco create` + `haco ssh` + 通常の `ssh` がnon-VS-Code proofです。詳細は [`CLIENT_ADAPTER_CONTRACT.ja.md`](CLIENT_ADAPTER_CONTRACT.ja.md) を参照してください。

## Core と OCI plugin

containerd / nerdctl / Docker は Hacocoon Core の必須要件ではありません。project-maintained OCI plugin profile が必要に応じて containerd + nerdctl や Docker compatibility を提供します。Base lifecycle は `haco base ...`、OCI workload tooling は `haco plugin oci ...` に分離します。

## OCI Seed / storage

v0.17はbuild/publish、operations-hardening、credential-free managed-Environment harvestのrepository sliceを実装済みです。trusted Host acquisition/cache → offline no-NIC Seed Builder → immutable Seed revision/current pointer → exact-parent resolution → normal Incus/storage-driver clone の経路を維持し、複数Environmentで一つのwritable `/var/lib/containerd` を共有しません。

v0.20ではlocal rootfs storageをIncus-owned Btrfsへ統一します。Environment、Tooling Base builder、Seed builder、trusted hostがroot storageを必要とした時点でlocal compositionが `haco-local-default` をlazyにensureします。sparse backing file、loop device、Btrfs filesystem、mount lifecycleはIncusが所有し、Host Workspaceはpool外からbind mountします。

v0.21ではtransparent compressionのdefault policyとしてIncusへ `btrfs.mount_options=compress=zstd:3` を渡します。`compress-force` と `autodefrag` はdesired defaultにせず、既存extentを自動rewriteしてrecompressしません。

v0.25はこのstorage pathのreal ordinary-user acceptance checkpointです。GitHub-hosted Ubuntu 26.04でactual `haco` をreal Incusへ接続し、lazy `haco-local-default` creation、Incus-owned sparse backing image、loop attachment、live Btrfs mount、zstd policy、writable `haco create` / `exec` / `delete` / `run`、pool reuse、guarded cleanupまで確認します。詳細は [`design/btrfs-storage-layout.ja.md`](design/btrfs-storage-layout.ja.md) を参照してください。

Local Registryはprerequisiteではなくroadmap versionも予約しません。残件はauthenticated/private-registry combination、physical Btrfs compression ratio / CPU cost / COW measurement、compaction behavior、broader real-host failure injection、Windows/WSL behaviorなどです。

## Docker compatibility

v0.18のrepository gateは実装済みです。`HACO_PLUGIN_OCI=docker` で `haco plugin oci docker status <environment>` / `prepare <environment>` を使えます。`prepare` はpackage installやHost socket mountをせず、Base/Seed側にDocker CLI、dockerd、containerd、systemd、docker group、Hacocoon-pinned socket/service unitがあることを要求し、unit driftやactive vendor Docker daemonではfail closedします。

## Cloud status

v0.7のprovider-neutral Environment routing seamは維持します。以前のconcrete EC2/AWS/EBS implementationはactive treeから削除済みで、**cloud implementationは現在deferred**です。

## Acceptance gaps

v0.23でGitHub-hosted Ubuntu 26.04上のphased real-Incus substrate + Core lifecycleを、v0.25でordinary-user Incus-owned Btrfs CLI behaviorを、v0.26でtrusted-host lifecycle/control-socket isolationをreal Incusで自動証明するようになりました。ただしproxy-only bridge ACL/dnsmasqを含む全network/resource behavior、Windows/WSL + VS Codeとinteractive `haco-host` entry、private-registry credential、Docker compatibility、physical Btrfs compression/COW/compaction、broader storage failure injection、desktop notification delivery、future cloud adapterなどは引き続きenvironment-dependentです。前のmilestoneにacceptance残件があっても、後続minor checkpointへ進むことは妨げません。
