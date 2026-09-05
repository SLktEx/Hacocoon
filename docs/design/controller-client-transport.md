# Controller client transport

[**日本語**](controller-client-transport.ja.md) | English

Status: **partial**. The local Unix-domain-socket protocol, Physical Host controller, trusted-`haco-host` endpoint projection, client-only `haco-host` CLI, guarded general `haco` provisioning, typed Environment lifecycle calls, the first interactive stream, and the first general `haco env ...` controller-client namespace are implemented. Migrating the remaining `haco` operations, PTY control framing, Environment port forwarding, and any remote transport remain follow-up work.

## Summary

Current reset-CLI boundary: product `haco` exposes help/version and its WSL login alias; older `haco env ...` descriptions on this page refer to the retained migration CLI. See [implementation status](../IMPLEMENTATION_STATUS.md).

WSL may open the login shell before the enabled controller service has bound its socket. The login alias waits up to 30 seconds using read-only ping calls, retrying only transport unavailability. Protocol/operation rejection is not retried; the client never starts another controller or changes service state. This startup timeout does not limit the interactive session's lifetime.

Interactive completion must not wait for the user to close local stdin after the remote shell exits. The Incus adapter supplies a dedicated OS stdin pipe to the child process and owns its closure, while the controller closes the client connection after publishing process completion. Output is drained and the actual exit status is preserved. A Windows acceptance run found the previous socket-reader assignment blocked process completion after `exit`; component tests keep client input open through both successful and nonzero process exit. The WSL login alias also requires actual terminal file descriptors, so a character device such as `/dev/null` cannot start a trusted-host shell.

Hacocoon clients ask the trusted Physical Host controller to perform Environment and Host-authority operations instead of receiving direct Incus authority.

The local path is:

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

Inside trusted `haco-host`, the client endpoint is projected as:

```text
trusted haco-host
  |
  | /var/lib/hacocoon-control.sock
  | Incus proxy device: haco-control
  v
Physical Host /run/hacocoon/control.sock
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

The supported WSL bootstrap runs `haco-controller` as a Physical Host systemd service. Its control socket is verified as `root:hacocoon`, mode `0660`, before the trusted Host is provisioned. Membership in this local group grants controller authority. The projected trusted-host socket below remains `root:root`, mode `0600`.

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
environment.HACO_CLIENT_MODE=controller
```

The instance-side path intentionally lives outside `/run`: guest systemd commonly mounts runtime tmpfs state during boot, so a proxy listener that must exist independently of guest boot ordering uses a stable `/var/lib` path.

`haco host ensure` verifies the trusted-host ownership marker, reconciles the exact endpoint shape, starts the instance when needed, and provisions both `/usr/local/bin/haco-host` and the same-release general `/usr/local/bin/haco`. Provisioning is digest-checked and requires each Physical Host source binary to be an executable regular file owned by the invoking effective UID and not writable by group/other users. The installed binaries must converge to `0755 root:root`.

`HACO_CLIENT_MODE=controller` is deliberately a safety/execution-context marker, not an authorization credential. When the full `haco` binary runs inside trusted `haco-host`, the marker prevents still-unmigrated commands from silently constructing guest-local Hacocoon state. Authorization and policy remain controller-side.

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

Both the client-only `haco-host` executable and the controller-backed `haco env ...` namespace use this API without direct Incus authority.

## General `haco` client namespace

`haco` is the general Hacocoon client. The first migrated namespace is:

```text
haco env list
haco env create --workspace <path> <environment>
haco env status <environment>
haco env exec <environment> -- <command...>
haco env shell <environment>
haco env delete <environment>
```

These commands are intentionally dispatched **before** `composition.Local()` is initialized. A `haco env ...` invocation therefore cannot silently become a guest-local Incus operation when the same binary is executed inside trusted `haco-host`.

The command either reaches the configured Hacocoon controller endpoint or fails through the controller-client transport. It does not fall back to direct local composition.

The historical flat commands such as `haco create`, `haco status`, `haco exec`, `haco shell`, and `haco delete` remain temporary compatibility/local paths on the Physical Host during migration. Inside trusted `haco-host`, controller-client mode forces those Environment aliases through the same controller client instead of letting them reach local composition.

Any other still-unmigrated `haco` command is rejected in controller-client mode with an explicit fail-closed error. This makes permanent installation of the general binary safe without pretending that every historical namespace has already been migrated. New docs, automation, and integrations should use `haco env ...` rather than the compatibility aliases.

The general `/usr/local/bin/haco` binary is now provisioned by `haco host ensure` beside `/usr/local/bin/haco-host`. The trusted instance receives an explicit controller socket and client-mode marker; it still receives no raw Incus authority or Physical Host state tree.

## `haco-host` transition surface

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

The `haco-host env ...` surface remains useful during migration, but ordinary Environment lifecycle belongs to general `haco` UX. Long-term `haco-host` commands should focus on operations whose execution domain is the trusted logical Host itself, such as trusted tooling, credential brokering, OCI/runtime administration, and Windows/WSL integration.

`env create --workspace` still uses the controller-side Workspace path contract. Moving repository ownership or Workspace path resolution fully into the logical Host is separate architecture work.

## Streaming

The stream handshake validates the request before acknowledging success where possible, then carries bidirectional bytes over the same Unix-domain transport.

The current implementation uses it for interactive Environment shell traffic and preserves client half-close semantics. Future framing may add:

- streamed non-interactive stdin/stdout/stderr plus exit metadata;
- PTY resize/control events;
- Environment TCP forwarding;
- other bounded controller-mediated streams.

`Session` is not introduced as a new public domain concept; the stream is an implementation detail for an Execution or client connection.

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
- `haco-host` and general `haco` binary provisioning with digest/idempotency checks;
- explicit controller-client mode and refusal of unexpected mode drift;
- real trusted-instance `haco-host doctor` round trip to the Physical Host controller;
- stopped/restarted trusted Host regaining controller access;
- production-provisioned `haco env` create/list/status/exec/delete from inside the real trusted Host through the Physical Host controller;
- historical Environment aliases being forced through the controller in trusted client mode;
- still-unmigrated commands failing before guest-local composition is initialized;
- guest command exit-status/stdout/stderr propagation through the general client path;
- absence of raw Incus control-socket exposure;
- absence of the trusted controller endpoint and client-mode marker on ordinary Environments.

Still planned:

- classify and migrate the remaining appropriate `haco` commands onto the controller client interface;
- remove or explicitly deprecate compatibility aliases once their replacements are established;
- move trusted Host-local tooling into the long-term `haco-host` namespaces;
- streamed Execution framing with explicit stdout/stderr/exit metadata;
- PTY resize/control framing;
- generic Environment forwarding;
- remote transport only if a real use case requires it;
- FD passing/zero-copy only if profiling demonstrates a worthwhile benefit.
