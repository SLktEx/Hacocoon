# Controller client transport

[**日本語**](controller-client-transport.ja.md) | English

Status: **partial**. The local Unix-domain protocol, Physical Host controller, trusted-host endpoint projection, client-only `haco-host`, typed Environment API and interactive streams are implemented. Product `haco` currently exposes help/version, controller-backed `setup` and `doctor`, and its controller-backed WSL login alias. Environment lifecycle in the new product CLI, PTY control framing, port forwarding and remote transport remain planned.

## Summary

Current reset-CLI boundary: product `haco` exposes help/version, controller-backed `setup` and `doctor`, and its WSL login alias; older `haco env ...` descriptions on this page refer to the retained migration CLI. See [implementation status](../IMPLEMENTATION_STATUS.md).

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

`haco setup` verifies the trusted-host ownership marker, reconciles the exact endpoint shape, starts the instance when needed, and provisions both `/usr/local/bin/haco-host` and the same-release general `/usr/local/bin/haco`. Provisioning is digest-checked and requires each Physical Host source binary to be an executable regular file owned by the invoking effective UID and not writable by group/other users. The installed binaries must converge to `0755 root:root`.

`HACO_CLIENT_MODE=controller` is deliberately a safety/execution-context marker, not an authorization credential. The retained `hacoq` migration binary uses this marker to prevent guest-local state construction. The reset product `haco` does not contain that local composition path. Authorization and policy remain controller-side.

The supported WSL bootstrap then executes `haco-host doctor` inside the real trusted instance. Bootstrap fails before changing the user's automatic login shell if the round trip cannot reach the Physical Host controller.

## Host setup

Status: **implemented**; commit-bound packaged and real-Incus acceptance is recorded in [implementation status](../IMPLEMENTATION_STATUS.md).

`haco setup` invokes `system.setup` on the existing Physical Host controller from either client context. It prepares the owned host, storage, network and the two required client binaries. Requests take no parameters; companion paths are resolved next to the running controller executable. Both sources are validated before provider mutation. No legacy CLI, guest controller or caller-selected root command participates.

Only one setup executes at a time. The server bounds it to 15 minutes and the CLI to 16 minutes. Client cancellation closes the connection; the controller may still finish its bounded operation. Another request receives busy until that operation ends. An explicit retry reuses owned resources and verified clients. Failures retain data and never imply permission to reformat or delete. Setup reports resource preparation; the installer separately verifies the controller round trip and connectivity before completion. Use `haco doctor` for read-only inspection.

The controller owns setup failure logging and returns a selected error/next action without raw provider output. The client renders that failure; transport/protocol failures are logged at the client boundary. See [ADR 0006](../adr/0006-controller-owned-host-setup.md).

## Host diagnostics

Status: **implemented**; packaged acceptance of this command is tracked separately in implementation status. The controller release binary carries the same version, commit and build date as the product client. The Windows gate compares their complete build identities in both execution contexts; a development/default or stale controller identity cannot satisfy packaged acceptance.

`haco doctor` and `haco doctor --json` use the same `system.doctor` controller method on the Physical Host and inside trusted `haco-host`. Help/version remain standalone. The doctor response identifies the controller build and protocol and contains five ordered checks:

| Check | What is observed |
|---|---|
| runtime | Incus API availability and trusted management access |
| storage | The configured Btrfs pool and its configured mount policy |
| trusted_host | Running owned host, explicit root/NIC, no inherited profiles, and the narrow controller endpoint/client mode |
| trusted_network | The owned bridge's configured DNS, DHCP, NAT, routing and firewall policy |
| trusted_connectivity | IPv4 DNS, a default route and HTTPS to the fixed public target github.com from the verified trusted host |

The controller's provider adapter performs the checks. The client neither invokes `hacoq`/Incus nor constructs guest-local state. The RPC takes no paths, commands, targets or repair options. It cannot create/start a host, initialize storage, reconcile a NIC/firewall, or change service state. A stopped host is a failed check; connectivity is skipped when host/network ownership or configuration fails.

Results use `ok`, `failed` or `skipped`. Exit 0 requires every check to pass; a failed/skipped check returns a report and exit 1, while usage errors return 2. Transport/protocol failure returns exit 1 with no successful JSON report. Missing, duplicate, unknown or malformed check results are rejected. Diagnostic summaries are bounded fixed predicates; raw backend/guest output and errors are not copied into the report. Failure logging uses the shared logger on stderr, leaving stdout for the text/JSON result.

Cold WSL execution can precede the enabled controller socket. The CLI first waits up to 30 seconds using read-only ping, retrying only transport unavailability. It then requests diagnostics once; protocol/operation rejection and failed checks are not retried. The wait neither starts services nor repairs resources.

Each provider probe is bounded to five seconds, the server operation to 30 seconds and the complete CLI operation to 65 seconds. Interrupt/cancellation closes the client connection. No automatic repair or privileged fallback occurs. The fixed external GET uses no Host credentials or caller input. The guest probe clears inherited environment variables and disables curl's user configuration; credentials/proxy options from the interactive shell or `.curlrc` are not imported.

A successful report is a point-in-time infrastructure check. Configured storage options are not proof of actual compression/COW or live mount behavior. Trusted-host connectivity is not acceptance of Environment proxy-only egress, SSH, Workspace retention, or firewall behavior across future reload/startup orders. The retained `haco-host doctor` is still a ping-only migration diagnostic.

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

The client-only `haco-host` executable and the retained migration CLI `hacoq env ...` use this API without direct Incus authority. These retained commands do not establish support in the reset product `haco`.

## General `haco` client namespace

Product `haco` is the common user entry point on the WSL Physical Host and inside trusted `haco-host`. Its help/version commands are standalone; `setup`, `doctor` and the WSL login alias call the controller directly. It does not delegate to `hacoq`; unimplemented commands, including `haco host ensure` and `haco host shell`, fail explicitly.

The prior Environment namespace remains in temporary `hacoq`, while the typed controller API remains reusable. Rebuilding the product lifecycle commands must use that API without guest-local composition or Incus authority. The installer invokes `haco setup` through the existing controller; the legacy bootstrap orchestration and guest hacoq provisioner have been removed.

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

Repository tests and the maintained real-Incus gate cover the following contracts. The setup/client-only gate passed on `b71f88e`; subsequent product changes need their own acceptance:

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
- production-provisioned `haco-host env` create/list/status/exec/delete from inside the real trusted Host through the Physical Host controller;
- absence of legacy guest `hacoq` after fresh setup;
- component coverage of retained legacy aliases, Base routing and fail-closed local composition;
- guest command exit-status/stdout/stderr propagation through the client-only companion;
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
