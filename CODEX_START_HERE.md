# CODEX START HERE

> **Hacocoon is a secure workspace runtime, not an AI orchestrator.**

This file is the fast path for coding agents and maintainers. Hacocoon is **pre-1.0**: breaking changes are acceptable when they simplify the system, strengthen trust boundaries, or correct unsafe/accidental contracts.

## Current status

- **Implemented milestones: v0.1 → v0.17**
- **Next planned milestone: v0.18 Optional Local OCI Registry**
- **Planned after that: v0.19 OCI Seed Builder & Btrfs/COW**
- **EC2 remains experimental and disabled by default**
- **OCI/Docker/nerdctl are optional plugin concerns, not Core requirements**

[`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md) is the source of truth for current code reality.

## Read first

1. [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md)
2. [`docs/00_REBASELINE_AND_ROADMAP.md`](docs/00_REBASELINE_AND_ROADMAP.md)
3. [`docs/00D_VERSIONING_AND_RELEASE_STATUS.md`](docs/00D_VERSIONING_AND_RELEASE_STATUS.md)
4. [`docs/00B_SECURITY_ARCHITECTURE.md`](docs/00B_SECURITY_ARCHITECTURE.md)
5. [`docs/00C_TERMINOLOGY_AND_BOUNDARIES.md`](docs/00C_TERMINOLOGY_AND_BOUNDARIES.md)
6. [`docs/00A_PLUGIN_ARCHITECTURE.md`](docs/00A_PLUGIN_ARCHITECTURE.md)
7. The relevant versioned specification.
8. [`docs/90_CODEX_IMPLEMENTATION_HANDOFF.md`](docs/90_CODEX_IMPLEMENTATION_HANDOFF.md)
9. [`.github/security/ADVERSARIAL_AUDIT.md`](.github/security/ADVERSARIAL_AUDIT.md)

## Roadmap snapshot

```text
v0.1   Secure Workspace Runtime MVP           implemented
v0.2   Workspace Abstraction & Lease          implemented
v0.3   Client & Interactive Access            implemented
v0.4   Policy & Capability Foundation         implemented
v0.5   Git / GitHub Push Capability           implemented
v0.6   Agent & Orchestrator Integration       implemented
v0.7   Remote / Cloud Runtime                 experimental
v0.8   Client Adapters & VS Code              implemented
v0.9   Per-Agent Sandbox                      implemented
v0.10  VS Code Remote Agent Host Adapter      implemented
v0.11  Base Images & Custom Environments      first slice implemented
v0.12  Sandbox Resource Limits                first slice implemented
v0.13  Managed Sandbox Network                implemented
v0.14  Git Fetch Plugin                       implemented
v0.15  OCI Seed Usage & Recommendation        optional-plugin first slice implemented
v0.16  OCI Image Deletion                     optional-plugin first slice implemented
v0.17  Docker Compatibility                   packaging foundation implemented
v0.18  Optional Local OCI Registry            planned
v0.19  OCI Seed Builder & Btrfs/COW           planned
```

## Architecture placement

```text
Workspace / leases             -> Core
Environment lifecycle          -> Core contract + provider adapter
Incus lifecycle/networking     -> runtime.incus adapter
Per-agent session binding      -> trusted integration outside Core
VS Code / Agent Host adapter   -> client integration outside Core
Base identity / ResourceBudget -> provider-neutral Core contracts
Policy / approval / audit      -> Core capability boundary
GitHub privileged Git          -> haco plugin git
OCI/Docker/nerdctl workflows   -> haco plugin oci
Git worktrees / task DAGs      -> external orchestrator
IDE / AI chat UX               -> client
Registry / Btrfs mechanics     -> optional infrastructure/provider detail
```

## Hard rules

- Do not make Git worktree, agent DAGs, model routing, retry strategy, or token budgets Core concepts.
- Do not make VS Code, JetBrains, Incus, AWS, Btrfs, OCI Registry, nerdctl or Docker brands part of Core vocabulary.
- Do not give coding agents Hacocoon/Incus management authority.
- Do not mount Host HOME, `~/.ssh`, `~/.aws`, Host Docker sockets, Incus sockets or Hacocoon control state into Environments as shortcuts.
- Do not export reusable parent credentials into arbitrary Environments.
- Privileged external operations must cross Policy/Capability/Audit boundaries.
- Deterministic Environment names are not ownership proof; persisted trusted binding is required.
- Mutable Base/OCI names are convenience input; durable identity pins immutable revisions/digests.
- Requested controls that cannot be enforced must fail closed.
- Managed sandbox networking must not silently fall back to broad/default Incus networking.
- Real-host acceptance is separate from unit/fake-provider/repository CI.
- Cleanup, retry, cancellation, concurrency and partial failure are part of the feature.

## Current extension commands

### Git

```text
haco plugin git fetch <environment>
haco plugin git push <environment> --branch <branch>
```

GitHub HTTPS authentication stays Host-side through the hardened `gh auth git-credential` broker path. Ordinary Git UX remains Git's responsibility.

### Optional OCI

```text
HACO_PLUGIN_OCI=nerdctl  # or docker
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

With `HACO_PLUGIN_OCI` unset, Hacocoon Core must not require container CLI/runtime tooling.

Top-level `haco image list|inspect` is Hacocoon Base identity, not workload OCI management.

## Planned gates

### v0.18 — Optional Local OCI Registry

Optional only where repeated downloads, rate limits or centralized policy justify it. Normal upstream pulls and OCI Seed construction do not require a Local Registry.

### v0.19 — OCI Seed Builder & Btrfs/COW

Trusted Host acquisition feeds an offline builder which publishes immutable Incus Seeds. Never share one writable `/var/lib/containerd` across Environments; physical sharing belongs to normal Incus/storage-driver COW semantics.

## Work method

1. Read the status and owning architecture/specification docs.
2. Inspect actual code/tests; docs are not proof of behavior.
3. Identify authority, ownership, lifecycle, network, client, Base, resource and failure boundaries.
4. Reproduce/define behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise hostile input, retry, cancellation, concurrency, partial failure and cleanup.
7. Run the maintained local entry point or equivalent checks:

```bash
bash tools/ci-local.sh
```

8. Keep real Incus, Windows/WSL + VS Code, Agent Host/AHP, AWS and OCI-tool acceptance separate from fake/process tests.
9. Update `docs/IMPLEMENTATION_STATUS.md` whenever repository reality changes.
10. Update numbering docs whenever milestone assignment changes.

When uncertain, choose the smallest design that preserves the trust boundary and keeps Core provider-neutral.
