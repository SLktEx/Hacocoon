# Controller client transport

[**日本語**](controller-client-transport.ja.md) | English

Status: **partial**. The repository implements the local Unix-domain-socket transport, protocol/version boundary, Physical Host controller executable, typed Environment lifecycle calls, interactive Environment streaming, a client-only `haco-host` CLI, and the first real trusted-`haco-host` control-channel provisioning path. Migrating Physical-Host-authority `haco` operations, PTY resize framing, Environment port forwarding, and any remote transport remain follow-up work.

## Summary

Hacocoon clients ask the trusted Physical Host controller to perform Environment and Host-authority operations instead of receiving direct Incus authority.

For local communication, the default transport is a Unix domain socket:

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

The transport is an implementation mechanism. Policy, approval, state, privileged backend access, and operation ownership remain controller responsibilities.

## Goals

- Keep one trusted controller boundary for local client operations.
- Avoid giving clients the Incus control socket or direct Physical Host storage authority.
- Use Unix domain sockets for same-Host communication instead of requiring localhost TCP.
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

The controller is part of the trusted Physical Host control path. A client connection is not authority by itself; exposed methods still preserve the operation's policy, approval, ownership, and lifecycle rules.

Normal Environments are untrusted with respect to Host authority and must not receive the Hacocoon control socket as ambient filesystem state. `haco-host` is the separately managed trusted logical Host and receives only a narrow Hacocoon-owned endpoint.

On the local Incus backend the current provisioning path is:

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

The proxy does not mount the Physical Host socket, `/run/incus`, `/var/lib/incus`, or a broad Physical Host directory into `haco-host`. Ordinary Environments do not receive this proxy device.

The trusted-host reconciler verifies the `haco-host` ownership marker before provisioning. If an existing `haco-control` proxy differs in security-relevant fields (`listen`, `connect`, `bind`, `uid`, `gid`, or `mode`), reconciliation fails closed instead of silently taking it over.

## Local Unix-domain-socket transport

The default Physical Host endpoint is:

```text
/run/hacocoon/control.sock
```

Trusted operational/testing paths may override it. The controller creates the socket owner-only (`0600`) by default. Broader access must be an explicit deployment decision with an authorization model; it must not arise from a permissive default.

The local path intentionally does not create a localhost TCP listener merely to resemble a possible future remote transport. A future transport may implement the same client interface separately.

Startup fails closed when the configured endpoint is already active or when an existing filesystem entry cannot be safely identified as a stale Unix socket. A regular file at the configured path is never removed as stale control state.

## Protocol boundary

The current protocol starts each connection with a versioned JSON envelope. A request identifies a method and whether it transitions into a raw bidirectional stream.

Control envelopes are size-bounded. Bulk data belongs on the post-handshake stream instead of unbounded JSON metadata. The controller also bounds concurrent accepted connections so a client cannot create an unbounded goroutine count through the control endpoint.

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

Validation that can be completed before opening the stream happens before the success acknowledgement. Runtime failures after a stream has started remain part of the streamed operation and require the higher-level operation protocol to represent them.

The current slice uses this mechanism for an interactive Environment shell. Client-side half-close is preserved so EOF on stdin does not automatically discard remaining target output. Later work may layer explicit framing over the stream for:

- non-interactive Execution stdin/stdout/stderr and explicit exit metadata;
- PTY resize/control events;
- local-client-to-Environment TCP forwarding;
- other bounded controller-mediated byte streams.

`Session` is not introduced as a new public domain concept. In Hacocoon terminology these streams carry an **Execution** or a client connection; the stream is a transport implementation detail.

## Incus implementation

The Incus adapter can attach caller-provided streams to an interactive `incus exec` invocation. Incus remains the component that enters the target Environment; Hacocoon does not require an Environment SSH server for this path.

Provider-specific process mechanics stay below the Environment backend boundary. Core/client contracts must not assume that every backend uses the Incus CLI, WebSockets, containers, or a shared kernel.

## `haco-host` client and provisioning

The repository builds and packages a client-only `haco-host` executable with the first everyday Environment command namespace:

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

