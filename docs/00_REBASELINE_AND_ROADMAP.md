# Architecture & Roadmap

> **Architecture baseline · Updated 2026-08-30**
>
> Hacocoon is a **Secure Workspace Runtime**.  
> For what is actually present on `main`, use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).  
> For authoritative milestone numbering, use [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md).

Hacocoon runs developer tools and coding agents inside isolated Environments while keeping host, GitHub, cloud, network, and other privileged authority behind explicit trusted boundaries.

> [!IMPORTANT]
> Hacocoon is **pre-1.0**. Roadmap milestones describe product gates, not API stability, production support, or proof that every real-host acceptance test has passed.

## Project status at a glance

| Track | Status |
|---|---|
| Implemented milestones | **v0.1 → v0.12** contiguous on `main` |
| Next milestone | **v0.13 Local OCI Registry** — planned, not implemented |
| v0.13 companion | **OCI Seed & Btrfs/COW optimization** — planned second slice |
| Default local runtime | Incus |
| Remote runtime | Deferred; current implementation registers Incus only |
| Current code reality | [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) |
| Version numbering | [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) |

## Product boundary

```text
VS Code / Shell / coding agents / orchestrators / other clients
                              |
                     optional Client Adapter
                              |
                          Workspace
                              v
                    +-------------------+
                    |     Hacocoon      |
                    | Secure Workspace  |
                    |      Runtime      |
                    +---------+---------+
                              |
              +---------------+----------------+
              |                                |
         Environment                    Policy/Capability
              |                                |
      Environment provider              trusted authority
              |                         /       |       \
        runtime.incus               GitHub   External   Host
        current runtime
```

### Hacocoon owns

- Workspace resolution and canonical identity;
- isolated Environment lifecycle and cleanup;
- command and interactive execution;
- Workspace lease and ownership safety;
- client-access primitives such as loopback forwarding and SSH preparation;
- policy evaluation, security approval, and audit;
- scoped privileged capabilities;
- trusted per-session Environment binding;
- provider-neutral Base identity and ResourceBudget contracts;
- recovery behavior for authority-sensitive operations.

### Hacocoon does not own

- the IDE/editor or AI chat UI;
- model selection, model routing, or token budgets;
- task decomposition, retry strategy, or agent DAGs;
- Git branch strategy or worktree orchestration as a Core concern;
- development review/merge workflow;
- provider-native Incus, cloud, Btrfs, OCI, or client protocol details inside Core.

Thin adapters may integrate those systems without redefining Core.

## Architecture invariants

These rules are more important than preserving accidental pre-1.0 behavior.

1. **Core stays provider-neutral.** Incus, cloud providers, VS Code, AHP, Btrfs, OCI, and similar technologies remain behind adapters.
2. **The coding Environment is not the control plane.** Coding agents do not gain Hacocoon/Incus administrator authority merely because an Environment was allocated.
3. **Credentials stay on the trusted side.** Host credentials and reusable upstream credentials are not exported into arbitrary Environments.
4. **Privileged operations are brokered.** Git push and external-service authority cross Policy/Capability boundaries.
5. **Ownership is proven, not inferred.** Deterministic names alone are not sufficient proof for adopting or deleting agent-bound Environments.
6. **Mutable names resolve to immutable identity.** Base aliases and future OCI/Seed references must pin immutable revisions/digests where durable identity matters.
7. **Explicit limits fail closed.** A provider that cannot enforce a requested finite ResourceBudget must reject the request rather than silently ignore it.
8. **Network authority is explicit.** Managed Incus networking defaults to a Hacocoon-owned sandbox profile; higher-level domain-aware authorization remains a separate policy layer.
9. **Real-host acceptance is separate from repository tests.** Unit tests, fake-provider E2Es, race checks, and CI do not prove real Incus/Windows/future-cloud behavior.
10. **Cleanup and recovery are part of the feature.** Cancellation, retry, partial failure, and crash recovery are design requirements.

