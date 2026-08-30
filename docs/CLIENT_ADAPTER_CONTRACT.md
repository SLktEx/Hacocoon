# Reusable Client Adapter Contract

Hacocoon exposes a client-neutral adapter API through `github.com/SLktEx/Hacocoon/pkg/clientadapter`.

This contract is for IDEs, browser/code-server clients, CLI tools, JetBrains adapters, and future clients that need to create or reuse an Environment and connect to it without depending on VS Code-specific behavior.

The package is a **client integration boundary**, not a new UI and not an authorization bypass. Policy/capability approval remains in Hacocoon's trusted authority path, while interaction events remain read-only observations through `pkg/interaction`.

## Public operations

| Operation | Purpose |
| --- | --- |
| `NewLocal` | Open the adapter against the local Hacocoon Host |
| `Ensure` | Reuse an exact Environment/Workspace/access-mode match or create it |
| `Status` | Inspect client-safe Environment state |
| `Connections` | Reconcile current client connections from Hacocoon/runtime state |
| `PrepareSSH` | Install a client-supplied **public** key and create loopback-only SSH access |
| `Forward` | Create a loopback-only TCP forwarding connection |
| `Revoke` | Revoke one managed SSH/forward connection |
| `Delete` | Delete the Environment and its Hacocoon lifecycle state |
| `InteractionBatch` | Read minimized, resumable `pkg/interaction` events |

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
- loopback-only proxy enforcement;
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

A private key is not accepted by `pkg/clientadapter`. `PrepareSSH` accepts only public-key text. The client uses the corresponding private key directly when it connects to the loopback endpoint.

## Fail-closed reuse

`Ensure` may reuse an existing Environment only when both of these match exactly:

1. the canonical Host Workspace path;
2. the requested read-only/read-write access mode.

An Environment with a different Workspace or different authority is not silently repurposed. The adapter returns `ErrAlreadyExists` instead.

If creation succeeds but post-create verification fails, the adapter attempts to remove the new Environment. Ambiguous cleanup is surfaced as `ErrRecoveryRequired`.

## Connection security

The underlying provider already enforces loopback-only managed proxies. `pkg/clientadapter` performs a second projection-time check and refuses any returned/reconciled connection whose Host is not a loopback address.

For SSH, the adapter additionally requires:

- connection kind `ssh`;
- target port `22`;
- valid loopback Host and host port.

For TCP forwarding it requires the expected target port. If a newly-created connection violates the contract, the adapter revokes it; if revocation cannot be proven, recovery is required.

This keeps an adapter from accidentally accepting a provider drift that broadens a local-only connection to a LAN/WAN listener.

## Reconnect and process restart

A client process does not own Hacocoon's connection truth. After restart it can call:

1. `Status(environment)`;
2. `Connections(environment)`;
3. `InteractionBatch(lastOffset, ...)`.

Incus-backed connection reconciliation reconstructs managed proxy metadata, so a reconnecting client does not need an in-memory VS Code session to discover the current endpoint. The client can then reuse or explicitly revoke the existing connection.

## Generic non-VS-Code proof

The ordinary `haco` CLI already exercises the same generic client boundary. No VS Code extension or VS Code protocol is needed:

```sh
haco create --workspace "$PWD" demo
haco ssh demo --public-key "$HOME/.ssh/id_ed25519.pub" --host-port 2222
ssh -i "$HOME/.ssh/id_ed25519" -p 2222 root@127.0.0.1
```

Inspect/reconnect after restarting the client shell or another adapter process:

```sh
haco status demo --json
haco connections demo --json
```

Revoke only the client connection:

```sh
haco unforward demo ssh-2222
```

Or delete the Environment when its lifecycle is finished:

```sh
haco delete demo
```

The private key is consumed by the ordinary `ssh` client, not Hacocoon.

## code-server and other IDEs

code-server, JetBrains remote tooling, or another IDE can be treated as ordinary software inside the Environment plus a client-owned launch/connection adapter. Hacocoon does not need a `code-server`, `jetbrains`, or `vscode` conditional in Core.

For a web workload, a client may prepare a loopback forwarding connection to the workload port. Browser exposure, authentication, URL handling, and UI remain client responsibilities.

## Interaction events

`InteractionBatch` returns the public `pkg/interaction` contract introduced for client-neutral notifications. Reading those events is side-effect free and never approves or executes a capability.

See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md) for event minimization, resume cursors, and Browser Notification mapping.

## Public compatibility boundary

`pkg/clientadapter` exported signatures use package-owned DTOs and public error sentinels rather than `internal/core` types. Provider/runtime and IDE-specific details remain implementation details behind the adapter boundary.

This is a pre-1.0 contract. Breaking changes are still possible, but client-specific branching should be added in the client adapter, not Hacocoon Core.
