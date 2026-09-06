# 実装状況

## WSL向け更新 — 2026-09-06

この候補branchは以下のWSL向け実装を含み、M0–M1は引き続き **partial**。更新mainは `e8974ef` で、#441/#442/#453/#456と#458/#459のimage cacheを含む。merge済みPRや過去のgreenは、この候補の受入証拠ではない。

- **storageはimplemented:** Incus所有Btrfsの配布とmount policyへ一本化した。残存する外部 `driver` / `source` attachmentや曖昧な検査結果はfail closed。policy照合中のrootfs/Workspace保持はreal-Incus CI契約に含むが、広い範囲の実COW/compaction受入とは区別する。
- **trusted networkはimplemented:** adapterが `haco-host0` の所有権・設定を検証し、fresh hostはprofileを継承しない。現在hostの限定NIC移行はroot disk/UUIDを保持する。installerはtrusted-hostのDNS・default route・HTTPSを検査する。Docker転送許可はowned bridgeと戻り通信だけに限定。[ADR 0005](adr/0005-trusted-host-network-ownership.md)を参照。
- **installerはimplemented:** 既定は管理account `hacocoon`、password入力不要。`-InteractiveUserSetup` はopt-in。現在版の再実行はaccount識別子とpassword状態を保持し、sudo policyを作らない。PS5.1引数、cache hash/昇格/再利用、読み取り専用account照会の再試行に回帰テストがある。native照会の失敗はaccount不在の証拠ではなく、`63fdf24` で観測した一時的WSL失敗の原因は未確定。[bootstrap](WINDOWS_WSL_BOOTSTRAP.ja.md)と[ADR 0004](adr/0004-wsl-installer-authority.md)を参照。
- **製品CLIはpartial:** 新 `haco` はhelp/version・`setup`・`doctor`・controller経由のWSL login aliasを提供し、`hacoq` へ委譲しない。起動時は既存controllerを待ち、対話processの終了はstdinのcloseを待たない。installerは `haco setup` からcontrollerへbootstrapを依頼する。旧lifecycle/Base/SSH導線は移行残件。#456のcontroller adapterは再利用できる。
- **controller診断はimplemented:** `haco doctor [--json]` は期限付き読み取り専用APIでruntime・Btrfs設定・所有する稼働host/network・固定対象へのtrusted疎通を検査する。修復やguest-local state作成はしない。failed/skippedには選んだ失敗summaryと長さを制限した次の操作を返す。guest出力をコピーせず、固定終了値でDNS/route/HTTPS失敗を区別する。action欠落・不正値や不正応答は成功扱いしない。cause/action追加は対象race/回帰と、下記 `663e2cd` hosted gate・実機の現在install検査が成功した。controller待機上限2分への変更も `32592fb` のWindows package受入が成功した。packaged受入ではclient/controllerのbuild全体の一致を要求する。[診断契約](design/controller-client-transport.ja.md#host診断)を参照。
- **package受入の実測 — `663e2cd`:** 未変更cached BATで現在packageを適用し終了0。その後の通常WSL入口・両clientのdoctor全5項目・build全体一致・保持file/UUID/account/sudo policy/Btrfs基準値が成功した。直接cold doctorは最初にreportなし終了9。読み取り一覧で停止確認後の別cold起動でも、Incusがactivating・その後に起動するcontrollerがinactiveの間に30秒待機が切れた。後の通常入口は成功。kernelにはWSL systemdの10秒boot timeoutも記録された。後続の `32592fb` packageでは直接cold診断2回が成功し、うち1回は読み取り一覧で停止確認後に実行した。正規BATの適用・同版再実行、通常入口、両clientのbuild一致と保持基準も成功。[fresh Windows gate](https://github.com/SLktEx/Hacocoon/actions/runs/34002476669) も同じfull commitで成功した。診断検査は再試行しない。別の終了9の原因や、すべての低速起動の受入を確定する結果ではない。
- **repository検証 — `b71f88e`:** 維持する `ci-local.sh test` から全Go shuffle test・vet・JavaScript構文2件・notification test 5件が成功。対象race・installer段階順序/network/package・docs・workflow policyも成功。Ubuntu Go 1.27.1と検証toolchain用wrapperを通した既存Windows Nodeを使用し、GoReleaserとpackage checksumも成功した。検証toolは製品環境変数の上書きやinstall済みresourceの修復を行っていない。
- **実機受入はpending:** Windows OS再起動、firewall再読込・起動順変更時のnetwork、Environmentの許可proxy通信と直接通信拒否、SSH、Environment/Workspace作業保持。M1の広い層別診断と再起動時の続行もpartial。firewallはDocker FORWARD DROP、DOCKER-USERなしのACCEPT、`2d79778` ではDOCKER-USERと限定trusted-bridge ruleありのACCEPTと観測が変化した。受入時の修復は加えておらず、起動順とcold疎通失敗は未検証。隔離Linux packet gateはWindows上の実Docker共存やinstall済みEnvironment proxy受入ではない。
- **Seed撤去はplanned:** codeは残り、[Base/任意OCIとの依存](design/oci-seed-and-cow.ja.md)を文書化した。Base選択と任意Pluginは保持する。
- **commitを固定したhosted受入 — `663e2cd`:** [Windows正規BAT/停止/再実行/cold doctor](https://github.com/SLktEx/Hacocoon/actions/runs/34001263337)、[Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34001266478)、[Incus全4job](https://github.com/SLktEx/Hacocoon/actions/runs/34001265234) が成功。Windows freshはrunnerのcurrent WSL基盤でHacocoon distributionがない状態からの作成であり、Windows機能無効状態やOS再起動の受入ではない。以前のharness修正2件はnegative回帰を維持する。後続の製品変更には独自の受入が必要。
- **controller所有setupはimplemented:** installerは `haco setup` を使い、旧bootstrap経路とguestへのhacoq配備を撤去した。固定companionの事前検証、client配備の途中失敗、所有権保持、同時実行・client切断、安全な失敗、installerの段階順序をテストする。上記package/hosted結果はこの置換を含む。[ADR 0006](adr/0006-controller-owned-host-setup.md)を参照。
- **Standard proxy lifecycleはimplemented、package受入pending:** install済みunitは既存Physical Host controllerのStandard proxyを有効にし、nftablesを明示的に導入する。bind前に共有guardを検証し、control/proxyの終了を連動させ、停止時にはhijack済みCONNECTも閉じる。daemonはambient approval providerを持たず、exact allowはauditを維持し、require-approvalはfail closedとなる。維持されるlocal Go全suite/vet/通知client検証、対象race、実socket cancel/半切断、Policy、logging、Windows assertion 9件、installer実shell 5件、文書整合性が成功した。この後続service変更は以前の `32592fb` package結果には含まれない。[ADR 0007](adr/0007-controller-owned-standard-egress.ja.md) を参照。
- **次の具体的実装:** install済みproxy packageを受入確認し、通常のcontroller-backed Environment・exact Policy操作を公開して、許可proxyと禁止直接通信を検証する。残るM1の層別/live-storage診断と再起動時の続行を完成させる。

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
