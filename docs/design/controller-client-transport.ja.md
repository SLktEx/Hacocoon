# Controller client transport

日本語 | [**English**](controller-client-transport.md)

Status: **partial**。Local Unix domain socket protocol、Physical Host controller、trusted `haco-host` への専用endpoint投影、client-only `haco-host` CLI、typed Environment lifecycle call、最初のinteractive streamは実装済みです。残るPhysical-Host-authority `haco` operationのcontroller client化、PTY control framing、Environment port forwarding、remote transportはfollow-upです。

## 概要

Hacocoon Client は raw Incus authority を直接受け取らず、trusted Physical Host controller に Environment / Host-authority operation を要求します。

Local path は次です。

```text
trusted haco-host
  |
  | /var/lib/hacocoon-control.sock
  | Incus proxy device: haco-control
  v
Physical Host /run/hacocoon/control.sock
  |
  v
haco-controller
  |
  | provider/backend boundary
  v
Incus or another Environment backend
```

Local IPC hopを1つ増やすことは意図的です。Policy、Approval、authoritative state、logging、provider authorityはcontroller側に集約します。

## Trust boundary

`haco-host` はtrustedですが、raw Incus daemon socket、`/var/lib/incus`、Physical HostのHacocoon state directoryは渡しません。

通常のEnvironmentにはHacocoon control endpoint自体を渡しません。

```text
ordinary Environment       X---- haco-control deviceなし
trusted haco-host          -----> Hacocoon controller UDS
Physical Host controller   -----> Incus authority
```

専用Incus proxy `haco-control` は exact な trusted-host ownership marker を確認した後だけreconcileします。既存deviceやclient endpoint configが想定外ならsilent overwriteせずfail closedします。

## Physical Host endpoint

Controllerの既定local endpointは次です。

```text
/run/hacocoon/control.sock
```

Supported WSL bootstrapでは`haco-controller`をPhysical Hostのsystemd serviceとして常駐させます。Runtime directoryはprivateにし、trusted Hostをprovisionする前にcontrol socketがroot-owned mode `0600`であることを検証します。

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
```

Instance側socketを`/run`配下に置かないのは意図的です。Guest systemdはboot時にruntime tmpfsをmountするため、guest boot orderingから独立して存在させたいproxy listenerはstableな`/var/lib` pathに置きます。

`haco host ensure`はownership markerを検証し、endpoint shapeを完全一致でreconcileし、必要ならinstanceをstartし、client-only `/usr/local/bin/haco-host` binaryをprovisionします。Client binaryはSHA-256で検証し、sourceはinvoking effective UID所有のregular executableかつgroup/other writableでないことを要求します。

Supported WSL bootstrapはその後、実際のtrusted instance内で`haco-host doctor`を実行します。Physical Host controllerへのround tripが成功しない場合、normal userのautomatic login shellを変更する前にbootstrapを失敗させます。

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

Client-only `haco-host` executableはこのAPIだけを使い、`composition.Local()`を初期化しません。

## Streaming

Stream handshakeでは可能な検証をsuccess acknowledgementより前に行い、その後同じUnix-domain transport上でbidirectional bytesを流します。

現在はinteractive Environment shellに利用し、client half-closeも維持します。今後のframingでは次を追加できます。

- streamed non-interactive stdin/stdout/stderr と exit metadata
- PTY resize/control event
- Environment TCP forwarding
- その他のbounded controller-mediated stream

`Session`を新しいpublic domain conceptにはしません。StreamはExecutionまたはclient connectionのimplementation detailです。

## `haco-host` client commands

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

`env create --workspace`は現時点ではcontroller-side Workspace path contractを維持します。Repository ownershipやWorkspace path resolutionをlogical Host側へ完全移行することは別architecture workです。

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
- client binary provisioningとidempotency
- 実trusted instanceの`haco-host doctor`からPhysical Host controllerへのround trip
- stopped/restarted trusted Hostでのcontroller再疎通
- raw Incus control socket非露出
- 通常Environmentにtrusted controller endpointが存在しないこと

今後のfollow-up:

- 適切なPhysical-Host-authority `haco` commandをcontroller client interfaceへ移行
- stdout/stderr/exit metadataを持つstreamed Execution framing
- PTY resize/control framing
- generic Environment forwarding
- 実需が出た場合のみremote transport
- profilingで必要性が示された場合のみFD passing / zero-copy
