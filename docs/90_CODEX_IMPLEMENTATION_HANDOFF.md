# Codex Implementation Handoff

> **Maintenance guide for the current pre-1.0 architecture**

Fully implemented product milestones are contiguous through **v0.16**. v0.17 Docker Compatibility Plugin has a foundation but is incomplete. v0.18 Optional Local OCI Registry and v0.19 OCI Seed Builder & Btrfs/COW are planned. The v0.7 provider-routing seam remains, while the former concrete EC2/AWS/EBS implementation is deferred.

Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for current repository reality and [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) for authoritative milestone numbering.

## Versioning rule

> **One independently useful product feature is approximately one minor milestone.**

When a new independent feature lands, its implementation PR must take the next `v0.N` and update both numbering and implementation-status docs. Security fixes, bug fixes, hardening, refactors, CLI namespace cleanup, CI, docs, release engineering, and test-only work normally do not consume another product version.

## Required execution order

1. Read `IMPLEMENTATION_STATUS.md`, `00D_VERSIONING_AND_RELEASE_STATUS.md`, and the owning design/specification.
2. Inspect actual implementation/tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, network, agent binding, Base, resource, plugin, client, and failure boundaries.
4. Reproduce/define intended behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise retry, cancellation, concurrency, partial failure, and cleanup.
7. Run the maintained local CI entry point or relevant individual checks.
8. Keep real-provider/client acceptance claims separate.
9. Update `IMPLEMENTATION_STATUS.md` when code reality materially changes.
10. If this is a new independent product feature, update `00D_VERSIONING_AND_RELEASE_STATUS.md` and assign the next minor **in the same PR**.

## Architecture placement

```text
Workspace identity/lease       -> Core + workspace boundary
Environment lifecycle          -> Core contract + provider adapter
Incus lifecycle/networking     -> runtime.incus adapter
future cloud runtime           -> provider adapter; currently deferred
Per-agent session binding      -> trusted integration outside Core
VS Code Agent Host adapter     -> client adapter outside Core
Base name/revision             -> provider-neutral Environment contract / `haco base`
ResourceBudget                 -> provider-neutral Environment config
Policy/approval/audit          -> Policy + Capability
Git push/fetch                 -> Git/GitHub plugin/capability boundary
OCI/container lifecycle        -> optional `haco plugin oci` adapter boundary
Docker compatibility           -> optional plugin/adapter; not Core runtime
OCI registry/Seed mechanics    -> host/provider adapter; not Core
Btrfs/QCOW2/storage mechanics  -> provider/adapter detail
IDE/editor/AI chat UX          -> client responsibility
worktree/task DAG              -> external orchestrator responsibility
```

Do not move provider/client/plugin vocabulary into Core merely to make wiring convenient.

## Current newer contracts

### v0.13 — Managed Sandbox Network

Read `13_v0.13_MANAGED_SANDBOX_NETWORK.md`. Use Hacocoon-owned managed network/profile state, verify before use, fail closed on drift, and never silently fall back to broad/default Incus networking. Domain-aware egress policy belongs in a higher-layer broker/proxy/plugin.

### v0.14 — Git Fetch Plugin

Read `14_v0.14_GIT_FETCH_PLUGIN.md`. Keep fetch under `haco plugin git`, keep reusable GitHub credentials on the trusted Host, use `gh auth git-credential`, reject repository-controlled credential/transport rewrites, and do not move Git into Core.

### v0.15 — OCI Seed Recommendation

Read `15_v0.15_OCI_SEED_RECOMMENDATION.md`. OCI/containerd/nerdctl operations stay under `haco plugin oci`; telemetry stores OCI identity rather than arbitrary Environment/Workspace data; recommendations require immutable digests; deterministic top-10% automatic promotion selects content only, never credentials.

### v0.16 — OCI Image Deletion

Read `16_v0.16_OCI_IMAGE_DELETION.md`. Deletion stays under `haco plugin oci image delete`; identity is reference + immutable digest; mutable ambiguity/tag movement fails closed; no forced all-Environment deletion; tombstones override automatic Seed re-promotion; partial destructive work becomes recovery-required.

### Base / OCI namespace rule

`haco base ...` owns Hacocoon Environment starting points. `haco plugin oci ...` owns OCI/containerd/nerdctl-specific lifecycle. The old ambiguous `haco image ...` surface is intentionally removed. This is a boundary/refactor rule and does not consume a milestone by itself.

### v0.17 — Docker Compatibility Plugin

Read `17_v0.17_DOCKER_COMPATIBILITY_PLUGIN.md` and `OCI_RUNTIME_AND_DOCKER_COMPAT.md`. Standard runtime remains containerd + nerdctl; Docker compatibility stays optional/outside Core; genuine Docker CLI compatibility is allowed; `dockerd` should be on-demand; current foundation is not yet the complete v0.17 gate.

### v0.18 — Optional Local OCI Registry

Read `18_v0.18_LOCAL_OCI_REGISTRY.md`. A registry/proxy is optional. Do not make it a prerequisite for normal `nerdctl pull` or OCI Seed.

### v0.19 — OCI Seed Builder & Btrfs/COW

Read `19_v0.19_OCI_SEED_AND_COW.md`. Never share one writable `/var/lib/containerd` between Environments, copy live containerd internals as a shortcut, manipulate hidden Incus/Btrfs state behind the provider, open unrestricted Internet egress for the Seed Builder, or embed reusable Host registry credentials into a Seed.

## Existing important contracts

- v0.7 keeps the provider-neutral routing seam, but the former concrete EC2/AWS/EBS implementation is currently deferred and absent from the active tree.
- v0.9 ownership requires persisted binding proof; deterministic names are insufficient.
- v0.10 `haco-agent-host` keeps private SSH keys client-side and leaves VS Code responsible for Agent Host/AHP behavior.
- v0.11 Bases resolve mutable names once to immutable revisions before Environment creation and use `haco base` as their CLI namespace.
- v0.12 requested finite resource limits fail closed if the provider cannot enforce/read them back.

## Implementation style

- Go remains the primary implementation language.
- Keep OS/Incus/process/client/plugin side effects in narrow imperative adapter layers.
- Prefer simple values/functions in core paths.
- Introduce interfaces for real seams or testing value, not noun similarity.
- Fail explicitly; cleanup and partial failure are part of the feature.
- Treat Environment workloads, image contents, backend output, filesystem state, client config, and external process output as hostile.
- Do not expose long-lived Host credentials or Hacocoon control authority inside an Environment.

## Validation baseline

Prefer the maintained local CI entry point:

```bash
tools/ci-local.sh
```

Real Incus, networking/resource enforcement, Base/image lifecycle, Windows/WSL + VS Code, Agent Host/AHP, private registries, Docker compatibility, and OCI Seed/Btrfs behavior require suitable external environments before being reported as passed. Cloud acceptance remains deferred until a concrete cloud adapter is restored.

## Stop conditions

Revisit the design instead of improvising when a change would:

- expand Core with provider-specific/client/plugin brands;
- weaken fail-closed authority/network/resource behavior;
- expose parent credentials or control sockets to an Environment/builder;
- silently destroy recoverable state or referenced revisions;
- make a deferred/experimental backend implicitly active;
- turn Hacocoon into an AI orchestrator, worktree manager, IDE owner, or Docker-specific Core runtime;
- land a new independent feature without assigning/reconciling its minor milestone in the same PR.
