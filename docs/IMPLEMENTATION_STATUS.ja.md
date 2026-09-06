# 実装状況

## WSL向け更新 — 2026-09-06

この候補branchのWSL向け実装を以下に示す。今回指定されたWSL M0–M1の範囲は **implemented、受入済み**。更新main `e8974ef` は#441/#442/#453/#456と#458/#459を含む。後続のrelease準備・取消2commitの最終ファイル差分はなく、取込merge `b58f82c` の製品treeは受入対象 `c749ff9` と同じ。merge済みPRや過去のgreenを後続製品変更の受入としない。

- **storageはimplemented:** 配布/runtimeはIncus所有Btrfsだけを使う。外部 `driver`/`source` attachmentや不確実な検査はfail closed。desired policyは `compress=zstd:3,noatime,nodiscard`。[読み取り専用mount診断](design/btrfs-storage-layout.ja.md#読み取り専用のmount診断)は設定・検証済みlive反映・反映待ち `pending` を区別する。backing device/inode、単一の全image loop関連付け、Btrfs root mountの一致を要求し、不明・不正・観測中の変化を成功としない。独自のimage/loop/mount lifecycleや診断修復は追加しない。
- **installerとtrusted Hostはimplemented:** 既定はnon-root `hacocoon`、passwordはlocked。`-InteractiveUserSetup` は任意。現在版再実行はaccount識別/password状態を保持し、sudo policyを書かない。controller所有の `haco setup` が所有trusted hostと限定endpointを準備し、common installerは製品doctorの全項目成功を完了条件とする。fresh hostはprofileを継承せずdeviceを明示する。所有 `haco-host0` はtrusted基盤向けDNS/DHCP/NATを提供し、Docker転送許可はそのbridgeと戻り通信だけに限定する。[bootstrap](WINDOWS_WSL_BOOTSTRAP.ja.md)、[trusted Host](design/trusted-host.ja.md)、ADR [0004](adr/0004-wsl-installer-authority.md)・[0005](adr/0005-trusted-host-network-ownership.md)・[0006](adr/0006-controller-owned-host-setup.md)を参照。
- **製品CLIはpartial:** 新 `haco` はhelp/version・`setup`・`doctor`・controller経由WSL login aliasを提供し、`hacoq` を呼ばない。controller state・Policy・provider・Incus権限はPhysical Hostが所有し、guestにcontrollerやIncus daemonを置かない。[診断](design/controller-client-transport.ja.md#host診断)は順序付き6項目と長さを制限した失敗/pending actionを返す。controller待機とguest DNS/routeの読み取り専用起動待機は、失敗した外部検査の再試行やresource修復をしない。広いlifecycle/Base/SSH CLI移行は別件で、#456のcontroller adapterは再利用できる。
- **Standard proxy lifecycleはimplemented:** install済みcontrollerは固定proxy listenerを所有し、bind前に共有guardを検証する。control/proxyの停止を連動させ、hijack済みCONNECTも閉じる。daemonはambient approval providerを持たず、exact allowのauditを維持し、require-approvalはfail closed。同PID listenerと未管理元拒否はEnvironmentの許可通信と区別する。[ADR 0007](adr/0007-controller-owned-standard-egress.ja.md)を参照。
- **repository検証 — `c749ff9`:** 対象race/vet、pendingのCLI/API回帰、Windows assertion 9件、installer実shell 5件、shell構文、文書検査が成功した。維持する `ci-local.sh test` から全Go shuffle test・vet・JavaScript構文2件・notification test 5件が成功した。先行local vetは `bin/` に取得した調査用sourceを含めて停止したが、その観測資料を `.txt` に直してentry point全体を再実行し成功した。製品環境変数のoverrideやinstall済みresourceの修復は与えていない。
- **Seed撤去はplanned:** codeは残り、[Base/任意OCIとの依存](design/oci-seed-and-cow.ja.md)を保持する。Base選択と任意Pluginの境界は維持する。
- **登録時の続行はimplemented、Windows package受入済み:** WSL一覧取得失敗を不存在とせず、native作成成功後も対象名の登録を読戻し確認する。作成/読戻し失敗時は手動で現在版BATを再実行するための段階/option記録を保存する。記録は権限を与えず、実行もしない。明示的な終了3010は再起動待ちとして伝え、終了0でも未登録なら未完了とし、再起動案内は条件付きにする。PowerShell 5.1 component testと実BATの終了code伝達testは成功した。これらはWindows機能installやOS再起動の受入ではない。[bootstrap続行](WINDOWS_WSL_BOOTSTRAP.ja.md#登録の中断とwindows再起動)を参照。

package受入の対象は **`c749ff9033b33c3526e108f60ce2009638075152`**:

| 環境 | 実測した受入 |
|---|---|
| [Windows gate](https://github.com/SLktEx/Hacocoon/actions/runs/34008408570) | 正規cached BATのfresh作成、通常入口、停止/再入場、同版再実行、cold doctor、build識別、保持、proxy所有、未管理元403が成功 |
| [Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34008411207) | 配布物からのordinary-user installとtrusted-host検査が成功 |
| [Incus gates](https://github.com/SLktEx/Hacocoon/actions/runs/34008410296) | standalone・owned Btrfs・authenticated private registry・Coreの全jobが成功 |
| 現在のWindows実機 | 未変更ZIPの適用と同版BAT再実行はreadiness全6項目成功後に終了0。通常入口、両clientのbuild全体一致、UUID/file/account/sudo policy保持、Btrfs状態、proxy検査が成功。distro停止確認後のdoctorは51.906秒で終了0 |

実機ZIPは `0.26.1-SNAPSHOT-c749ff9`、build日時 `2026-09-06T03:12:38Z`、SHA-256 `f638379fb293cf249f32ef46b5576b95906ff775bc2f00f96ae3ed602724d3f9`。fresh Windowsはrunnerのcurrent WSL基盤でHacocoon distributionがない状態を意味し、Windows機能無効状態やWindows OS再起動の受入ではない。実機の保持証拠はtrusted-host sentinelと基準値であり、Workspaceの未commit・未追跡・未push作業保持の証明ではない。

**未解決の起動失敗:** `42e2fb3` の通常入口で11:33:30 JSTにIncus本体PID 282がSIGKILLを受け、標準600秒start-post待機とcontroller依存が残った。signal送信元は未確定で、得られたkernel記録はOOMを示していない。手動service/mount修復なしで11:43:16にIncus標準の自動再起動が始まり、後の入口/保持検査は成功した。guest DNS/DHCP起動の競合は別途修正・受入済みであり、その修正や後の `c749ff9` 成功からSIGKILL送信元や以前の独立したWSL終了9の原因を確定しない。

**登録package受入 — `4df465a71aedcdc70c28b543220b79b2465808ab`:** [Windows run 34010791925](https://github.com/SLktEx/Hacocoon/actions/runs/34010791925)、job `101426135649` で正規fresh cached BAT、通常入口、停止/再開、同版再実行、データ保持、doctor 6項目、PowerShell/BAT回帰が成功した。手元のPS5.1実一覧/引数伝達、配布/provenance、`ci-local.sh docs` / `workflow-policy`、native文書検査も成功。provenanceの最初のUbuntu 22.04実行は26.04以上の条件で正しく停止し、製品条件を変えず対応基盤で成功した。実機向けZIPのSHA-256は `439dfc8a0a4dab5ef4adf05f1b1ed9b3e02883a5009b66dca7513c528d0d3105`、version `0.26.1-SNAPSHOT-4df465a`、build `2026-09-06T04:10:02Z`。build/checksum確認まで行い、手元で再installは繰り返していない。現在の実機installは受入済み `c749ff9` のままで、変更したfresh登録/再実行はCIで確認した。
**現在のM1範囲:** 最新のユーザー方針により、実Windows OS再起動の実装/受入と続行案内の追加作り込みは対象外。具体的な変更や失敗に見合う検証に絞り、追加で維持する回帰はCIへ置く。新しい根拠なしに成功済み検証を繰り返さない。必須だった既存controller/provider境界を使うinstall済みEnvironmentの許可proxy通信/直接通信拒否の受入は成功した。原因未確定の起動事象は記録に残し、後の限定signal観測でもその原因は特定できていない。診断機能の拡大、firewall起動順の網羅、CLI/SSH開発導線、Workspace保持は後続とし、追加の完了条件にしない。

**Environment接続元の修正:** `f373cfc` のWindows gateは正規BAT経路に成功したが、許可HTTPS probeがproxy 403になった。永続接続元resolverがprovider-local参照とEnvironment作成のroute付き参照を比較していたため、正規router decoderでproviderとnative参照の両方を照合するよう修正した。実際のBase routerの作成結果を使う最小回帰で失敗を再現し、別providerの同一native参照は拒否する。

**M1受入 — `81c0d160722b96864daa8d6f5f3b9ea86423ff48`:** [Windows run 34013409969](https://github.com/SLktEx/Hacocoon/actions/runs/34013409969)、job `101432997324` でfresh cached BAT install、通常入口、停止/再開、同版再実行、trusted-hostデータ保持、doctor 6項目が成功した。install済みcontrollerのEnvironment検証でも、証明書を検証する許可HTTPS、未承認hostnameの403、直接TCP拒否、管理socket非公開、controller cleanupが成功した。CIの対象はPR merge commit `9049df39f8000e32103b6a2f3939ea3d14fc5ffe` で、candidate `81c0d16` と全treeが一致することを確認した。route付き参照の回帰は修正前に失敗し、修正後の手元egress・Environment router・composition・Standard proxy testはすべて成功した。文書整合性検査も成功。

新しい手元ZIPは `0.26.1-SNAPSHOT-81c0d16`、build日時 `2026-09-06T05:12:08Z`、SHA-256 `4938622b994a66b71d5647086819db63e7ee7a7a8ea1189e3b2ad964ccb69c6b`。GoReleaser配布物作成と全checksum検証が成功した。このZIPは現在のWindows実機に再installしておらず、実機のinstall版は `c749ff9` のまま。上記candidateのWindows受入はCIでの結果である。実Windows OS再起動は対象外のままとする。

**次の具体的な一件:** M2として、既存controller-backed adapter経由のEnvironment作成を新 `haco` から利用できるようにする。

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
