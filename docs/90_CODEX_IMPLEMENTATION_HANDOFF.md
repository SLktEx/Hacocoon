# Codex Implementation Handoff

> **Maintenance guide for the current pre-1.0 architecture**

Implemented milestones are contiguous through **v0.12**. **v0.13 Local OCI Registry is planned and not implemented on `main`**; OCI Seed & Btrfs/COW is a planned second slice.

Use [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) for current repository reality and [`00D_VERSIONING_AND_RELEASE_STATUS.md`](00D_VERSIONING_AND_RELEASE_STATUS.md) for milestone numbering.

## Objective

Keep Hacocoon small, explicit, and fail-closed while it is still pre-1.0:

- harden security and failure behavior;
- close test and real-host acceptance gaps;
- simplify accidental complexity;
- preserve architecture and ownership boundaries;
- make deliberate breaking changes when the current contract is unsafe or unnecessarily costly;
- keep client/provider-specific convenience outside Core;
- keep roadmap numbering aligned with implementation reality;
- never report planned gates as implemented.

## Required execution order

For a change or bug fix:

1. Read `IMPLEMENTATION_STATUS.md`, `00D_VERSIONING_AND_RELEASE_STATUS.md`, and the owning design/specification.
2. Inspect actual implementation and tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, network, agent binding, Base, resource, client, and failure boundaries.
4. Reproduce the bug or define intended behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise retry, cancellation, concurrency, partial failure, and cleanup paths.
7. Run unit, race, vet, build, docs, integration, and available E2E checks.
8. Keep real-provider/client acceptance claims separate when Incus/Windows/VS Code/AWS infrastructure is unavailable.
9. Update `IMPLEMENTATION_STATUS.md` whenever code reality materially changes.

## Architecture placement

```text
Workspace identity/lease       -> Core concepts + workspace boundary
Environment lifecycle          -> Core contract + provider adapter
Per-agent session binding      -> trusted integration layer outside Core
VS Code Agent Host adapter     -> trusted client adapter boundary, outside Core
Base name/revision             -> Environment starting-point domain contract
ResourceBudget                 -> provider-neutral Environment configuration
Incus image mapping            -> runtime.incus adapter
Incus lifecycle/networking     -> runtime.incus adapter
Client status/SSH/ports        -> generic client access boundary
VS Code SSH config/launch      -> client adapter, outside Core
IDE/editor/AI chat UX          -> client responsibility
Daintree/worktree/task DAG     -> external orchestrator responsibility
Policy/approval/audit          -> Policy + Capability foundation
Git/GitHub authority           -> Git/GitHub capability boundary
AWS external authority         -> aws.api capability boundary
EC2 lifecycle                  -> runtime.ec2, experimental/default-off
OCI registry/cache/Seed        -> host/provider adapter; planned v0.13 work
Btrfs/QCOW2/storage mechanics  -> provider/adapter detail only when justified
```

Do not move provider fields, `if ec2`, Incus aliases/fingerprints, IDE-brand behavior, AHP details, OCI implementation details, or orchestrator-brand behavior into Core merely to make wiring convenient.

## Implemented contracts

### v0.8 — Client Adapter

The first adapter is `haco-vscode`.

```text
Workspace
 -> create/reuse Hacocoon Environment
 -> request loopback-only SSH access
 -> write adapter-owned client SSH config
 -> launch standard VS Code Remote-SSH
 -> open /workspace
```

Rules:

- private SSH keys remain client-side;
- do not own the user's entire SSH configuration;
- distinguish Windows client SSH state from WSL host state;
- do not reimplement Remote-SSH;
- do not add a Hacocoon AI chat UI;
- future clients reuse generic Environment/client-access contracts rather than adding client brands to Core.

### v0.9 — Per-Agent Sandbox

Read `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md` before changing `internal/agenthost`.

Required rules:

- coding agents do not receive Hacocoon/Incus management authority;
- raw session IDs are not runtime names or ownership proof;
- exact reacquire is idempotent;
- Workspace/access-mode rebinding fails closed;
- release requires persisted binding proof;
- deterministic Environment-name coincidence is not enough to adopt/delete an Environment;
- parallel RW sessions normally use distinct canonical Workspace paths/worktrees;
- real Agent Host/AHP routing acceptance is separate from broker implementation.

### v0.10 — Remote Agent Host Adapter

`haco-agent-host` is implemented on `main` through PR #137.

Rules:

