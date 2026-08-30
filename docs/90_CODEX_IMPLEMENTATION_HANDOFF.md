# Codex Implementation Handoff

> **Maintenance guide for the current pre-1.0 architecture**

Implemented milestones are contiguous through **v0.17**. v0.18 Optional Local OCI Registry and v0.19 OCI Seed Builder/Btrfs-COW are planned.

Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for repository reality and [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) for milestone numbering.

## Objective

Keep Hacocoon small, explicit and fail-closed:

- preserve authority/ownership boundaries;
- prefer deliberate breaking simplification over accidental coupling;
- keep provider/client/tool-specific convenience outside Core;
- distinguish implementation from real-host acceptance;
- keep milestone numbering aligned with code reality.

## Required execution order

1. Read current status, versioning and the owning feature contract.
2. Inspect actual code/tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, network, Base/resource/client and failure boundaries.
4. Reproduce/define behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise retry, cancellation, concurrency, partial failure and cleanup.
7. Run maintained local/repository checks.
8. Report real-provider/client acceptance separately.
9. Update `IMPLEMENTATION_STATUS.md` whenever code reality changes.

## Architecture placement

```text
Workspace identity/lease       -> Core
Environment lifecycle          -> Core contract + provider adapter
Per-agent session binding      -> trusted integration outside Core
VS Code / Agent Host           -> client adapter outside Core
Base name/revision             -> provider-neutral Core contract
ResourceBudget                 -> provider-neutral Core contract
Managed Incus network          -> runtime.incus implementation of safety contract
Policy / approval / audit      -> Core capability boundary
GitHub privileged Git          -> haco plugin git
AWS authority                  -> capability/provider adapter
EC2 lifecycle                  -> runtime.ec2, experimental/default-off
OCI/Docker/nerdctl workflows   -> haco plugin oci
Local Registry / Btrfs detail  -> optional infrastructure/provider detail
Task DAG / worktree strategy   -> external orchestrator
IDE / AI chat UX               -> client
```

Do not move Incus/AWS/VS Code/GitHub/nerdctl/Docker/OCI Registry/Btrfs implementation details into Core just to simplify wiring.

## Implemented gates to preserve

### v0.8-v0.10 — Client and Agent Host

`haco-vscode` and `haco-agent-host` prepare standard loopback-only SSH access while private keys stay client-side. Coding agents do not receive Hacocoon/Incus management authority. Persisted binding proof, not deterministic names, controls agent-session ownership.

### v0.11 — Base identity

Logical Base names resolve to immutable revisions before Environment creation. Existing Environments remain bound to their creation-time revision. Top-level `haco image list|inspect` is Base identity, not workload OCI management.

### v0.12 — ResourceBudget

CPU/memory/PID/root-storage requests are provider-neutral. Requested finite limits that cannot be enforced fail closed. Incus applies and verifies finite limits before Environment access.

### v0.13 — Managed Sandbox Network

Read [`13_v0.13_MANAGED_SANDBOX_NETWORK.md`](13_v0.13_MANAGED_SANDBOX_NETWORK.md). Managed Incus network/profile drift or unsafe fallback must fail closed. IP/CIDR transport and domain-aware authorization are separate layers.

### v0.14 — Git Fetch Plugin

Read [`14_v0.14_GIT_FETCH_PLUGIN.md`](14_v0.14_GIT_FETCH_PLUGIN.md) and [`GIT_GITHUB_CAPABILITY.md`](GIT_GITHUB_CAPABILITY.md).

```text
haco plugin git fetch <environment>
haco plugin git push <environment> --branch <branch>
```

GitHub HTTPS credentials remain Host-owned and are accessed through the hardened `gh auth git-credential` broker path. Ordinary Git UX remains Git's responsibility.

### v0.15-v0.17 — Optional OCI plugin

Read:

- [`15_v0.15_OCI_SEED_RECOMMENDATION.md`](15_v0.15_OCI_SEED_RECOMMENDATION.md)
- [`16_v0.16_OCI_IMAGE_DELETION.md`](16_v0.16_OCI_IMAGE_DELETION.md)
- [`17_v0.17_DOCKER_COMPATIBILITY.md`](17_v0.17_DOCKER_COMPATIBILITY.md)
- [`OCI_RUNTIME_AND_DOCKER_COMPAT.md`](OCI_RUNTIME_AND_DOCKER_COMPAT.md)

OCI tooling is explicit opt-in:

```text
HACO_PLUGIN_OCI=nerdctl  # or docker
haco plugin oci status
haco plugin oci seed sample
haco plugin oci seed recommend
haco plugin oci image delete <reference>
```

When disabled, Core must not require nerdctl, Docker CLI, dockerd, Host OCI cache or Local Registry.

Deletion uses immutable identity and trusted tombstones. Docker compatibility is Environment-local; never mount the Host Docker socket into an Environment.

## Planned gates

### v0.18 — Optional Local OCI Registry

Read [`18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.md`](18_v0.18_OPTIONAL_LOCAL_OCI_REGISTRY.md). It is optional infrastructure for deployments with a measured need, not the default image path and not a Seed prerequisite.

### v0.19 — OCI Seed Builder & Btrfs/COW

Read [`19_v0.19_OCI_SEED_AND_COW.md`](19_v0.19_OCI_SEED_AND_COW.md).

Trusted Host acquisition feeds an offline builder that publishes immutable Incus Seeds. Never share one writable `/var/lib/containerd` across Environments and never open unrestricted builder egress as a shortcut.

## Client interaction work

Browser/Web notifications and richer Interaction API belong to future client/adapter work. A VS Code extension may surface them but must remain optional and must not become a Core transport dependency.

## Implementation style

- Go is the primary implementation language.
- Keep OS/provider/process/client side effects in narrow adapter/plugin layers.
- Prefer simple values/functions in Core.
- Add interfaces when testing or a real second implementation justifies the seam.
- Fail explicitly; cleanup and recovery are part of a feature.
- Treat Environment workloads, backend output, files and external process output as hostile.
- Never expose long-lived Host credentials or Hacocoon control authority inside an Environment.

## Validation baseline

Prefer the maintained entry point:

```bash
bash tools/ci-local.sh
```

At minimum it covers docs consistency, workflow/release policy, test/vet, race and E2E jobs when their local dependencies are available. Real Incus, Windows/WSL + VS Code, Agent Host, AWS, OCI tooling and Btrfs acceptance require suitable external environments before being reported as passed.

## Stop conditions

Revisit the design rather than improvising if a change would:

- expand Core with provider/client/tool brands;
- weaken fail-closed policy/network/resource behavior;
- expose Host credentials or control sockets to an Environment;
- silently destroy recoverable/referenced state;
- make an experimental backend implicitly active;
- turn Hacocoon into an orchestrator, worktree manager, IDE or container-tool manager;
- assign a roadmap version without updating the authoritative versioning/status documents.
