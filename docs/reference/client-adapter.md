# Reusable Client Adapter Contract

Hacocoon exposes a client-neutral adapter API through `github.com/SLktEx/Hacocoon/pkg/clientadapter`.

This contract is for IDEs, browser/code-server clients, CLI tools, JetBrains adapters, and future clients that need to create or reuse an Environment and connect to it without depending on VS Code-specific behavior.

The package is a **client integration boundary**, not a new UI and not an authorization bypass. Policy/capability approval remains in Hacocoon's trusted authority path, while interaction events remain read-only observations through `pkg/interaction`.

## Public operations

| Operation | Purpose |
| --- | --- |
| `NewController` / `NewControllerAt` | Open lifecycle and connection operations through the configured / explicit controller socket |
| `NewLocal` | Initialize local composition and an interaction reader on the Physical Host |
| `Ensure` | Reuse an exact Environment/Workspace/access-mode match or create it |
| `Status` | Inspect client-safe Environment state |
| `Connections` | Reconcile current client connections from Hacocoon/runtime state |
| `PrepareSSH` | Install a client-supplied **public** key and create a persistent SSH grant with a portless stream target |
| `Forward` | Create a loopback-only TCP forwarding connection |
| `Revoke` | Revoke one managed SSH/forward connection |
| `Delete` | Delete the Environment and its Hacocoon lifecycle state |
| `InteractionBatch` | Read minimized, resumable `pkg/interaction` events |

`NewController() *Adapter` selects the configured endpoint without opening a
connection. `NewControllerAt(path string) (*Adapter, error)` also validates an
explicit, nonempty socket path. Connection and protocol failures are reported
by the requested operation. Callers of `NewController` no longer receive an
unused constructor error.

Every Environment returned to an adapter reports the in-guest workspace as:

```text
/workspace
```

The Host source path is returned separately as `source_workspace` for local lifecycle/reuse decisions. Hacocoon does not automatically send that Host path to a remote service.

## Ownership model

### Hacocoon owns

- Environment identity and lifecycle;
- Workspace lease enforcement;
- Incus/provider connection setup and cleanup;
- generation-bound SSH streams and loopback-only explicit forwarding;
- the managed SSH **public-key** marker installed in the Environment;
- reconnectable connection metadata;
- trusted Policy/Capability approval/execution;
- the read-only interaction event source.

### The client owns

- the SSH private key;
- IDE/project configuration;
- the client process and launch behavior;
- UI, notifications, and Browser Notification permission;
- persistence of its own interaction cursor/event IDs when cross-session deduplication is desired.

A private key is not accepted by `pkg/clientadapter`. `PrepareSSH` accepts only public-key text. The client uses the corresponding private key directly when it uses ProxyCommand with the returned target.

## Fail-closed reuse

`Ensure` may reuse an existing Environment only when both of these match exactly:

1. the canonical Host Workspace path;
2. the requested read-only/read-write access mode.

An Environment with a different Workspace or different authority is not silently repurposed. The adapter returns `ErrAlreadyExists` instead.

If creation succeeds but post-create verification fails, the adapter attempts to remove the new Environment. Ambiguous cleanup is surfaced as `ErrRecoveryRequired`.

## Connection security

The adapter verifies that SSH has no Host address/port and carries a valid stream target. Explicit TCP forwarding remains loopback-only.

For SSH, the adapter additionally requires:

- connection kind `ssh`;
- target port `22`;
- no Host address or allocated port, and a durable target whose Token method
  encodes the ProxyCommand argument.

For TCP forwarding it requires the expected target port. If a newly-created connection violates the contract, the adapter revokes it; if revocation cannot be proven, recovery is required.

This keeps an adapter from accidentally accepting a provider drift that broadens a local-only connection to a LAN/WAN listener.

## Reconnect and process restart

A client process does not own Hacocoon's connection truth. After restart it can call:

1. `Status(environment)`;
2. `Connections(environment)`;
3. `InteractionBatch(lastOffset, ...)`.

Incus-backed connection reconciliation reads persistent SSH grants and explicit forwarding proxy metadata, so a reconnecting client does not need an in-memory VS Code session to discover the current endpoint. The client can then reuse or explicitly revoke the existing connection.

## Generic non-VS-Code proof

The controller-backed CLI can explicitly select an external path on the Physical Host. For ordinary managed Workspace use, follow [getting started](../guides/getting-started.md).

```sh
haco env create --no-oci --workspace "$PWD" demo
haco ssh setup demo
ssh haco-demo
```

Inspect/reconnect after restarting the client shell or another adapter process:

```sh
haco env status --json demo
```

Revoke only the client connection:

```sh
haco env disconnect demo <grant-id>
```

Or delete the Environment when its lifecycle is finished:

```sh
haco env delete demo
```

The private key is consumed by the ordinary `ssh` client, not Hacocoon.

## code-server and other IDEs

code-server, JetBrains remote tooling, or another IDE can be treated as ordinary software inside the Environment plus a client-owned launch/connection adapter. Hacocoon does not need a `code-server`, `jetbrains`, or `vscode` conditional in Core.

For a web workload, a client may prepare a loopback forwarding connection to the workload port. Browser exposure, authentication, URL handling, and UI remain client responsibilities.

## Interaction events

`InteractionBatch` returns the public `pkg/interaction` contract introduced for client-neutral notifications. Reading those events is side-effect free and never approves or executes a capability.

See [`INTERACTION_EVENTS.md`](interaction-events.md) for event minimization, resume cursors, and Browser Notification mapping.

## Public compatibility boundary

`pkg/clientadapter` exported signatures use package-owned DTOs and public error sentinels rather than `internal/core` types. Provider/runtime and IDE-specific details remain implementation details behind the adapter boundary.

Ordinary clients, including those inside trusted `haco-host`, use the controller
constructors. `NewLocal` requires Physical Host authority to initialize local
services. `InteractionBatch` is available from `NewLocal`; the controller
constructors currently expose only lifecycle and connection operations.

An empty explicit controller endpoint returns `ErrInvalidArgument`. Caller
cancellation and deadline expiry preserve `context.Canceled` and
`context.DeadlineExceeded` for `errors.Is`; a missing or refused endpoint returns
`ErrUnavailable`.

This is a pre-1.0 contract. Breaking changes are still possible, but client-specific branching should be added in the client adapter, not Hacocoon Core.

## Public host-key pinning

Status: implemented for Incus SSH preparation. `PrepareSSH` returns
`host_public_key` containing the validated public server identity obtained
through the provider channel. Comments and arbitrary guest output are excluded;
malformed key data fails preparation and revokes the managed connection.
A failed cleanup remains recovery-required. Private host keys are never read.
The adapter validates the key again before exposing it to clients. Other
providers and connection-list reconciliation may omit it; clients must obtain
trusted identity before installing or changing a pin. This does not authorize
silently replacing an existing known-host key. Automated SSH setup is implemented; clients retain ownership of private keys and local configuration.

## Portless SSH target

`PrepareSSH` no longer accepts a Host port. Its portless target binds the
Environment generation, Workspace and grant; `StreamTarget.Token()` produces
the argument consumed by `haco stream`. Generic forwarding keeps its separate
loopback-port contract. See [interactive access](../design/client-and-interactive-access.md)
for exact identity validation and cold-start behavior.
