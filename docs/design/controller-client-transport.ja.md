# Controller client transport

日本語 | [**English**](controller-client-transport.md)

Status: **partial**。ローカル Unix domain socket transport、protocol/version 境界、Physical Host controller executable、typed Environment lifecycle call、interactive Environment stream、client-only `haco-host` CLI、実trusted `haco-host` への最初のcontrol-channel provisioningまで実装済み。Physical Host authority を必要とする `haco` operation のclient化、PTY resize framing、Environment port forwarding、remote transportはfollow-upとする。

## Summary

HacocoonのClientはIncus authorityを直接受け取らず、trusted Physical Host controllerへEnvironment / Host-authority operationを要求する。

ローカル通信の既定transportはUnix domain socketとする。

```text
client (`haco-host`, future adapters)
        |
        | Hacocoon Unix domain socket
        v
Physical Host Hacocoon controller
        |
        | provider/backend boundary
        v
Incus or another Environment backend
```

transportは実装上の仕組みであり、Policy、Approval、state、privileged backend access、operation ownershipのauthorityはcontrollerに残す。

## Goals

- ローカルClient operationに対してtrusted controller boundaryを1つ維持する。
- ClientへIncus control socketやPhysical Host storage authorityを渡さない。
- 同一Host内通信ではlocalhost TCP listenerを必須にせずUnix domain socketを使う。
- 同じclient/controller boundaryで通常のrequest/responseと長時間のbidirectional streamを扱う。
- 必要になった場合だけ別transportを追加できるようcommand semanticsをtransport固有にしない。
- provider boundaryより上をbackend-neutralに保つ。

## Non-goals

この設計では次を必須にしない。

- controller transportとしてのSSH。
- public TCP listener。
- TLS、mTLS、VPN integration、named remote context。
- Incus bridgeへの直接routing。
- `haco-host`や通常EnvironmentへのIncus Unix socket公開。
- profilingで必要性が示される前のFD passing / zero-copy最適化。

## Authority and trust boundaries

controllerはtrusted Physical Host control pathの一部である。Clientがsocketへ接続できたこと自体をoperation authorityとはみなさず、公開methodは各operationのPolicy、Approval、ownership、lifecycle ruleを維持する。

通常EnvironmentはHost authorityに対してuntrustedであり、Hacocoon control socketをambient filesystem stateとして渡さない。`haco-host`は別途管理するtrusted logical Hostで、Hacocoon-ownedの狭いendpointだけを受け取る。

Local Incus backendの現在のprovisioning pathは次の通り。

```text
trusted haco-host instance
  /run/hacocoon/control.sock
          |
          | Incus proxy device
          | unix <-> unix, bind=instance
          v
Physical Host Hacocoon control socket
          |
     haco-controller
          |
          | Incus API/socket stays here
          v
        incusd
```

このproxyはPhysical Host socketそのもの、`/run/incus`、`/var/lib/incus`、広いPhysical Host directoryを`haco-host`へmountしない。通常Environmentにはこのproxy deviceを付与しない。

trusted-host reconcilerはprovisioning前に`haco-host` ownership markerを確認する。既存`haco-control` proxyのsecurity-relevant field（`listen`、`connect`、`bind`、`uid`、`gid`、`mode`）が期待値と違う場合は、暗黙に上書きせずfail closedする。

## Local Unix-domain-socket transport

Physical Host側の既定endpointは次のpath。

```text
/run/hacocoon/control.sock
```

trustedな運用・テスト用途ではpathを上書きできる。controllerは既定socketをowner-only (`0600`) で作成する。より広いaccessはauthorization modelを伴う明示的なdeployment decisionとし、permissive defaultにはしない。

将来remote transportを追加できるようにする目的だけでlocalhost TCP listenerを作らない。remote transportが必要になった場合は同じclient interfaceの別実装として追加する。

configured endpointが既にactiveな場合、または既存filesystem entryをstale Unix socketだと安全に確認できない場合、startupはfail closedする。regular fileをstale control stateとして削除しない。

## Protocol boundary

現在のprotocolは各connection先頭にversioned JSON envelopeを置く。requestはmethodと、成功後にraw bidirectional streamへ遷移するかを指定する。

control envelopeにはsize limitを設ける。bulk dataはunbounded JSON metadataではなくhandshake後のstreamへ流す。controllerはaccepted connection数にも上限を設け、control endpointから無制限にgoroutineを増やせないようにする。

protocol version mismatchは明示的なerrorとし、Incus direct accessへsilent fallbackしない。

現在のtyped Environment APIはcreate、list、status、exec、shell、deleteを公開する。client-only `haco-host` binaryはこれらを利用し、local compositionを初期化せずIncus authorityを直接importしない。

## Control streams

interactive operationやbulk operationにはunary request/response以上の通信が必要になる。そのためclient/controller boundaryはvalidate済みstream handshakeと、その後のbidirectional bytesを扱う。

```text
request envelope
    -> controller validates target/method
    -> success/error response envelope
    -> on success, bidirectional stream
```

stream開始前に検証可能な内容はsuccess acknowledgementより前に検証する。stream開始後のruntime failureはstreamed operationの一部であり、より上位のoperation protocolで表現する必要がある。

現在はinteractive Environment shellにこの仕組みを使う。Client側half-closeを維持し、stdin EOFだけで残りのtarget outputを捨てない。今後stream上に明示的なframingを追加して次を扱える。

- non-interactive Executionのstdin/stdout/stderrと明示的なexit metadata。
- PTY resize/control event。
- local ClientからEnvironmentへのTCP forwarding。
- その他のbounded controller-mediated byte stream。

`Session`を新しいpublic domain conceptにはしない。Hacocoonの用語ではstreamが運ぶのは**Execution**またはClient connectionであり、streamはtransport implementation detailである。