## Core vocabulary

The intentionally small Core vocabulary includes:

```text
Workspace
WorkspaceLease
Environment
Execution
CapabilityRequest
PolicyDecision
ApprovalRequest
BaseName / BaseRevision / BaseRef
ResourceBudget
```

Agent-session identities, VS Code/AHP types, Incus aliases, cloud identifiers, OCI registry mechanics, and storage-driver details remain integration/provider concerns.

## Roadmap

**Status legend:** ✅ implemented · 🧪 experimental/historical · 🚧 planned

| Version | Gate | Repository status |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented |
| v0.3 | Client & Interactive Access | ✅ implemented |
| v0.4 | Policy & Capability Foundation | ✅ implemented |
| v0.5 | Git / GitHub Capability | ✅ implemented |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 provider-routing seam retained; prior EC2/AWS/EBS implementation deferred |
| v0.8 | Client Adapters & VS Code Integration | ✅ implemented; real-client acceptance remains host-dependent |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented |
| v0.11 | Base Images & Custom Environments | ✅ first implementation slice |
| v0.12 | Sandbox Resource Limits | ✅ first implementation slice |
| v0.13 | Local OCI Registry | 🚧 planned; not implemented on `main` |

The implemented progression is contiguous through **v0.12**. v0.13 is the next design/implementation milestone and must not be described as current code reality until its implementation lands.

### v0.9 — Per-Agent Sandbox

A trusted integration maps an opaque external session identity to one dedicated Environment through the normal Environment/WorkspaceLease lifecycle.

```text
trusted client / integration
        |
 opaque session identity
        |
 persisted binding proof
        |
 dedicated Environment
```

Parallel write-capable sessions should receive distinct canonical Workspace paths, normally distinct Git worktrees. Worktree creation/branch strategy remains outside Core.

See [`09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`](09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md).

### v0.10 — VS Code Remote Agent Host Adapter

`haco-agent-host` connects a v0.9-bound Environment to the VS Code Agents workflow using the trusted client boundary and loopback-only SSH.

```text
VS Code Agents window
        |
   Remote SSH
        |
 haco-agent-host
        |
 bound Environment
        |
    /workspace
```

The client retains the private SSH key. Hacocoon owns Environment selection and safe connection preparation; VS Code owns Agent Host/AHP behavior.

See [`10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md`](10_v0.10_VSCODE_REMOTE_AGENT_HOST_ADAPTER.md).

### v0.11 — Base Images & Custom Environments

A logical Base is resolved once to an immutable revision before Environment creation:

```text
logical Base
     |
 provider source
     |
resolve once
     v
immutable revision
     |
 Environment
```

The first implementation slice includes Base selection, list/inspect, immutable Incus fingerprint pinning, and persisted Base identity. Custom build/import/history/rollback/GC remain follow-up work.

See [`11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md`](11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md) and [`BASE_IMAGES.md`](BASE_IMAGES.md).

### v0.12 — Sandbox Resource Limits

Each Environment can carry an explicit provider-neutral ResourceBudget:

```text
Environment
  +-- CPU
  +-- memory
  +-- PIDs
  +-- root storage
```

For Incus, finite limits are applied and verified before `start`. Unsupported finite requests fail closed instead of being silently ignored.

See [`12_v0.12_SANDBOX_RESOURCE_LIMITS.md`](12_v0.12_SANDBOX_RESOURCE_LIMITS.md).

### v0.13 — Local OCI Registry

v0.13 is **planned, not implemented on `main`**.

The intended first slice adds a Hacocoon-managed local OCI registry/cache gateway so `containerd`/`nerdctl` workloads can use ordinary image references while Hacocoon mediates upstream registry access and avoids distributing reusable upstream credentials to sandboxes.

A planned second slice adds OCI Seed + storage-driver COW optimization without sharing one writable `/var/lib/containerd` across Environments.

