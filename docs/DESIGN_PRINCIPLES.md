# Hacocoon design principles

[日本語](DESIGN_PRINCIPLES.ja.md) | English

These are product constraints, not a claim that every possible backend or planned feature is implemented.

## Incus-first development runtime

Hacocoon adds development conveniences, distinct data lifetimes and explicit security boundaries to Incus. Use native Incus capabilities and keep Core small. Interfaces should separate real ownership and testing responsibilities; hypothetical backends must not constrain today's implementation or create extra recovery machinery.

Fast creation, low idle cost, immutable Bases and copy-on-write storage make isolation practical for everyday work. These are goals; physical sharing and performance require measurement.

## Freedom inside an untrusted Environment

Treat commands, dependencies, build tools and coding agents as untrusted with respect to Host authority. They may install packages, run services, compile code and modify their own filesystem, including as Environment-local root where the backend safely permits it. Configured resource limits still apply. See [resource limits](design/sandbox-resource-limits.md) for the currently exposed controls.

A writable Workspace is working data, not a vault protected from the agent. The workload may change or delete its files. Use read-only access, Git, explicit snapshots and review when protection is needed; never expand a selected Workspace into unrelated Host data.

## Data outlives execution

Environments are disposable. Ordinary deletion preserves Workspace/Git, retained OCI data and saved snapshots. Unsaved runtime state need not be recoverable. Recreate uses current retained data; snapshot restore uses saved data. The exact rules belong to [data lifetime](guides/data-lifetime.md) and [snapshots](design/environment-snapshots.md).

Ownership identity and positive cleanup confirmation remain mandatory even for disposable execution. A stopped Environment still holds its leases. Ambiguous cleanup retains ownership and reports recovery-required.

## Authority stays outside

Host credentials, HOME, SSH/cloud configuration, runtime/control sockets, protected Hacocoon state and unrelated paths are not ambient Environment capabilities. Windows drives and process interoperability likewise belong only to deliberately trusted infrastructure.

```text
untrusted Environment
  -> narrow request
  -> Policy / Approval / Capability
  -> authorized Host or external-service operation
```

Bind approval to the actual target, Environment identity and relevant state. Recheck before privileged execution to reject stale approvals, changed repositories/remotes and confused-deputy requests. Keep reusable credentials on the trusted side instead of copying them to workloads.

## Fail closed at trust boundaries

Refuse an operation when required Policy, approval, ownership, network configuration or resource enforcement cannot be verified. Convenience features may fail independently; security failures must not silently broaden authority. Never interpret attempted cleanup as confirmed absence.

The [security architecture](security/security-architecture.md) owns detailed trust boundaries; [ADR 0002](adr/0002-environment-lifecycle-ownership.md) explains lifecycle ownership.

## State the actual isolation guarantee

The current Incus system-container backend shares the Host kernel. The kernel, Incus daemon and Hacocoon control plane are trusted. Hacocoon does not guarantee defense against a successful kernel exploit, container escape or compromise of that control plane.

VM/microVM isolation is a possible future backend direction, not a current selection. A backend must state its guarantees and refuse unmet requirements. Platform differences do not permit silently dropping an isolation or anti-spoofing invariant.

## Separate contracts from implementations

Core owns lifecycle, Execution and Policy/Approval/Capability semantics. Standard supplies replaceable defaults such as Incus and the implemented egress proxy. Plugins supply optional workload integrations. Detailed classification and package boundaries live in [adapter architecture](design/plugin-architecture.md).

Outbound authority is a Core concern; HTTP/HTTPS enforcement and provider network mechanics are implementation responsibilities. Git and AWS operations express their narrower authority through specialized Capabilities.

Clients own editor launch, private keys and presentation. Orchestrators own tasks, models and budgets. Neither IDE nor orchestrator concepts become Core dependencies. Optional Docker, nerdctl, Git or cloud tooling remains optional even when a maintained profile provides it.
