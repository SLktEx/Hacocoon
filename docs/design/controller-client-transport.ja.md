# Controller client transport

日本語 | [**English**](controller-client-transport.md)

Status: **partial**。Local Unix domain protocol、Physical Host controller、trusted-host endpoint投影、client-only `haco-host`、typed Environment API、対話streamは実装済み。製品 `haco` は現在help/version・controller経由の `doctor` とcontroller-backed WSL login aliasを提供する。新CLIのEnvironment lifecycle、PTY制御、port forwarding、remote transportはplanned。

## 概要

現在のreset CLI境界: 製品 `haco` はhelp/version・controller経由の `doctor` とWSL login aliasを持ち、このpageの旧 `haco env ...` 記述は保持している移行CLIの機能を指す。[実装status](../IMPLEMENTATION_STATUS.ja.md)を参照。

WSLは有効なcontroller serviceがsocketをbindする前にlogin shellを開くことがある。login aliasは読み取り専用pingで最大30秒待ち、transport未準備だけをretryする。protocol・operationの拒否はretryせず、clientが第二のcontrollerを起動したりservice状態を変更したりしない。この起動待ち期限は対話sessionの寿命を制限しない。

対話sessionはremote shell終了後にlocal stdinが閉じられるまで待ってはいけない。Incus adapterは子processへ専用OS stdin pipeを渡してclosureを所有し、controllerはprocess終了結果の記録後にclient connectionを閉じる。outputをdrainし、実際のexit statusを保持する。Windows受入で、以前のsocket reader直接指定では `exit` 後も終了待ちする不具合が見つかった。component testはclient入力を開いたまま正常・非zero終了を確認する。WSL login aliasも実際のterminal fdを要求し、`/dev/null` のようなcharacter deviceからtrusted-host shellを開始しない。

Hacocoon Clientはraw Incus authorityを直接受け取らず、trusted Physical Host controllerにEnvironment / Host-authority operationを要求します。

Local pathは次です。

```text
client (`haco`, `haco-host`, future adapters)
  |
  | Hacocoon Unix-domain endpoint
  v
Physical Host haco-controller
  |
  | provider/backend boundary
  v
Incus or another Environment backend
```

Trusted `haco-host`内ではclient endpointを次のように投影します。

```text
trusted haco-host
  |
  | /var/lib/hacocoon-control.sock
  | Incus proxy device: haco-control
  v
Physical Host /run/hacocoon/control.sock
```

Local IPC hopを1つ増やすことは意図的です。Policy、Approval、authoritative state、logging、provider authorityはcontroller側に集約します。

## Trust boundary

`haco-host`はtrustedですが、raw Incus daemon socket、`/var/lib/incus`、Physical HostのHacocoon state directoryは渡しません。

通常のEnvironmentにはHacocoon control endpoint自体を渡しません。

```text
ordinary Environment       X---- haco-control deviceなし
trusted haco-host          -----> Hacocoon controller UDS
Physical Host controller   -----> Incus authority
```

専用Incus proxy `haco-control`はexactなtrusted-host ownership markerを確認した後だけreconcileします。既存deviceやclient endpoint configが想定外ならsilent overwriteせずfail closedします。

## Physical Host endpoint

Controllerの既定local endpointは次です。

```text
/run/hacocoon/control.sock
```

Supported WSL bootstrapでは`haco-controller`をPhysical Hostのsystemd serviceとして常駐させます。trusted Hostのprovision前にcontrol socketが `root:hacocoon`、mode `0660` であることを検証します。このlocal groupへの所属はcontroller authorityを与えます。以下のtrusted-host側投影socketは `root:root`、mode `0600` のままです。

localhost TCP listenerは不要です。将来remote transportが本当に必要になった場合だけ、同じclient boundaryの別実装として追加します。

Development/testでは`HACO_CONTROL_SOCKET`でlocal pathを上書きできます。ただしroot authorityでtrusted-hostをreconcileする経路では、inherited environmentによって任意のHost socketへredirectされないよう固定のPhysical Host endpointを使います。

既存pathが安全なstale socketだと証明できない場合はfail closedします。

## Trusted `haco-host` endpoint

Trusted instanceには`haco-control`という1本だけのIncus `proxy` deviceを付与します。

```text
type=proxy
bind=instance
listen=unix:/var/lib/hacocoon-control.sock
connect=unix:/run/hacocoon/control.sock
mode=0600
uid=0
gid=0
```

