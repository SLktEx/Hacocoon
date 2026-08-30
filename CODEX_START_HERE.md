# CODEX START HERE

> **Hacocoon is a secure workspace runtime, not an AI orchestrator.**

This is the fast path for coding agents and maintainers. Hacocoon is **pre-1.0**: breaking changes are acceptable when they simplify the system, strengthen trust boundaries, or correct unsafe/accidental contracts.

## Current status

- **Fully implemented product milestones: v0.1 → v0.16**
- **v0.17 Docker Compatibility Plugin: foundation implemented, completion pending**
- **v0.18 Optional Local OCI Registry: planned**
- **v0.19 OCI Seed Builder & Btrfs/COW: planned**
- **EC2 remains experimental and disabled by default**

Do not infer implementation from a specification filename. [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) is the source of truth for current code reality.

## Read first

1. [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md)
2. [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md)
3. [`docs/00D_VERSIONING_AND_RELEASE_STATUS.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.md)
4. [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md)
5. [`docs/00C_TERMINOLOGY_AND_BOUNDARIES.md`](docs/00C_TERMINOLOGY_AND_BOUNDARIES.md)
6. The relevant versioned specification
7. [`docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`](docs/90_CODEX_IMPLEMENTATION_HANDOFF.md)
8. [`.github/security/ADVERSARIAL_AUDIT.md`](.github/security/ADVERSARIAL_AUDIT.md) for hostile/security-sensitive review

## Versioning rule

> **One independently useful product feature is approximately one minor milestone.**

When a feature lands, update the authoritative numbering and implementation-status docs in that feature PR. Security fixes, bug fixes, hardening, refactors, CI, docs, and test-only work normally do not consume a new product version.

## Roadmap snapshot

```text
v0.1   Secure Workspace Runtime MVP                implemented
v0.2   Workspace Abstraction & Lease               implemented
v0.3   Client & Interactive Access                 implemented
v0.4   Policy & Capability Foundation              implemented
v0.5   Git / GitHub Capability                     implemented
v0.6   Agent & Orchestrator Integration            implemented
v0.7   Remote / Cloud Runtime                      experimental implementation
v0.8   Client Adapters & VS Code                   implemented
v0.9   Per-Agent Sandbox & Agent Host Integration  broker foundation implemented
v0.10  VS Code Remote Agent Host Adapter           implemented
v0.11  Base Images & Custom Environments           first slice implemented
v0.12  Sandbox Resource Limits                     first slice implemented
v0.13  Managed Sandbox Network                     implemented
v0.14  Git Fetch Plugin                            implemented
v0.15  OCI Seed Recommendation                     implemented
v0.16  OCI Image Deletion                          first slice implemented
v0.17  Docker Compatibility Plugin                 foundation / partial
v0.18  Optional Local OCI Registry                 planned
v0.19  OCI Seed Builder & Btrfs/COW                planned
```

## Architecture placement

```text
Workspace / leases             -> Core + workspace boundary
Environment lifecycle          -> Core contract + provider adapter
Incus lifecycle/networking     -> runtime.incus adapter
Per-agent session binding      -> trusted integration outside Core
VS Code / Agent Host adapter   -> client integration outside Core
Base identity                  -> provider-neutral Environment contract
ResourceBudget                 -> provider-neutral Environment contract
Policy / approval / audit      -> Policy + Capability
GitHub / AWS authority         -> capability plugins/adapters
Git fetch/push                 -> Git/GitHub plugin boundary
Docker compatibility           -> optional plugin/adapter, not Core runtime
Git worktrees / task DAGs      -> external orchestrator
IDE / AI chat UX               -> client
OCI registry / Seed mechanics  -> host/provider adapter, not Core
```

## Hard rules

- Do not make Git worktrees, agent DAGs, model routing, retry strategy, or token budgets Core concepts.
- Do not make VS Code, JetBrains, AHP, Incus, AWS, Btrfs, OCI, Docker, or similar provider/client brands Core vocabulary.
- Do not give coding agents Hacocoon/Incus management authority.
- Do not mount host HOME, reusable credential stores, Incus sockets, or Hacocoon control state into Environments as shortcuts.
- Privileged external operations must cross explicit Policy/Capability/plugin boundaries.
- Deterministic Environment names are not ownership proof.
- Mutable Base/OCI names are convenience input; durable identity pins immutable revisions/digests.
- Requested limits/security controls that cannot be enforced fail closed.
- Managed sandbox networking must not silently fall back to broad/default Incus networking.
- Standard OCI runtime direction remains containerd + nerdctl; Docker compatibility is optional.
- OCI Seed must never share one writable `/var/lib/containerd` across Environments.
- Real-host acceptance is separate from unit/fake-provider/repository CI.
- Cleanup, retry, cancellation, concurrency, and partial failure are part of the feature.

## Current newer gates

### v0.13 — Managed Sandbox Network

Hacocoon owns/verifies `haco-sandbox0`, `haco-sandbox-egress`, and `haco-sandbox`; drift fails closed. Domain-aware authorization remains a higher-layer concern.

### v0.14 — Git Fetch Plugin

`haco plugin git fetch` uses the trusted Host `gh auth git-credential` path and rejects repository-controlled transport/credential configuration.

### v0.15 — OCI Seed Recommendation

`haco image seed sample|recommend` records trusted latest snapshots, recommends immutable identities, and marks the deterministic top 10% for future Seed inclusion.

### v0.16 — OCI Image Deletion

`haco image delete` safely removes/tombstones an immutable identity for future Seed selection; optional all-Environment deletion is explicit and retry-safe.

### v0.17 — Docker Compatibility Plugin

Current repository reality is foundation only. Keep containerd + nerdctl standard; finish Docker Engine compatibility as an optional plugin/on-demand path rather than Core.

### v0.18 / v0.19 — planned OCI infrastructure

Local Registry is optional. OCI Seed Builder is offline and Host-fed; physical immutable Seed publication/COW remains v0.19.

## Work method

1. Read current status and owning architecture/specification docs.
2. Inspect actual code/tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, network, client, Base, resource, plugin, and failure boundaries.
4. Reproduce/define behavior with tests where practical.
5. Implement the smallest coherent change.
6. Exercise hostile input, retry, cancellation, concurrency, partial failure, and cleanup.
7. Run maintained checks:

```bash
tools/ci-local.sh
```

or the individual Go/docs checks documented in `CONTRIBUTING.md`.

8. Keep real Incus, Windows/WSL + VS Code, Agent Host/AHP, private registries, and AWS acceptance separate from fake/process tests.
9. Update `docs/IMPLEMENTATION_STATUS.md` whenever repository reality materially changes.
10. For a new independent product feature, take the next minor milestone and update `docs/00D_VERSIONING_AND_RELEASE_STATUS.md` **in the same PR**.

When uncertain, choose the smallest design that preserves the trust boundary and keeps Core provider-neutral.
