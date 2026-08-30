# Controller client transport

[**日本語**](controller-client-transport.ja.md) | English

Status: **partial**. The local Unix-domain-socket protocol, Physical Host controller, trusted-`haco-host` endpoint projection, client-only `haco-host` CLI, typed Environment lifecycle calls, and the first interactive stream are implemented. Migrating the remaining Physical-Host-authority `haco` operations, PTY control framing, Environment port forwarding, and any remote transport remain follow-up work.

## Summary

Hacocoon clients ask the trusted Physical Host controller to perform Environment and Host-authority operations instead of receiving direct Incus authority.

The local path is:

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

The extra local hop is intentional. Policy, approval, authoritative state, logging, and provider authority stay on the controller side.

## Trust boundary

`haco-host` is trusted, but it still does **not** receive the raw Incus daemon socket, `/var/lib/incus`, or the Physical Host Hacocoon state directory.

Normal Environments do not receive the Hacocoon control endpoint at all.

```text
ordinary Environment       X---- no haco-control device
trusted haco-host          -----> Hacocoon controller UDS
Physical Host controller   -----> Incus authority
```

The dedicated `haco-control` Incus proxy is reconciled only after the exact trusted-host ownership marker has been verified. An unexpected existing device or client endpoint configuration is rejected rather than silently replaced.

## Physical Host endpoint

The controller listens locally at:

```text
/run/hacocoon/control.sock
```

The supported WSL bootstrap runs `haco-controller` as a Physical Host systemd service. Its runtime directory is private and the control socket is verified as root-owned mode `0600` before the trusted Host is provisioned.

The controller does not require a localhost TCP listener. A future remote transport, if one is genuinely needed, should implement the same client boundary separately.

Development and tests may override the local path with `HACO_CONTROL_SOCKET`. Root-authority trusted-host reconciliation deliberately uses the fixed Physical Host endpoint instead of trusting an inherited override.

Startup and stale-socket handling fail closed when an existing path cannot be proven safe to reuse.

## Trusted `haco-host` endpoint

The trusted instance receives a single Incus `proxy` device named `haco-control` with the intended shape:

```text
type=proxy
bind=instance
listen=unix:/var/lib/hacocoon-control.sock
connect=unix:/run/hacocoon/control.sock
mode=0600
uid=0
gid=0
```

The instance also receives:

```text
environment.HACO_CONTROL_SOCKET=/var/lib/hacocoon-control.sock
```

The instance-side path intentionally lives outside `/run`: guest systemd commonly mounts runtime tmpfs state during boot, so a proxy listener that must exist independently of guest boot ordering uses a stable `/var/lib` path.

`haco host ensure` verifies the trusted-host ownership marker, reconciles the exact endpoint shape, starts the instance when needed, and provisions the client-only `/usr/local/bin/haco-host` binary. Provisioning is digest-checked and requires the source binary to be an executable regular file owned by the invoking effective UID and not writable by group/other users.

The supported WSL bootstrap then executes `haco-host doctor` inside the real trusted instance. Bootstrap fails before changing the user's automatic login shell if the round trip cannot reach the Physical Host controller.

## Protocol boundary

Each connection starts with a versioned, size-bounded JSON envelope. Requests identify a method and whether the connection transitions into a bidirectional stream.

Protocol mismatch is explicit and never falls back to direct Incus access. The controller also bounds concurrently accepted connections.

The typed Environment API currently includes:

- create;
- list;
- status;
- bounded exec;
- interactive shell stream;
- delete;
- controller ping/doctor diagnostics.

The client-only `haco-host` executable uses this API and does not initialize `composition.Local()`.

## Streaming

The stream handshake validates the request before acknowledging success where possible, then carries bidirectional bytes over the same Unix-domain transport.

The current implementation uses it for interactive Environment shell traffic and preserves client half-close semantics. Future framing may add:

- streamed non-interactive stdin/stdout/stderr plus exit metadata;
- PTY resize/control events;
- Environment TCP forwarding;
- other bounded controller-mediated streams.

`Session` is not introduced as a new public domain concept; the stream is an implementation detail for an Execution or client connection.

## `haco-host` client commands

The packaged client-only binary currently exposes:

```text
haco-host env list
haco-host env create --workspace <path> <environment>
haco-host env status <environment>
haco-host env exec <environment> -- <command...>
haco-host env shell <environment>
haco-host env delete <environment>
haco-host doctor
```

`env create --workspace` still uses the controller-side Workspace path contract. Moving repository ownership or Workspace path resolution fully into the logical Host is separate architecture work.

## Performance

The baseline is ordinary buffered Go forwarding over Unix domain sockets. The controller hop is retained even for local calls because centralized authority is more valuable than eliminating one local IPC hop.

An opt-in generated 100 GiB-class benchmark exists to measure the baseline without committing a giant fixture. FD passing, `splice(2)`, buffer pooling, or other reduced-copy techniques should be added only when measurements justify them.

## Current acceptance

Repository and real-Incus acceptance cover:

- local request/response over UDS without TCP;
- bounded envelopes and connection concurrency;
- explicit protocol errors and cancellation;
- half-close behavior;
- typed Environment lifecycle calls through the controller;
- interactive shell streaming;
- trusted `haco-host` ownership reconciliation;
- exact `haco-control` proxy reconciliation and mismatch refusal;
- client binary provisioning and idempotency;
- real trusted-instance `haco-host doctor` round trip to the Physical Host controller;
- stopped/restarted trusted Host regaining controller access;
- absence of raw Incus control-socket exposure;
- absence of the trusted controller endpoint on ordinary Environments.

Still planned:

- move the appropriate Physical-Host-authority `haco` commands onto the controller client interface;
- streamed Execution framing with explicit stdout/stderr/exit metadata;
- PTY resize/control framing;
- generic Environment forwarding;
- remote transport only if a real use case requires it;
- FD passing/zero-copy only if profiling demonstrates a worthwhile benefit.