さらにinstance configとして次を設定します。

```text
environment.HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock
environment.HACO_CLIENT_MODE=controller
```

Instance側socketを`/run`配下に置かないのは意図的です。Guest systemdはboot時にruntime tmpfsをmountするため、guest boot orderingから独立して存在させたいproxy listenerはstableな`/var/lib` pathに置きます。

`hacoq host ensure`はownership markerを検証し、endpoint shapeを完全一致でreconcileし、必要ならinstanceをstartし、`/usr/local/bin/haco-host`と同じreleaseのgeneral `/usr/local/bin/haco`の両方をprovisionします。各client binaryはSHA-256で検証し、Physical Host側sourceはinvoking effective UID所有のregular executableかつgroup/other writableでないことを要求します。Install後は`0755 root:root`へ収束させます。

`HACO_CLIENT_MODE=controller`はauthorization credentialではなく、意図的なsafety / execution-context markerです。移行用 `hacoq` はこのmarkerでguest-local stateの構築を防ぐ。reset後の製品 `haco` はそのlocal composition経路を持たない。Authorizationとpolicyは引き続きcontroller側がauthorityです。

Supported WSL bootstrapはその後、実際のtrusted instance内で`haco-host doctor`を実行します。Physical Host controllerへのround tripが成功しない場合、normal userのautomatic login shellを変更する前にbootstrapを失敗させます。

## Host診断

Status: **implemented**。このcommandのpackaged受入は実装statusで別途追跡する。配布controller binaryには製品clientと同じversion・commit・build日時を埋め込む。Windows gateは両方の実行場所でbuild識別子全体を照合し、開発用の既定値や古いcontrollerをpackaged受入の成功としない。

`haco doctor` と `haco doctor --json` は、Physical Hostとtrusted `haco-host` 内で同じ `system.doctor` controller methodを使う。help/versionは引き続き単独で動作する。応答はcontrollerのbuild・protocolと、順序を固定した5項目を返す。

| Check | 確認する内容 |
|---|---|
| runtime | Incus APIの利用可否とtrustedな管理アクセス |
| storage | 設定対象Btrfs poolと設定上のmount policy |
| trusted_host | 所有hostの稼働、明示root/NIC、profile継承なし、限定controller endpointとclient mode |
| trusted_network | 所有bridgeのDNS・DHCP・NAT・routing・firewall設定 |
| trusted_connectivity | 検証済みtrusted hostからのIPv4 DNS、default route、固定公開対象github.comへのHTTPS |

検査はcontrollerのprovider adapterが実行する。clientは `hacoq` / Incusを起動せず、guest-local stateを作らない。RPCはpath・command・通信先・修復optionを受け取らない。host作成・起動、storage初期化、NIC/firewall調整、service状態変更は行わない。hostが停止していればfailedとなり、host/networkの所有権・設定が不一致なら疎通検査をskipする。

結果は `ok`・`failed`・`skipped`。全項目成功だけが終了0で、failed/skippedがあればreportを出して終了1、不正な使い方は終了2。transport/protocol失敗は終了1で、成功を示すJSON reportを出さない。項目欠落・重複・不明値・不正応答を拒否する。summaryは長さを制限した固定の検査条件で、backend/guestの生出力・errorをreportへコピーしない。失敗は共有loggerでstderrへ記録し、stdoutはtext/JSON結果に使う。

provider probeは各5秒、server operationは30秒、CLIは35秒を上限とする。割込み・cancelでclient connectionを閉じる。自動修復や権限を上げるfallbackはしない。固定対象への外部GETにHost credentialやcaller入力を渡さない。guest probeは継承環境変数を消去し、curlのuser設定を無効にする。対話shellや `.curlrc` のcredential/proxy optionは取り込まない。

成功reportはその時点の基盤検査である。storage設定の一致は実圧縮・COW・live mountの証明ではない。trusted-host疎通はEnvironmentのproxy-only egress、SSH、Workspace保持、将来のfirewall再読込・起動順変更の受入ではない。保持している `haco-host doctor` は引き続きpingだけの移行用診断である。

## Protocol boundary

各connectionの先頭にはversionedかつsize-boundedなJSON envelopeを置きます。Requestはmethodと、成功後にbidirectional streamへ遷移するかを指定します。

Protocol mismatchは明示的なerrorとし、direct Incus accessへfallbackしません。Controllerはaccepted connection数もboundedにします。

現在のtyped Environment APIは次を含みます。

