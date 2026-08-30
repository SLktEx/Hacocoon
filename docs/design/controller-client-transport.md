# Controller client transport

[**日本語**](controller-client-transport.ja.md) | English

Status: **partial**. The repository contains the local Unix-domain-socket transport, protocol/version boundary, controller executable, typed Environment lifecycle calls, the first interactive Environment stream, and a client-only `haco-host` CLI for ordinary Environment operations. Provisioning a narrow Physical Host control endpoint into the trusted `haco-host` instance, migrating Physical-Host-authority `haco` operations, PTY resize framing, Environment port forwarding, and any remote transport remain follow-up work.

## Summary

Hacocoon clients must ask the trusted Host control path to perform Environment and Host-authority operations instead of receiving direct Incus authority.

For local communication, the default transport is a Unix domain socket:

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

The transport is an implementation mechanism. The controller remains the authority for policy, approval, state, privileged backend access, and operation ownership.

## Goals

- Keep one trusted controller boundary for local client operations.
- Avoid giving clients the Incus control socket or direct Host storage authority.
- Use Unix domain sockets for same-Host communication instead of requiring a localhost TCP listener.
- Support ordinary request/response operations and long-lived bidirectional streams through the same client/controller boundary.
- Keep command semantics independent from a specific transport so another transport can be added later if a real use case requires it.
- Preserve backend neutrality above the provider boundary.

## Non-goals

This design does not require:

- SSH as the controller transport;
- a public TCP listener;
- TLS, mTLS, VPN integration, or named remote contexts;
- direct routing to an Incus bridge;
- exposing the Incus Unix socket to `haco-host` or an Environment;
- FD passing or zero-copy optimization before profiling justifies it.

## Authority and trust boundaries

The controller is part of the trusted Host control path. A client connection is not authority by itself; exposed methods still have to preserve the operation's policy, approval, ownership, and lifecycle rules.

Normal Environments are untrusted with respect to Host authority and must not receive the Hacocoon control socket as ambient filesystem state. `haco-host` is the separately managed trusted logical Host. The repository now has a client-only `haco-host` executable, but the real trusted instance does not yet receive the Physical Host control endpoint. That provisioning must be explicit and must not broaden socket access to ordinary Environments.

The raw Incus socket remains controller/provider-side:

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

A controller-mediated shell therefore does not require an SSH daemon, an Environment IP reachable by the client, or direct client access to the Incus bridge.

## Local Unix-domain-socket transport

The default local endpoint is conceptually:

```text
/run/hacocoon/control.sock
```

The exact path may be overridden for trusted operational/testing purposes. The default socket is created with owner-only permissions (`0600`) in the current repository slice. Broader access must be an explicit deployment decision with an authorization model; it must not arise from a permissive default.

The local path intentionally does not create a localhost TCP listener only to resemble a possible future remote transport. A future transport may implement the same client interface separately.

Startup fails closed when the configured endpoint is already active or when an existing filesystem entry cannot be safely identified as a stale Unix socket. A regular file at the configured path is never removed as stale control state.

## Protocol boundary

The current protocol starts each connection with a versioned JSON envelope. A request identifies a method and whether it transitions into a raw bidirectional stream.

Control envelopes are size-bounded. Bulk data belongs on the post-handshake stream instead of unbounded JSON metadata.

The controller also bounds concurrent accepted connections so a client cannot create an unbounded goroutine count through the control endpoint.

Protocol-version mismatch is explicit and must not silently fall back to direct Incus access.

The typed Environment API currently exposes create, list, status, exec, shell, and delete. The client-only `haco-host` binary uses those methods and does not initialize local composition or directly import Incus authority.

## Control streams

Interactive and bulk operations require more than unary request/response calls. The client/controller boundary therefore supports a validated stream handshake followed by bidirectional bytes:

```text
request envelope
    -> controller validates target/method
    -> success/error response envelope
    -> on success, bidirectional stream
```

Validation that can be completed before opening the stream should happen before the success acknowledgement. Runtime failures after a stream has started are still part of the streamed operation and require the higher-level operation protocol to represent them.

The current repository slice uses this mechanism for an interactive Environment shell. Client-side half-close is preserved so EOF on stdin does not automatically discard remaining target output. Later work may layer explicit framing over the stream for:

- non-interactive Execution stdin/stdout/stderr and explicit exit metadata;
- PTY resize/control events;
- local-client-to-Environment TCP forwarding;
- other bounded controller-mediated byte streams.

