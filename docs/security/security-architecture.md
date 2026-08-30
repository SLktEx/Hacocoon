# Security Architecture

Status: authoritative cross-cutting security baseline.

See also [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md) for the broader product and backend-isolation principles.

## Trust model

Commands, developer tools, and coding agents executed inside a Hacocoon Environment are **untrusted with respect to host authority**.

The host-side Hacocoon control path owns environment lifecycle, policy decisions, capability mediation, and privileged credentials. Convenience must not collapse that boundary.

```text
untrusted workspace process
          |
          | request operation
          v
     Hacocoon boundary
      /      |       \
 policy   approval   capability
      \      |       /
       privileged host/service action
```

## Trusted computing base and non-goals

Hacocoon does not promise that every Environment backend has VM-equivalent isolation.

For the Incus system-container backend, the following are part of the trusted computing base:

- the host Linux kernel;
- the Incus daemon/runtime control plane;
- the trusted Hacocoon host process and its policy/capability state.

A successful kernel exploit, Incus/container escape, or compromise of the trusted Hacocoon host control plane is outside the containment guarantee of that backend. This limitation is intentional and must be documented rather than hidden behind a generic "sandbox" claim.

Backends with a stronger isolation model, such as a VM or microVM, may reduce this shared-kernel trust without changing Core semantics. Isolation strength is a backend guarantee.

## Environment-local root is allowed

Hacocoon may deliberately give the coding agent `root` inside an Environment when the selected backend supports that model safely.

The security objective is not to prevent the agent from administering or destroying its own Environment. The objective is to prevent Environment-local authority from silently becoming host authority.

Therefore Environment-local root must not imply ambient access to host credentials, host control sockets, unrelated host filesystems, unrestricted devices, or privileged runtime configuration.

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

## Fail closed

Failure to evaluate policy, obtain required approval, acquire a scoped credential, or verify a backend security guarantee must deny the privileged operation. Cleanup failures must be surfaced rather than hidden.