- create
- list
- status
- bounded exec
- interactive shell stream
- delete
- controller ping / doctor diagnostics

Client-only `haco-host` と移行用に残る `hacoq env ...` はdirect Incus authorityを持たず、このAPIを利用する。これらの保持は、reset後の製品 `haco` での提供を意味しない。

## General `haco` client namespace

製品 `haco` はWSL Physical Hostとtrusted `haco-host` 内で共通の利用者入口となる。help/versionは単独で動作し、`doctor` とWSL login aliasはcontrollerを直接呼ぶ。`hacoq` へ処理を委譲せず、未提供の `haco host ensure`・`haco host shell` も明示的に失敗する。

旧Environment namespaceは一時的な `hacoq` に残り、typed controller APIは再利用できる。新CLIのlifecycle実装はこのAPIを使い、guest-local compositionやIncus authorityを持たない。現在のinstallerはbootstrap provisionのため `hacoq host ensure` を直接呼ぶ。この移行依存は新CLIとは別で、撤去予定として残る。

## `haco-host` transition surface

Package済みclient-only binaryは現在次を提供します。

```text
haco-host env list
haco-host env create --workspace <path> <environment>
haco-host env status <environment>
haco-host env exec <environment> -- <command...>
haco-host env shell <environment>
haco-host env delete <environment>
haco-host doctor
```

`haco-host env ...`はmigration中のsurfaceとして有用ですが、通常のEnvironment lifecycleはgeneral `haco` UXへ移します。Long-termの`haco-host` commandはtrusted tooling、credential broker、OCI/runtime administration、Windows/WSL integrationなど、trusted logical Host自体がexecution domainであるoperationへ寄せます。

`env create --workspace`は現時点ではcontroller-side Workspace path contractを維持します。Repository ownershipやWorkspace path resolutionをlogical Host側へ完全移行することは別architecture workです。

## Streaming

Stream handshakeでは可能な検証をsuccess acknowledgementより前に行い、その後同じUnix-domain transport上でbidirectional bytesを流します。

現在はinteractive Environment shellに利用し、client half-closeも維持します。今後のframingでは次を追加できます。

- streamed non-interactive stdin/stdout/stderrとexit metadata
- PTY resize/control event
- Environment TCP forwarding
- その他のbounded controller-mediated stream

`Session`を新しいpublic domain conceptにはしません。StreamはExecutionまたはclient connectionのimplementation detailです。

## Performance

BaselineはUnix domain socket上の通常のGo buffered forwardingです。Local callでもcontroller hopを残し、authorityの一元化を優先します。

巨大fixtureをcommitせず測れるopt-in 100 GiB-class benchmarkがあります。FD passing、`splice(2)`、buffer poolingなどはprofilingで価値が確認できた場合だけ追加します。

## 現在のacceptance

Repository testとreal Incus acceptanceでは次を確認します。

- TCPなしのlocal UDS request/response
- bounded envelope / connection concurrency
- explicit protocol error / cancellation
- half-close behavior
- controller経由のtyped Environment lifecycle call
- interactive shell streaming
- trusted `haco-host` ownership reconciliation
- exact `haco-control` proxy reconciliationとmismatch refusal
- `haco-host`とgeneral `haco` binary provisioningのdigest / idempotency検証
- explicit controller-client modeと想定外mode driftの拒否
- 実trusted instanceの`haco-host doctor`からPhysical Host controllerへのround trip
- stopped/restarted trusted Hostでのcontroller再疎通
- production provision済み`haco env`からcreate/list/status/exec/deleteをPhysical Host controller経由で実行できること
- trusted client modeではhistorical Environment aliasもcontrollerへ強制routeされること
- 未移行commandがguest-local composition初期化前にfail closedすること
- general client pathでguest commandのexit status/stdout/stderrが保持されること
- raw Incus control socket非露出
- 通常Environmentにtrusted controller endpointとclient-mode markerが存在しないこと

今後のfollow-up:

- 残る`haco` commandをclassifyし、適切なものをcontroller client interfaceへ移行
- replacementが確立したcompatibility aliasをremoveまたは明示deprecate
- trusted Host-local toolingをlong-termの`haco-host` namespaceへ移行
- stdout/stderr/exit metadataを持つstreamed Execution framing
- PTY resize/control framing
- generic Environment forwarding
- 実需が出た場合のみremote transport
- profilingで必要性が示された場合のみFD passing / zero-copy
