# Controller client transport

日本語 | [**English**](controller-client-transport.md)

Status: **partial**。リポジトリにはローカル Unix domain socket transport、protocol/version 境界、controller executable、typed Environment lifecycle call、最初の interactive Environment stream、通常の Environment 操作用 client-only `haco-host` CLI が実装されている。trusted `haco-host` instance への狭い Physical Host control endpoint のprovisioning、Physical Host authority を必要とする `haco` operation の移行、PTY resize framing、Environment port forwarding、remote transport は follow-up とする。

## Summary

Hacocoon の Client は Incus authority を直接受け取らず、trusted Host control path に Environment / Host-authority operation を要求する。

ローカル通信の既定 transport は Unix domain socket とする。

```text
Client (`haco-host`, future adapters)
        |
        | Hacocoon Unix domain socket
        v
trusted Hacocoon controller
        |
        | provider/backend boundary
        v
Incus or another Environment backend
```

transport は実装上の仕組みであり、Policy、Approval、state、privileged backend access、operation ownership の authority は controller に残る。

## Goals

- ローカル Client operation に対して trusted controller boundary を1つ維持する。
- Client に Incus control socket や Physical Host storage authority を渡さない。
- 同一 Host 内通信では localhost TCP listener を必須にせず Unix domain socket を使う。
- 同じ client/controller boundary で通常の request/response と長時間の bidirectional stream を扱えるようにする。
- 実際に必要になった場合だけ別 transport を追加できるよう、command semantics を transport 固有にしない。
- provider boundary より上を backend-neutral に保つ。

## Non-goals

この設計では次を必須にしない。

- controller transport としての SSH。
- public TCP listener。
- TLS、mTLS、VPN integration、named remote context。
- Incus bridge への直接 routing。
- `haco-host` や通常の Environment への Incus Unix socket 公開。
- profiling で必要性が示される前の FD passing / zero-copy 最適化。

## Authority and trust boundaries

controller は trusted Host control path の一部である。Client が socket に接続したこと自体を operation authority とみなしてはならず、公開する method は各 operation の Policy、Approval、ownership、lifecycle rule を維持する必要がある。

通常の Environment は Host authority に対して untrusted であり、Hacocoon control socket を ambient filesystem state として渡してはならない。`haco-host` は別途管理される trusted logical Host である。リポジトリには client-only `haco-host` executable が実装済みだが、実際の trusted instance にはまだ Physical Host control endpoint を渡していない。そのprovisioningは明示的に行い、通常の Environment へ socket access を広げてはならない。

raw Incus socket は controller/provider 側に残す。

```text
client
  |  Hacocoon control socket
  v
controller
  |  Incus API/socket
  v
incusd
  |
  v
Environment
```

そのため controller-mediated shell は、Environment 側 SSH daemon、Client から到達可能な Environment IP、Client から Incus bridge への直接到達性を必要としない。

## Local Unix-domain-socket transport

既定ローカル endpoint は概念上次の path とする。

```text
/run/hacocoon/control.sock
```

trusted な運用・テスト用途では path を上書きできる。現在の repository slice では既定 socket を owner-only (`0600`) で作成する。より広い access が必要な場合は authorization model を伴う明示的な deployment decision とし、permissive default にしない。

将来 remote transport を追加できるようにする目的だけで localhost TCP listener を作らない。remote transport が必要になった場合は、同じ client interface の別実装として追加する。

configured endpoint が既に active な場合、または既存 filesystem entry が stale Unix socket であると安全に確認できない場合、startup は fail closed する。configured path に regular file が存在する場合、それを stale control state として削除してはならない。

## Protocol boundary

現在の protocol は各 connection の先頭に versioned JSON envelope を置く。request は method と、成功後に raw bidirectional stream へ遷移するかどうかを指定する。

control envelope には size limit を設ける。bulk data は unbounded JSON metadata ではなく handshake 後の stream を流す。

controller は accepted connection 数にも上限を設け、control endpoint から無制限に goroutine を増やせないようにする。

protocol version mismatch は明示的な error とし、Incus direct access へ silent fallback してはならない。

現在の typed Environment API は create、list、status、exec、shell、delete を公開する。client-only `haco-host` binary はこれらを利用し、local composition を初期化せず、Incus authority を直接importしない。

## Control streams

interactive operation や bulk operation には unary request/response 以上の通信が必要になる。そのため client/controller boundary は、validate された stream handshake と、その後の bidirectional bytes を扱う。

```text
request envelope
    -> controller validates target/method
    -> success/error response envelope
    -> on success, bidirectional stream
```

stream 開始前に検証可能な内容は success acknowledgement より前に検証する。stream 開始後の runtime failure は streamed operation の一部であり、より上位の operation protocol で表現する必要がある。

現在の repository slice では interactive Environment shell にこの仕組みを利用する。Client 側の half-close を維持し、stdin EOF だけで残りの target output を捨てない。今後、stream 上に明示的な framing を追加して次を扱える。