See:

- [`13_v0.13_LOCAL_OCI_REGISTRY.md`](13_v0.13_LOCAL_OCI_REGISTRY.md)
- [`13A_v0.13_OCI_SEED_AND_COW.md`](13A_v0.13_OCI_SEED_AND_COW.md)

## Workspace and orchestration boundary

A Workspace is opaque to Core. It may be a directory, Git repository, Git worktree, or output of an external/optional WorkspaceProvider.

```text
external orchestrator
  -> task / branch / worktree ownership
  -> Workspace path
  -> Hacocoon
  -> Environment
```

Daintree and similar orchestrators therefore sit above Hacocoon rather than inside Core.

## Client adapters

The first client adapter is `haco-vscode`:

```text
VS Code
  -> haco-vscode
  -> Hacocoon Environment + SSH access
  -> standard Remote-SSH
  -> /workspace
```

Other clients should consume the same generic Environment/client-access boundary. Future JetBrains, web, code-server, or other adapters must not introduce IDE-specific branching into Core.

## Human-in-the-loop boundary

Development approval and security approval are intentionally separate:

```text
Development approval -> Human / GitHub / external orchestrator
Security approval    -> Hacocoon Policy/Capability
```

Hacocoon security approval protects privileged authority. It is not a general project-management or merge-review engine.

## Acceptance model

A feature can be implemented in the repository while still requiring real-host validation.

| Evidence | What it proves |
|---|---|
| unit / adversarial tests | local logic and invariant coverage |
| process / fake-provider E2E | executable integration without real external infrastructure |
| repository CI | repeatable host-independent regression coverage |
| real Incus / Windows / future cloud acceptance | actual provider/client behavior on supported hosts |

Current host-dependent areas include real Incus lifecycle/network/resource enforcement, Windows/WSL + VS Code, Agent Host/AHP routing, and Base/image sources. Cloud acceptance will be defined again when a cloud adapter is reintroduced.

For exact current status, always use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).

## Deferred cloud runtime

The previous EC2 Environment provider was **experimental and disabled by default**. The EC2 runtime, AWS capability, EBS helper, and cloud-specific E2Es are not present in the current implementation tree.

This is a temporary development-focus choice while Hacocoon's local runtime and provider contracts are still changing quickly. The provider-neutral routing seam remains, and Git history plus the v0.7 design documents retain the previous implementation as reference material for a future adapter.

If cloud support is restored, it must remain explicit opt-in, keep credentials and authority on the trusted side, and fail before provider side effects when required policy or enforcement cannot be satisfied.

## Historical note

> [!NOTE]
> The 2026-08-29/30 rebaseline established the current product boundary and renumbered several early design gates. Historical commit messages, closed PRs, candidate branches, and superseded planning text may retain older names or version assignments. They are historical records, not current status.

The original v0.1 vertical slice was:

```text
host directory
  -> haco create --workspace
  -> Incus system container
  -> haco exec / haco shell
  -> haco delete
```

Later milestones extend that baseline; this historical record does not imply later features are absent.

## Pre-1.0 compatibility

Breaking changes remain allowed while the architecture is being hardened. Prefer a deliberate incompatible correction over preserving behavior that:

- leaks authority across a trust boundary;
- makes ownership ambiguous;
- creates unsafe cleanup/retry semantics;
- pushes provider-specific concepts into Core;
- silently weakens enforcement.

Breaking-change freedom must not be used to justify unsafe authority boundaries, ambiguous ownership, or silent data loss.

## Further reading

- **Current code reality:** [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- **Version numbering:** [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
- **Security architecture:** [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- **Terminology:** [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
- **Plugin architecture:** [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)
- **Documentation map:** [`README.md`](README.md)

> **Hacocoon runs developer tools and coding agents inside isolated, resource-bounded Environments without owning the IDE, Git workflow, or AI orchestration layer.**
