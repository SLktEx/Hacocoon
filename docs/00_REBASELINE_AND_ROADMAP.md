# Architecture & Roadmap

> **Architecture baseline · Updated 2026-08-30**
>
> Hacocoon is a **Secure Workspace Runtime**.  
> For code reality use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md).  
> For authoritative milestone numbering use [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md).

Hacocoon runs developer tools and coding agents inside isolated Environments while keeping host, GitHub, cloud, network, and other privileged authority behind explicit trusted boundaries.

> [!IMPORTANT]
> Hacocoon is **pre-1.0**. Roadmap milestones describe product gates, not API stability, production support, or proof that every real-host acceptance test has passed.

## Project status at a glance

| Track | Status |
|---|---|
| Fully implemented milestones | **v0.1 → v0.16** contiguous on `main` |
| Partial milestone | **v0.17 Docker Compatibility Plugin** — foundation implemented |
| Planned milestones | **v0.18 Optional Local OCI Registry**, **v0.19 OCI Seed Builder & Btrfs/COW** |
| Default local runtime | Incus |
| Standard OCI runtime direction | containerd + nerdctl |
| Remote runtime | EC2 — experimental and disabled by default |
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
         /           \                  /       |       \
 runtime.incus   runtime.ec2        GitHub     AWS     Host
 local default  experimental