- non-interactive Execution の stdin/stdout/stderr と明示的な exit metadata。
- PTY resize/control event。
- local Client から Environment への TCP forwarding。
- その他の bounded controller-mediated byte stream。

`Session` を新しい public domain concept にしない。Hacocoon の用語では、これらの stream が運ぶのは **Execution** または Client connection であり、stream は transport implementation detail である。

## Incus implementation

現在の Incus adapter は caller-provided stream を interactive `incus exec` に接続できる。target Environment に入る responsibility は Incus に残り、この経路のために Environment SSH server を必須にしない。

provider-specific な process mechanics は Environment backend boundary より下に置く。Core/Client contract は、すべての backend が Incus CLI、WebSocket、container、shared kernel を使うと仮定してはならない。

## `haco-host` client slice

リポジトリは client-only `haco-host` executable をbuild/packageし、最初の日常 Environment command namespace を提供する。

```text
haco-host env list
haco-host env create --workspace <path> <environment>
haco-host env status <environment>
haco-host env exec <environment> -- <command...>
haco-host env shell <environment>
haco-host env delete <environment>
haco-host doctor
```

このbinaryは controller client API 経由だけで通信する。`composition.Local()` を呼ばず、Incus control socket を必要としない。

trusted `haco-host` Incus instance lifecycle は local Incus backend 側ですでに別途実装されている。残るintegrationは、compatibleなclient binaryと狭いHacocoon-owned controller endpointをそのtrusted instanceへprovisionし、raw Incus socketや同じendpointを通常のEnvironmentへ露出させないことである。

`env create --workspace` は現在、既存のcontroller-side Workspace path contractを維持する。repository ownershipやWorkspace path resolutionをlogical `haco-host`側へ完全移行することは別architecture workであり、このtransport sliceだけで実現済みとは扱わない。

## Cancellation and cleanup

Client connection は caller context に紐付ける。Client operation が cancel された場合、local control connection を閉じる。

controller は handler/stream 終了後に accepted connection を閉じ、serve context が cancel された場合は新しい work の受付を停止する。より上位の streamed operation は target process cancellation と exit/error propagation を引き続き定義する必要があり、transport close を target outcome 不明のまま success として扱ってはならない。

stale socket recovery は conservative に行う。filesystem/listener state が曖昧な場合、controller が確認した stale endpoint だと証明できない entry を削除せず失敗する。

## Performance

baseline は Unix domain socket 上の通常の Go buffered forwarding とする。local call でも controller hop は維持する。同一 Host IPC hop を1つ削ることより、authority を controller に集約することを優先する。

リポジトリには opt-in の generated 100 GiB-class stream benchmark を含める。100 GiB の fixture を disk に保存する必要はない。FD passing、`splice(2)`、buffer pooling、その他の reduced-copy technique は、throughput、CPU、allocation、latency の問題が計測で示された場合だけ実装する。

## Current repository slice

現在の partial slice で実装済み:

- versioned controller protocol。
- Unix-domain-socket client/server transport。
- private default socket mode。
- bounded control envelope と concurrent connection。
- structured controller error。
- context-bound client connection cleanup と half-close support。
- controller executable。
- typed Environment create/list/status/exec/delete call。
- stable Environment list ordering。
- bounded unary execでguestのnon-zero exit statusを保持。
- pre-validated interactive Environment shell stream。
- Incus interactive stream bridge。
- client-only `haco-host` Environment CLI と controller diagnostics。
- `haco-controller` / `haco-host` のrelease packaging。
- opt-in 100 GiB-class UDS baseline benchmark。

planned/follow-up:

- 実際のtrusted `haco-host` instanceへHacocoon control endpointとclient binaryをprovision。
- 必要なPhysical Host authority `haco` commandをcontroller client interfaceへ移行。
- stdin/stdout/stderrとexit metadataを持つ明示的な streamed Execution framing。
- PTY resize/control framing。
- controller stream を通した Environment TCP forwarding。
- 必要になった場合のみ remote transport。
- 計測で必要と分かった場合のみ FD passing / zero-copy 最適化。

## Acceptance

ローカル transport/client slice は、repository test で次を確認できれば acceptance とする。

- TCP listener なしで通常 call が Unix socket 上で動く。
- direct Incus accessなしでtyped clientからEnvironment lifecycle operationを呼べる。
- interactive bytes が stream path を双方向に流れる。
- stdin half-close後もoutput sideをreadできる。
- missing/invalid target を事前検証できる場合、stream success acknowledgement より前に失敗する。
- cancellation で Client stream が閉じる。
- control envelope と connection concurrency が bounded である。
- stale/active/non-socket path を安全に扱う。
- protocol mismatch が明示的である。
- 巨大 fixture をcommitせず、100 GiB-class benchmark で buffered baseline を測れる。

実trusted `haco-host`へのcontrol-channel provisioning、real interactive terminal behavior、将来のremote Client acceptanceは environment-dependent であり、repository testとは別に追跡する。