`haco host ensure` now reconciles both the trusted Incus instance and this client path. It resolves a compatible `haco-host` binary, installs it in the trusted instance as `/usr/local/bin/haco-host`, creates or validates the narrow Incus Unix proxy, and runs `haco-host doctor` inside the instance before reporting success.

The supported WSL bootstrap also installs/enables `haco-controller` as a Physical Host systemd service and verifies that the controller responds before it reconciles `haco-host` or changes the default interactive WSL entry.

The current provisioning intentionally installs only the client-only `haco-host` binary. The existing `haco` executable still contains direct local-composition paths, so installing it in the guest before those Physical-Host-authority operations are migrated to the controller-client interface could target guest-local state accidentally.

`env create --workspace` currently preserves the existing controller-side Workspace-path contract. Moving repository ownership or Workspace path resolution fully into the logical `haco-host` remains separate architecture work.

## Cancellation and cleanup

Client connections are bound to the caller context. Cancelling the client operation closes the local control connection.

The controller closes accepted connections after the handler/stream ends and stops accepting new work when its serving context is cancelled. Higher-level streamed operations must continue to define target-process cancellation and exit/error propagation; a closed transport must not be reported as successful completion when the target outcome is unknown.

Stale socket recovery is conservative. Ambiguous filesystem or listener state fails rather than deleting an entry that cannot be proven to be the stale endpoint observed by the controller.

## Performance

The baseline is ordinary buffered Go forwarding over Unix domain sockets. The controller hop is retained even for local calls because authority centralization is more important than removing a same-Host IPC hop.

The repository includes an opt-in generated 100 GiB-class stream benchmark. It does not require a 100 GiB fixture on disk. FD passing, `splice(2)`, buffer pooling, or other reduced-copy techniques should be implemented only when measurements show a meaningful throughput, CPU, allocation, or latency problem.

## Current repository slice

Implemented:

- versioned controller protocol;
- Unix-domain-socket client/server transport;
- private default socket mode;
- bounded control envelopes and concurrent connections;
- structured controller errors;
- context-bound client connection cleanup and half-close support;
- `haco-controller` executable;
- typed Environment create/list/status/exec/delete calls;
- stable Environment list ordering;
- guest non-zero exec status propagation for bounded unary exec;
- pre-validated interactive Environment shell stream;
- Incus interactive stream bridge;
- client-only `haco-host` Environment CLI and controller diagnostics;
- release packaging for `haco-controller` and `haco-host`;
- trusted `haco-host` client-binary provisioning;
- trusted-host-only Incus Unix proxy to the Physical Host control socket;
- WSL systemd controller service setup and readiness check;
- real Ubuntu 26.04 + Incus + managed-Btrfs acceptance of the trusted-host control path;
- opt-in 100 GiB-class UDS baseline benchmark.

Still planned/follow-up:

- migrate Physical-Host-authority `haco` commands to the controller client interface where appropriate;
- complete the user-facing `haco` / `haco-host` responsibility split;
- explicit streamed Execution framing with stdin/stdout/stderr and exit metadata;
- PTY resize/control framing;
- Environment TCP forwarding through the controller stream;
- a remote transport, only if needed;
- measured FD-passing/zero-copy optimization, only if needed.

## Acceptance

Repository and real-Incus acceptance now demonstrate that:

- ordinary calls work over a Unix socket without a TCP listener;
- Environment create/list/status/exec/delete can be invoked through the client without direct Incus access;
- the trusted `haco-host` instance can reach the Physical Host controller through the dedicated Unix proxy;
- ordinary Environments do not receive the trusted control proxy;
- the trusted instance does not require `/run/incus` or `/var/lib/incus` exposure;
- interactive bytes flow bidirectionally through the stream path;
- stdin half-close keeps the output side readable;
- missing/invalid targets fail before stream success acknowledgement when they can be validated up front;
- cancellation closes client streams;
- control envelopes and connection concurrency are bounded;
- stale/active/non-socket path handling fails safely;
- protocol mismatch is explicit;
- the 100 GiB-class benchmark can exercise the buffered baseline without committing a giant fixture.

The GitHub-hosted acceptance proves the Linux/Incus control-channel mechanism. Real Windows terminal -> WSL default-login behavior, richer interactive PTY behavior, and any future remote-client transport remain separately environment-dependent.
