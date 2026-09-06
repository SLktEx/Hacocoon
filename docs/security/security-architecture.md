# Security Architecture

Status: authoritative cross-cutting security baseline.

See also [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md) for the broader product and backend-isolation principles and [`../design/trusted-host.md`](../design/trusted-host.md) for the current `haco-host` slice.

## Trust model

Commands, developer tools, and coding agents executed inside a Hacocoon Environment are **untrusted with respect to host authority**.

The trusted Hacocoon control path owns environment lifecycle, policy decisions, capability mediation, and privileged credentials. On local Incus deployments this path now distinguishes the **Physical Host** from the persistent trusted logical **`haco-host`**. Convenience must not collapse either boundary into an ordinary Environment.

```text
untrusted workspace process
          |
          | request operation
          v
     Hacocoon boundary
      /      |       \
 policy   approval   capability
      \      |       /
       trusted host/service action
```

## Trusted computing base and non-goals

The managed-repository WSL workflow mounts its upstream repository only in
trusted `haco-host` and an independent Incus volume copy in the Environment.
Authenticated Git reads only registered trusted metadata. A per-Environment
Git-only Unix proxy is bound to one registered repository; it exposes no
controller methods, Host shell, credentials or Incus socket. Untrusted packs
are validated before approval. Policy and approval bind the registered upstream,
ref, old/new OIDs and operation; execution uses those immutable values and an
exact remote-ref lease. See [ADR 0008](../adr/0008-managed-repository-workspaces.md).

Hacocoon does not promise that every Environment backend has VM-equivalent isolation.

For the Incus system-container backend, the following are part of the trusted computing base:

- the Physical Host Linux kernel;
- the Incus daemon/runtime control plane;
- the trusted Hacocoon Physical Host process and its policy/capability state;
- the persistent `haco-host` instance when that trusted logical Host is provisioned.

`haco-host` is therefore not a sandbox. Compromise of it may compromise Hacocoon-managed credentials, external-service authority, or other trusted capabilities as those features move into it. On WSL, future Windows interop or Windows filesystem mounts can extend that authority outside the Linux/Incus boundary and must remain restricted to trusted infrastructure.

A successful kernel exploit, Incus/container escape, compromise of the Physical Host Hacocoon control plane, or compromise of `haco-host` is outside the containment guarantee of the Incus backend. This limitation is intentional and must be documented rather than hidden behind a generic "sandbox" claim.

Backends with a stronger isolation model, such as a VM or microVM, may reduce this shared-kernel trust without changing Core semantics. Isolation strength is a backend guarantee.

## `haco-host` does not get raw Incus authority

Being trusted does not mean every control primitive should be mounted into `haco-host`.

The local Incus implementation keeps the daemon socket, `/var/lib/incus`, authoritative Hacocoon state and Policy on the Physical Host. Product `haco setup` and the WSL login entry call the existing Physical Host controller. Only that controller constructs the privileged provider composition. The trusted `haco-host` receives a narrow root-only Hacocoon endpoint and client binaries, never an Incus socket or second controller. See [ADR 0006](../adr/0006-controller-owned-host-setup.md).

If an existing Incus instance already occupies the literal `haco-host` name, Hacocoon reuses it only when the Hacocoon ownership marker matches exactly. Otherwise reconciliation fails closed rather than taking over an unrelated instance. The ordinary Environment name `host` is reserved by the Incus adapter because it would collide with that provider-local infrastructure name.

On the supported WSL path, root performs installation while the ordinary managed account keeps UID/GID 1000 and a locked password. Normal entry uses controller group access to the `root:hacocoon` socket with mode `0660`; it does not use sudo or grant `incus-admin` by default. Membership in that group grants privileged controller authority. The installer creates no sudo policy. Physical Host root remains the explicit recovery path. See [ADR 0004](../adr/0004-wsl-installer-authority.md).

## Environment-local root is allowed

Hacocoon may deliberately give the coding agent `root` inside an Environment when the selected backend supports that model safely.

The security objective is not to prevent the agent from administering or destroying its own Environment. The objective is to prevent Environment-local authority from silently becoming host authority.

Therefore Environment-local root must not imply ambient access to host credentials, host control sockets, unrelated host filesystems, unrestricted devices, `haco-host`, or privileged runtime configuration.

## v0.1 security baseline

v0.1 must at minimum:

- mount only the requested workspace rather than the host HOME;
- avoid mounting `~/.ssh`, `~/.aws`, GitHub tokens, Incus control sockets, or Hacocoon state into the Environment;
- make cleanup explicit and testable;
- keep Incus lifecycle authority outside the Environment;
- report command exit status without silently elevating privileges.

v0.1 does **not** need the full Policy/Capability engine.

## Workspace blast radius

A read-write Workspace is intentionally writable by the Environment. Hacocoon does not promise to protect writable Workspace contents from the coding agent itself.

Containment means that the agent's ordinary authority should stop at the explicitly selected Workspace, Environment resources, and explicitly granted capabilities. Use read-only access, version control, snapshots, or higher-level recovery/review mechanisms when the Workspace itself must be protected from destructive edits.

The long-term repository/Workspace location is intentionally not part of the `haco-host` trust definition. A local implementation may prefer `haco-host` storage for convenience while WSL integration evolves, but Core must not assume that every repository permanently lives there.

## Capability model (v0.4+)

Privileged external operations are represented as requests rather than ambient credentials.

```text
CapabilityRequest
    -> PolicyEvaluator
       -> allow
       -> deny
       -> require-approval
```

`require-approval` is a first-class policy result, not a UI afterthought.

## Human-in-the-loop split

Hacocoon owns **security approval**:

- credential issuance;
- GitHub push or other privileged repository operation;
- AWS/API privilege use;
- sensitive port exposure;
- runtime privilege changes;
- policy exceptions.

Development approval belongs above Hacocoon: task scope, code review, PR acceptance, and merge decisions can be handled by a human, GitHub, Daintree, Rookery, or another orchestrator.

## Credentials

Long-lived parent credentials should not become files or environment variables broadly readable by the executed agent.

Where possible, later capability adapters should use provider-native short-lived credentials or a brokered operation with narrow scope, short lifetime, and audit records.

Moving a credential-using operation into trusted `haco-host` does not make credential handling automatically safe. Reusable credentials still must not be copied into ordinary Environments, Seeds, logs, or broad workspace state.

## Fail closed

Failure to evaluate policy, obtain required approval, acquire a scoped credential, verify trusted-host ownership, or verify a backend security guarantee must deny the privileged operation. Cleanup failures must be surfaced rather than hidden.
