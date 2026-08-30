# Hacocoon Design Principles

[**日本語**](DESIGN_PRINCIPLES.ja.md) | English

Status: authoritative cross-cutting design principles.

These principles describe what Hacocoon is trying to preserve as the implementation and the set of Environment backends evolve. They are product and architecture constraints, not claims that every planned capability is already implemented.

## 1. Hacocoon manages Environments, not one runtime

Hacocoon is a system for creating and operating isolated development Environments. It is not an Incus wrapper.

Incus system containers are the first concrete backend because they provide a useful balance of Linux fidelity, systemd support, startup speed, storage efficiency, and operational simplicity. They must not become a permanent Core dependency.

Future Environment backends may include, when justified:

- Incus containers;
- Incus VMs;
- microVMs;
- Kubernetes or other scheduler-backed environments;
- remote/SSH-backed hosts;
- other local or remote isolation technologies.

Core should express Environment lifecycle and required capabilities. Backend-specific mechanics stay behind the Environment boundary.

## 2. Untrusted inside, trusted boundary outside

Commands, developer tools, build systems, dependencies, and coding agents inside an Environment are untrusted with respect to host authority.

The Hacocoon host/control plane is trusted to enforce the boundary. Privileged credentials, runtime control, policy decisions, and host-side capabilities stay outside the Environment unless explicitly mediated.

The goal is not to stop the agent from changing its own Environment. The goal is to stop ordinary Environment authority from silently becoming host authority.

## 3. Give the agent freedom inside the Environment

Hacocoon intentionally favors a useful, high-fidelity development machine over a heavily restricted application sandbox.

A backend may allow the agent to be `root` inside its Environment and to:

- install packages;
- run systemd services;
- compile arbitrary code;
- modify the Environment filesystem;
- run container/developer tooling when the selected Base or optional plugin provides it;
- consume CPU, memory, processes, and disk up to configured resource limits.

Environment-local `root` is not equivalent to host authority. Backends must preserve that distinction.

## 4. Minimize authority crossing the boundary

Do not solve developer convenience by exposing ambient host authority to the Environment.

By default, do not expose:

- the host HOME;
- `~/.ssh`, `~/.aws`, cloud credentials, or GitHub tokens;
- Incus, Docker, containerd, or other host control sockets;
- Hacocoon control state;
- arbitrary host filesystem paths;
- unrestricted privileged devices or runtime configuration.

Operations that require host or external-service authority should cross an explicit Policy / Approval / Capability boundary and use the minimum authority necessary.

## 5. The default security target is practical containment, not a VM-equivalent claim

Hacocoon should clearly state the security boundary it actually provides.

For an Incus system-container backend, the host kernel, Incus daemon, and trusted Hacocoon host process are part of the trusted computing base. Hacocoon does not claim to defend against a successful Linux-kernel exploit, Incus/container escape, or compromise of that trusted host control plane.

This is an intentional tradeoff: the default backend may prefer fast startup, low memory overhead, cheap cloning, and high Linux compatibility over separate-kernel isolation.

Stronger backends such as VMs or microVMs may provide a stronger isolation boundary without changing Hacocoon's Core semantics.

## 6. Isolation strength belongs to the backend

Core must not encode assumptions such as "every Environment shares the host kernel" or "every Environment is a VM".

A backend should report the capabilities and guarantees it can actually provide. Hacocoon should reject an operation when a requested guarantee cannot be satisfied rather than pretending all backends are equivalent.

Backend selection may therefore be driven by policy or user intent, for example choosing a lightweight container for normal development and a stronger VM/microVM backend for higher-risk workloads.

Do not spread backend-name conditionals through Core. Prefer stable capability-oriented boundaries once multiple real implementations justify them.

## 7. Workspaces are working data, not a protected vault

A writable Workspace is intentionally writable by the agent. If an Environment has read-write access, the agent may modify or delete files in that Workspace.

Hacocoon's containment goal is to limit the blast radius beyond the explicitly selected Workspace and granted capabilities. Protecting the Workspace from the agent is a different requirement; use read-only access, version control, snapshots, or higher-level review/recovery workflows when needed.

The host must not silently broaden a Workspace mount into access to unrelated host data.

## 8. External authority is brokered, narrow, and auditable

Credentials should remain host-owned whenever practical.

Prefer:

```text
untrusted Environment
       |
       | request
       v
Policy / Approval / Capability
       |
       | narrow authorized operation
       v
host or external service
```

over copying a reusable credential into the Environment.

A capability request should bind the operation to enough identity and state to detect stale approvals, target changes, or confused-deputy behavior before privileged execution.

## 9. Fail closed at trust boundaries

If Hacocoon cannot verify a security-sensitive assumption, it should refuse the privileged operation.

Examples include:

- policy cannot be evaluated;
- required approval is missing or stale;
- runtime/network/profile configuration drifted;
- a requested resource limit cannot be enforced;
- repository or remote identity changed after approval;
- cleanup failed and safe state cannot be proven.

Convenience failures may degrade gracefully. Trust-boundary failures must not silently become broader authority.

## 10. Lightweight and disposable is a feature

Hacocoon should make isolated Environments cheap enough to use routinely, not only for exceptional high-risk work.

Fast creation, low idle cost, copy-on-write storage, reusable immutable Bases/Seeds, and deterministic cleanup are architectural goals because developers and agents are more likely to use isolation when isolation is inexpensive.

Security mechanisms should protect the boundary without unnecessarily preventing the agent from acting like a normal developer inside it.

## 11. Core stays small; integrations stay optional

Core owns the generic Environment lifecycle and the policy/capability boundaries required to operate it safely.

Developer workloads and integrations such as GitHub, containerd, nerdctl, Docker, OCI registries, cloud CLIs, VS Code, or other IDEs do not become Core requirements merely because Hacocoon supports them.

Use adapters/plugins and ordinary package boundaries. Generalize only when a second implementation, a stable test seam, or a concrete replacement need justifies the abstraction.

## 12. Portability is a design constraint

An Environment should remain conceptually usable from different clients and on different backends.

Do not make VS Code, Incus, a specific container runtime, or a specific cloud provider part of the definition of an Environment. Client-specific launch behavior, backend-specific lifecycle mechanics, and workload-specific tooling belong outside the Core domain model.

## Security promise summary

| Layer | Hacocoon expectation |
|---|---|
| Agent / commands inside Environment | untrusted; may be highly privileged inside the Environment |
| Workspace | only explicitly selected data is mounted; writable mode permits destructive edits |
| Host credentials / control sockets | not ambiently exposed to the Environment |
| Privileged external operations | mediated through explicit policy/capability boundaries |
| Environment isolation | provided by the selected backend and its documented guarantees |
| Host kernel / trusted runtime daemon / Hacocoon host process | trusted computing base |
| Kernel exploit / container escape defense | not guaranteed by the container backend; use a stronger backend when required |

The intended default is therefore:

> Give the agent broad freedom inside a cheap Environment, keep host authority outside it, and let the backend define how strong the isolation boundary is.