- the client keeps the private SSH key;
- only the public key enters the existing Hacocoon SSH preparation path;
- reuse compatible managed connections and rotate changed connections safely;
- adapter setup failure must not silently release a pre-existing v0.9 Environment;
- VS Code owns Agent Host/AHP behavior;
- Hacocoon owns trusted Environment selection and safe connection preparation;
- real Windows/WSL + Incus + VS Code acceptance remains host-dependent.

### v0.11 — Base Images & Custom Environments

The first implementation slice is already present.

```text
logical Base name
  -> resolve immutable Base revision
  -> map to existing Incus image fingerprint internally
  -> create Environment
  -> persist Base name + immutable revision
```

Rules:

- Incus aliases/remotes/fingerprints stay adapter details;
- existing Environments remain bound to their creation-time Base revision;
- custom Base contents are untrusted guest data and cannot grant host authority;
- future build/import code must not execute directly with broad host authority;
- local archives are hostile input;
- future deletion/GC must preserve referenced revisions;
- moving to another Base revision means creating/recreating an Environment rather than mutating its starting identity.

### v0.12 — Sandbox Resource Limits

The first implementation slice is already present.

Rules:

- resource limits are Environment configuration, not Capabilities;
- provider-native Incus keys remain adapter details;
- requested-but-unsupported finite limits fail closed;
- finite limits are applied and verified before client/agent access;
- persist the effective creation-time budget;
- Base/image metadata cannot raise host-selected limits;
- v0.9/v0.10 agent integration cannot give coding agents authority to raise their own host-enforced limits;
- partial failure follows existing cleanup/recovery semantics.

## Planned v0.13 contracts

### Local OCI Registry

Read `13_v0.13_LOCAL_OCI_REGISTRY.md` before implementation.

The planned first slice must:

- provide a Hacocoon-owned local OCI registry/cache gateway;
- transparently route ordinary `containerd`/`nerdctl` image pulls through the trusted mediation path;
- keep reusable upstream credentials on the trusted side;
- fail closed instead of silently bypassing to arbitrary direct registries;
- keep OCI mechanics outside Core.

### OCI Seed & Btrfs/COW

Read `13A_v0.13_OCI_SEED_AND_COW.md` after the registry path exists.

Never:

- share one writable `/var/lib/containerd` between Environments;
- manipulate hidden Incus/Btrfs state behind the provider directly;
- open unrestricted Internet egress for a Seed Builder as a shortcut.

Seed identity must bind an immutable parent Base revision and immutable OCI digests. COW savings come from normal Incus/storage-driver cloning while every Environment retains independent logical containerd state.

## Implementation style

- Go remains the primary implementation language.
- Keep OS/Incus/process/client-launch side effects in narrow imperative adapter layers.
- Prefer simple values/functions in the core path.
- Introduce interfaces when they improve testing or a real second implementation needs the seam.
- Do not create cross-provider/client/image abstractions merely because two systems share a noun.
- Fail explicitly; cleanup and partial failure are part of the feature.
- Treat Environment workloads, image contents, backend output, filesystem state, client config, and external process output as potentially hostile.
- Do not expose long-lived host credentials or Hacocoon control authority inside an Environment.

## Breaking changes

Compatibility is not more important than a clear and safe architecture while Hacocoon is pre-1.0.

A breaking change is appropriate when it removes an unsafe contract, fixes ownership, reduces coupling, deletes unnecessary complexity, or repairs confusing roadmap status. Make incompatibility explicit, avoid silent data loss, preserve recovery where possible, and update tests/docs.

## Validation baseline

Run, as applicable:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
go build ./cmd/haco-agent-host
python tools/check_docs.py
```

Also run maintained process-boundary integration and E2E suites relevant to the changed subsystem.

Real Incus, networking/resource enforcement, Base/image lifecycle, Windows/WSL + desktop VS Code, Agent Host/AHP, and AWS/EC2/SSM/EBS acceptance require suitable external environments before being reported as passed.

## Stop conditions

Stop and revisit the design instead of improvising when a change would:

- expand Core with provider-specific or client-brand concepts;
- make Incus image aliases/fingerprints or OCI registry mechanics part of Core identity;
- weaken fail-closed policy/network/resource behavior;
- expose parent credentials to an Environment or image builder;
- silently destroy or abandon recoverable state or a referenced revision;
- make an experimental backend implicitly active;
- turn Hacocoon into an AI orchestrator, worktree manager, or IDE owner;
- assign a new roadmap version without reconciling `00D_VERSIONING_AND_RELEASE_STATUS.md` and current parallel work.