## Incus implementation

Incus adapterはcaller-provided streamをinteractive `incus exec`へ接続できる。target Environmentへ入るresponsibilityはIncusに残り、この経路のためにEnvironment SSH serverを必須にしない。

provider-specificなprocess mechanicsはEnvironment backend boundaryより下へ置く。Core/Client contractは、すべてのbackendがIncus CLI、WebSocket、container、shared kernelを使うと仮定しない。

## `haco-host` clientとprovisioning

リポジトリはclient-only `haco-host` executableをbuild/packageし、最初の日常Environment command namespaceを提供する。

```text
haco-host env list
haco-host env create --workspace <path> <environment>
haco-host env status <environment>
haco-host env exec <environment> -- <command...>
haco-host env shell <environment>
haco-host env delete <environment>
haco-host doctor
```

このbinaryはcontroller client API経由だけで通信する。`composition.Local()`を呼ばず、Incus control socketを必要としない。

`haco host ensure`はtrusted Incus instanceだけでなくclient pathまでreconcileする。compatibleな`haco-host` binaryを解決し、trusted instanceの`/usr/local/bin/haco-host`へinstallし、狭いIncus Unix proxyを作成またはexact validationし、最後にinstance内の`haco-host doctor`が成功することを確認してから成功扱いにする。

supported WSL bootstrapはPhysical Host上で`haco-controller`をsystemd serviceとしてenable/startし、controllerが応答することを確認した後に`haco-host`をreconcileし、最後にdefault interactive WSL entryを変更する。

現在のprovisioningでtrusted instanceへ入れるのはclient-only `haco-host` binaryだけ。既存`haco` executableにはdirect local-composition pathが残るため、Physical Host authority operationのcontroller-client移行前にguestへ入れるとguest-local stateへ誤って向く可能性がある。

`env create --workspace`は現在のcontroller-side Workspace path contractを維持する。repository ownershipやWorkspace path resolutionをlogical `haco-host`側へ完全移行することは別architecture workである。

## Cancellation and cleanup

Client connectionはcaller contextに紐付ける。Client operationがcancelされた場合、local control connectionを閉じる。

controllerはhandler/stream終了後にaccepted connectionを閉じ、serve contextがcancelされた場合は新しいworkの受付を停止する。より上位のstreamed operationはtarget process cancellationとexit/error propagationを引き続き定義する必要があり、transport closeをtarget outcome不明のままsuccessとして扱わない。

stale socket recoveryはconservativeに行う。filesystem/listener stateが曖昧な場合、controllerが確認したstale endpointだと証明できないentryを削除せず失敗する。

## Performance

baselineはUnix domain socket上の通常のGo buffered forwardingとする。local callでもcontroller hopは維持し、同一Host IPC hopを1つ削ることよりauthority集約を優先する。

リポジトリにはopt-in generated 100 GiB-class stream benchmarkを含める。100 GiB fixtureをdiskに保存する必要はない。FD passing、`splice(2)`、buffer poolingなどはthroughput、CPU、allocation、latencyの問題が計測で示された場合だけ実装する。

## Current repository slice

実装済み:

- versioned controller protocol。
- Unix-domain-socket client/server transport。
- private default socket mode。
- bounded control envelopeとconcurrent connection。
- structured controller error。
- context-bound client connection cleanupとhalf-close support。
- `haco-controller` executable。
- typed Environment create/list/status/exec/delete call。
- stable Environment list ordering。
- bounded unary execでguestのnon-zero exit statusを保持。
- pre-validated interactive Environment shell stream。
- Incus interactive stream bridge。
- client-only `haco-host` Environment CLIとcontroller diagnostics。
- `haco-controller` / `haco-host`のrelease packaging。
- trusted `haco-host`へのclient binary provisioning。
- trusted-host専用Incus Unix proxy。
- WSL bootstrapでのPhysical Host controller systemd serviceとreadiness check。
- real Ubuntu 26.04 + Incus + managed Btrfsでのtrusted-host control path acceptance。
- opt-in 100 GiB-class UDS baseline benchmark。

planned/follow-up:

- 必要なPhysical Host authority `haco` commandをcontroller client interfaceへ移行。
- user-facingな`haco` / `haco-host` responsibility splitを完了。
- stdin/stdout/stderrとexit metadataを持つ明示的なstreamed Execution framing。
- PTY resize/control framing。
- controller streamを通したEnvironment TCP forwarding。
- 必要になった場合のみremote transport。
- 計測で必要と分かった場合のみFD passing / zero-copy最適化。

## Acceptance

Repository testとreal Incus acceptanceでは次を確認する。

- TCP listenerなしで通常callがUnix socket上で動く。
- direct Incus accessなしでclientからEnvironment create/list/status/exec/deleteを呼べる。
- trusted `haco-host` instanceから専用Unix proxy経由でPhysical Host controllerへ到達できる。
- 通常Environmentにはtrusted control proxyが付かない。
- trusted instanceに`/run/incus`や`/var/lib/incus`を公開しない。
- interactive bytesがstream pathを双方向に流れる。
- stdin half-close後もoutput sideをreadできる。
- missing/invalid targetを事前検証できる場合、stream success acknowledgementより前に失敗する。
- cancellationでClient streamが閉じる。
- control envelopeとconnection concurrencyがboundedである。
- stale/active/non-socket pathを安全に扱う。
- protocol mismatchが明示的である。
- 巨大fixtureをcommitせず100 GiB-class benchmarkでbuffered baselineを測れる。

GitHub-hosted acceptanceでLinux/Incus上のcontrol-channel mechanismは実証する。実Windows terminalからWSL default loginへ入る挙動、より完全なinteractive PTY behavior、将来のremote Client transportは引き続き別のenvironment-dependent acceptanceとする。
