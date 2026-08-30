# CODEX START HERE

> **Hacocoon is a secure workspace runtime, not an AI orchestrator.**

This file is the fast path for coding agents and maintainers. Hacocoon is **pre-1.0**: breaking changes are acceptable when they simplify the system, strengthen trust boundaries, or correct unsafe/accidental contracts.

## Current status

- **Implemented on `main`: v0.1 → v0.12**
- **Next planned milestone: v0.13 Local OCI Registry**
- **Planned v0.13 second slice: OCI Seed & Btrfs/COW optimization**
- **EC2 remains experimental and disabled by default**

Do not infer implementation from a specification filename. [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) is the source of truth for current code reality.

## Read first

1. [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) — current repository reality and acceptance gaps.
2. [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md) — product boundary and roadmap intent.
3. [`docs/00D_VERSIONING_AND_RELEASE_STATUS.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.md) — authoritative milestone numbering.
4. [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md) — cross-cutting trust rules.
5. [`docs/00C_TERMINOLOGY_AND_BOUNDARIES.md`](docs/00C_TERMINOLOGY_AND_BOUNDARIES.md) — canonical vocabulary.
6. The relevant versioned specification.
7. [`docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`](docs/90_CODEX_IMPLEMENTATION_HANDOFF.md) — implementation workflow.
8. [`.github/security/ADVERSARIAL_AUDIT.md`](.github/security/ADVERSARIAL_AUDIT.md) for hostile/security-sensitive review.

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
v0.13  Local OCI Registry                          planned; not implemented
v0.13A OCI Seed & Btrfs/COW Optimization           planned second slice
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
GitHub / AWS authority         -> capability adapters
Git worktrees / task DAGs      -> external orchestrator
IDE / AI chat UX               -> client
OCI registry / Seed mechanics  -> host/provider adapter, not Core
```

## Hard rules

- Do not make Git worktree, agent DAGs, model routing, retry strategy, or token budgets Core concepts.
- Do not make VS Code, JetBrains, Daintree, AHP, Incus, AWS, Btrfs, or OCI provider/client brands part of Core vocabulary.
- Do not give coding agents Hacocoon/Incus management authority.
- Do not mount host HOME, `~/.ssh`, `~/.aws`, Incus sockets, or Hacocoon control state into Environments as shortcuts.
- Do not export reusable parent credentials into arbitrary Environments.
- Privileged external operations must cross Policy/Capability/Audit boundaries.
- Deterministic Environment names are not ownership proof; persisted trusted binding is required for agent-session lifecycle.
- Mutable Base/OCI names are convenience input; durable identity must pin immutable revisions/digests.
- Requested limits or security controls that cannot be enforced must fail closed.
- Managed sandbox networking must not silently fall back to broad/default Incus networking.
- Real-host acceptance is separate from unit/fake-provider/repository CI.
- Cleanup, retry, cancellation, concurrency, and partial failure are part of the feature.

## Implemented gates

### v0.8 — Client Adapter

`haco-vscode` translates generic Environment/client-access state into standard VS Code Remote-SSH. VS Code owns editor/terminal/Git/AI UX; Hacocoon owns the Environment and authority boundary.

### v0.9 — Per-Agent Sandbox

`internal/agenthost` binds opaque trusted session identities to dedicated Environments. Reacquire is idempotent only for the exact persisted binding; adoption/release without matching ownership proof fails closed.

### v0.10 — Remote Agent Host Adapter

`haco-agent-host` is implemented on `main` (PR #137). It prepares a v0.9-bound Environment as a loopback-only SSH target while keeping the private key on the client side. VS Code remains responsible for Agent Host/AHP behavior.

### v0.11 — Base Images

The first slice implements logical Base selection, immutable revision pinning, persisted Base identity, and `haco image list` / `inspect` / `create --base`. Custom build/import/history/rollback/GC remain follow-up work.

### v0.12 — Resource Budgets

The first slice implements provider-neutral CPU, memory, PID, and root-storage budgets. Incus finite limits are applied/read-back verified before start. Unsupported finite requests fail closed.

## Planned gate: v0.13

[`docs/13_v0.13_LOCAL_OCI_REGISTRY.md`](docs/13_v0.13_LOCAL_OCI_REGISTRY.md) and [`docs/13A_v0.13_OCI_SEED_AND_COW.md`](docs/13A_v0.13_OCI_SEED_AND_COW.md) are **design contracts, not implementation claims**.

Implementation order is security-sensitive:

1. Local OCI registry/cache gateway.
2. Transparent Environment-side registry routing without direct-registry fallback.
3. Narrow trusted Seed Builder registry path.
4. OCI Seed publish/pinning.
5. COW optimization through normal Incus/storage-driver cloning.

Never implement OCI Seed by sharing one writable `/var/lib/containerd` across Environments or by opening unrestricted builder egress.

## Work method

1. Read current status and the owning architecture/specification docs.
2. Inspect actual code and tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, network, client, Base, resource, and failure boundaries.
4. Reproduce/define behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise hostile input, retry, cancellation, concurrency, partial failure, and cleanup.
7. Run maintained checks:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
go build ./cmd/haco-agent-host
python tools/check_docs.py
```

8. Keep real Incus, Windows/WSL + VS Code, Agent Host/AHP, and AWS acceptance separate from fake/process tests.
9. Update `docs/IMPLEMENTATION_STATUS.md` whenever repository reality materially changes.
10. Update numbering docs only when the milestone assignment changes.

When uncertain, choose the smallest design that preserves the trust boundary and keeps Core provider-neutral.