Do not make `Session` a new public domain concept. In Hacocoon terminology, these streams carry an **Execution** or a client connection; the stream is a transport implementation detail.

## Incus implementation

The current Incus adapter can attach caller-provided streams to an interactive `incus exec` invocation. Incus remains the component that enters the target Environment; Hacocoon does not require an Environment SSH server for this path.

Provider-specific process mechanics stay below the Environment backend boundary. Core/client contracts must not assume that every backend uses the Incus CLI, WebSockets, containers, or a shared kernel.

## `haco-host` client slice

The repository now builds and packages a client-only `haco-host` executable with the first everyday Environment command namespace:

```text
haco-host env list
haco-host env create --workspace <path> <environment>
haco-host env status <environment>
haco-host env exec <environment> -- <command...>
haco-host env shell <environment>
haco-host env delete <environment>
haco-host doctor
```

This binary talks only through the controller client API. It does not call `composition.Local()` and does not require the Incus control socket.

The trusted `haco-host` Incus instance lifecycle is implemented separately by the local Incus backend. The remaining integration step is to provision compatible client binaries and a narrow Hacocoon-owned controller endpoint into that trusted instance without exposing the raw Incus socket or the same endpoint to normal Environments.

`env create --workspace` currently preserves the existing controller-side Workspace-path contract. Moving repository ownership or Workspace path resolution fully into the logical `haco-host` is separate architecture work and must not be implied by this transport slice.

## Cancellation and cleanup

Client connections are bound to the caller context. Cancelling the client operation closes the local control connection.

The controller closes accepted connections after the handler/stream ends and stops accepting new work when its serving context is cancelled. Higher-level streamed operations must continue to define target-process cancellation and exit/error propagation; a closed transport must not be reported as successful completion when the target outcome is unknown.

Stale socket recovery is conservative. Ambiguous filesystem or listener state fails rather than deleting an entry that cannot be proven to be the stale endpoint observed by the controller.

## Performance

The baseline is ordinary buffered Go forwarding over Unix domain sockets. The controller hop is retained even for local calls because authority centralization is more important than removing a same-Host IPC hop.

The repository includes an opt-in generated 100 GiB-class stream benchmark. It does not require a 100 GiB fixture on disk. FD passing, `splice(2)`, buffer pooling, or other reduced-copy techniques should be implemented only when measurements show a meaningful throughput, CPU, allocation, or latency problem.

## Current repository slice

Implemented in the current partial slice:

- versioned controller protocol;
- Unix-domain-socket client/server transport;
- private default socket mode;
- bounded control envelopes and concurrent connections;
- structured controller errors;
- context-bound client connection cleanup and half-close support;
- controller executable;
- typed Environment create/list/status/exec/delete calls;
- stable Environment list ordering;
- guest non-zero exec status propagation for bounded unary exec;
- pre-validated interactive Environment shell stream;
- Incus interactive stream bridge;
- client-only `haco-host` Environment CLI and controller diagnostics;
- release packaging for `haco-controller` and `haco-host`;
- opt-in 100 GiB-class UDS baseline benchmark.

Still planned/follow-up:

- provision the Hacocoon control endpoint and client binaries into the real trusted `haco-host` instance;
- migrate Physical-Host-authority `haco` commands to the controller client interface where appropriate;
- explicit streamed Execution framing with stdin/stdout/stderr and exit metadata;
- PTY resize/control framing;
- Environment TCP forwarding through the controller stream;
- a remote transport, only if needed;
- measured FD-passing/zero-copy optimization, only if needed.

## Acceptance

The local transport/client slice is acceptable when repository tests demonstrate that:

- ordinary calls work over a Unix socket without a TCP listener;
- Environment lifecycle operations can be invoked through the typed client without direct Incus access;
- interactive bytes flow bidirectionally through the stream path;
- stdin half-close keeps the output side readable;
- missing/invalid targets fail before a stream success acknowledgement when they can be validated up front;
- cancellation closes client streams;
- control envelopes and connection concurrency are bounded;
- stale/active/non-socket path handling fails safely;
- protocol mismatch is explicit;
- the 100 GiB-class benchmark can exercise the buffered baseline without committing a giant fixture.

Real trusted-`haco-host` control-channel provisioning, real interactive terminal behavior, and any future remote-client acceptance remain environment-dependent and must be tracked separately from repository tests.