```

### Hacocoon owns

- Workspace resolution/canonical identity;
- isolated Environment lifecycle/cleanup;
- execution and interactive access;
- Workspace lease/ownership safety;
- client-access primitives;
- policy, approval, audit, and scoped capabilities;
- trusted per-session Environment binding;
- provider-neutral Base and ResourceBudget contracts;
- managed Incus sandbox-network substrate;
- recovery behavior for authority-sensitive operations.

### Hacocoon does not own

- IDE/editor or AI chat UX;
- model selection/routing/token budgets;
- task decomposition, retry strategy, or agent DAGs;
- Git branch/worktree orchestration as a Core concern;
- development review/merge workflow;
- provider-native Incus/AWS/Btrfs/OCI/Docker/client protocol details inside Core.

Thin adapters/plugins may integrate those systems without redefining Core.

## Architecture invariants

1. **Core stays provider-neutral.** Incus, AWS, VS Code, AHP, Btrfs, OCI, Docker, and similar technologies remain behind adapters/plugins.
2. **The coding Environment is not the control plane.** Coding agents do not gain Hacocoon/Incus administrator authority.
3. **Credentials stay on the trusted side unless explicitly Environment-owned.** Reusable Host credentials are not copied into arbitrary Environments.
4. **Privileged operations are brokered.** Git push/fetch and external-service authority cross explicit plugin/capability boundaries.
5. **Ownership is proven, not inferred.** Deterministic names alone do not authorize adoption/deletion.
6. **Mutable names resolve to immutable identity.** Base aliases and OCI references pin immutable revisions/digests where durable identity matters.
7. **Explicit limits fail closed.** Unsupported finite ResourceBudget requests are rejected.
8. **Network authority is explicit.** Hacocoon-managed Incus networking does not silently fall back to broad/default profiles.
9. **Real-host acceptance is separate from repository tests.**
10. **Cleanup/retry/crash recovery are part of the feature.**
11. **One independent product feature is approximately one minor milestone.** Fixes/hardening/refactors do not normally consume a version.

## Roadmap

**Status legend:** ✅ implemented · 🧪 partial/experimental · 🚧 planned

| Version | Gate | Repository status |
|---|---|---|
| v0.1 | Secure Workspace Runtime MVP | ✅ implemented |
| v0.2 | Workspace Abstraction & Lease | ✅ implemented |
| v0.3 | Client & Interactive Access | ✅ implemented |
| v0.4 | Policy & Capability Foundation | ✅ implemented |
| v0.5 | Git / GitHub Capability | ✅ implemented |
| v0.6 | Agent & Orchestrator Integration | ✅ implemented |
| v0.7 | Remote / Cloud Runtime & External Capabilities | 🧪 experimental implementation; EC2 default-off |
| v0.8 | Client Adapters & VS Code Integration | ✅ implemented |
| v0.9 | Per-Agent Sandbox & Agent Host Integration | ✅ broker foundation implemented |
| v0.10 | VS Code Remote Agent Host Adapter | ✅ implemented |
| v0.11 | Base Images & Custom Environments | ✅ first implementation slice |
| v0.12 | Sandbox Resource Limits | ✅ first implementation slice |
| v0.13 | Managed Sandbox Network | ✅ implemented |
| v0.14 | Git Fetch Plugin | ✅ implemented |
| v0.15 | OCI Seed Recommendation | ✅ implemented |
| v0.16 | OCI Image Deletion | ✅ first implementation slice |
| v0.17 | Docker Compatibility Plugin | 🧪 foundation implemented; completion pending |
| v0.18 | Optional Local OCI Registry | 🚧 planned |
| v0.19 | OCI Seed Builder & Btrfs/COW | 🚧 planned |

### v0.13 — Managed Sandbox Network

Hacocoon creates/verifies its own Incus bridge/profile/ACL substrate and uses `haco-sandbox` for new local sandbox Environments. Drift or missing security state fails closed rather than silently falling back to Incus `default`.

Domain-aware egress authorization is not faked with Incus ACLs; it remains a separate higher-layer broker/proxy/plugin concern.

See [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md).

### v0.14 — Git Fetch Plugin

`haco plugin git fetch` uses the Host's `gh auth git-credential` path without exporting reusable credentials into the Environment. The broker rejects repository-controlled credential/transport configuration that could redefine the privileged operation.

See [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md).

### v0.15 — OCI Seed Recommendation

Hacocoon samples Environment OCI identities to trusted Host state, recommends immutable `reference + digest` candidates, and marks the top 10% for automatic future Seed inclusion.

This is selection policy only; physical Seed build/publish is v0.19.

See [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md).

### v0.16 — OCI Image Deletion

`haco image delete` safely removes a selected identity from the Host Seed cache and persists a tombstone that overrides v0.15 automatic promotion. Optional `--all-environments` is explicit, preflighted, retry-safe, and never uses forced removal.

See [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md).

### v0.17 — Docker Compatibility Plugin

The standard runtime remains containerd + nerdctl. Docker support is an optional plugin/integration path for software that requires genuine Docker CLI/Engine APIs. The repository currently contains the initial design and systemd socket/service foundation; full plugin lifecycle and on-demand Engine integration remain follow-up.

See [`17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md`](17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md) and [`OCI_RUNTIME_AND_DOCKER_COMPAT.md`](OCI_RUNTIME_AND_DOCKER_COMPAT.md).

### v0.18 — Optional Local OCI Registry

A Local Registry/proxy is optional infrastructure for repeated pull cost, rate limits, restricted-network installations, or centralized policy/audit. Normal Environment `nerdctl pull` remains direct-to-configured-upstream when policy allows.

See [`18_v0.18_LOCAL_OCI_REGISTRY.md`](18_v0.18_LOCAL_OCI_REGISTRY.md).

### v0.19 — OCI Seed Builder & Btrfs/COW

The planned Seed pipeline acquires OCI content on the trusted Host, feeds an offline builder, stops containerd cleanly, publishes an immutable Incus Seed revision, pins a manifest/current pointer, and relies on normal Incus/storage-driver cloning for COW benefits.

It must never share one writable `/var/lib/containerd` across Environments.

See [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md).

## Workspace/orchestration boundary

A Workspace is opaque to Core. It may be a directory, Git repository, worktree, or an external WorkspaceProvider output. Branch/worktree/task ownership remains above Hacocoon.

## Client adapter boundary

The first client adapter is `haco-vscode`. Other IDE/web adapters consume the same generic Environment/client-access boundary and must not introduce IDE-specific branching into Core.

## Human-in-the-loop boundary

```text
Development approval -> Human / GitHub / external orchestrator
Security approval    -> Hacocoon Policy/Capability
```

Hacocoon security approval protects privileged authority; it is not a general project-management/merge-review engine.

## Acceptance model

| Evidence | What it proves |
|---|---|
| unit / adversarial tests | local logic and invariant coverage |
| process / fake-provider E2E | executable integration without real external infrastructure |
| repository CI / local CI runner | repeatable host-independent regression coverage |
| real Incus / Windows / AWS acceptance | actual provider/client behavior on supported hosts |

## Historical note

The 2026-08-30 feature-number rebaseline replaced the old grouping where Local Registry and multiple OCI Seed subfeatures were all called `v0.13`/`v0.13A/B/C`. Historical commits, closed PRs, and old branches may retain those labels, but current documentation uses v0.13-v0.19 above.

## Pre-1.0 compatibility

Breaking changes remain allowed when they deliberately fix unsafe authority boundaries, ambiguous ownership, accidental provider coupling, unsafe cleanup/retry behavior, or unnecessary architecture complexity.

## Further reading

- [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md)
- [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md)
- [`00B_SECURITY_ARCHITECTURE.md`](00B_SECURITY_ARCHITECTURE.md)
- [`00C_TERMINOLOGY_AND_BOUNDARIES.md`](00C_TERMINOLOGY_AND_BOUNDARIES.md)
- [`00A_PLUGIN_ARCHITECTURE.md`](00A_PLUGIN_ARCHITECTURE.md)
- [`README.md`](README.md)
