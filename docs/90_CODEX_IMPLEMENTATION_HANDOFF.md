# Codex Implementation Handoff

Status: implementation and maintenance guide for the current pre-1.0 architecture. Implemented milestones are contiguous through v0.9; v0.10 is the active VS Code Remote Agent Host Adapter integration; v0.11 Base Images and v0.12 Resource Limits remain design-only.

## Current objective

Keep the existing v0.1-v0.9 implementation coherent while Hacocoon is still pre-1.0:

- harden security and failure behavior;
- close test and acceptance gaps;
- simplify accidental complexity;
- preserve architecture boundaries;
- make deliberate breaking changes when the current contract is unsafe or unnecessarily costly;
- keep client-specific convenience outside Core;
- keep roadmap numbering aligned with implementation reality;
- do not report design-only gates as implemented.

`docs/IMPLEMENTATION_STATUS.md` records current repository reality. `docs/00D_VERSIONING_AND_RELEASE_STATUS.md` owns the current version numbering.

## Required execution order

For a change or bug fix:

1. Read `docs/IMPLEMENTATION_STATUS.md`, `docs/00D_VERSIONING_AND_RELEASE_STATUS.md`, and the relevant release/design document.
2. Inspect actual implementation and tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, agent binding, Base, resource, client, and failure boundaries.
4. Reproduce the bug or define intended behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise retry, cancellation, concurrency, partial failure, and cleanup paths when applicable.
7. Run unit, race, vet, build, docs, integration, and E2E checks available in the current environment.
8. Keep real-provider/client acceptance claims separate when Incus/Windows/VS Code/AWS infrastructure is unavailable.
9. Update `IMPLEMENTATION_STATUS.md` when code reality changes materially.

## Architecture placement

```text
Workspace identity/lease       -> Core concepts + workspace boundary
Per-agent session binding      -> trusted integration layer outside Core
VS Code Agent Host adapter     -> trusted client adapter boundary, outside Core
Base name/revision             -> Environment starting-point domain contract
ResourceBudget                 -> provider-neutral Environment configuration
Incus image mapping            -> runtime.incus adapter
Incus lifecycle                -> runtime.incus adapter
Client status/SSH/ports        -> generic client access boundary
VS Code SSH config/launch      -> client adapter, outside Core
IDE/editor/AI chat UX          -> client responsibility
Daintree/worktree/task DAG     -> external orchestrator responsibility
Policy/approval/audit          -> Policy + Capability foundation
Git/GitHub authority           -> Git/GitHub capability boundary
Agent execution                -> generic execution; orchestration stays external
AWS external authority         -> aws.api capability boundary
EC2 lifecycle                  -> runtime.ec2, experimental/default-off
EBS replacement mechanics      -> storage.ebs adapter
Btrfs/QCOW2/storage mechanics  -> provider/adapter detail only when justified
```

Do not move provider-specific fields, `if ec2`, Incus alias/fingerprint details, IDE-brand behavior, AHP details, or orchestrator-brand behavior into Core merely to make wiring convenient.

## v0.8 Client Adapter contract

The first adapter is `haco-vscode`.

```text
Workspace
 -> create/reuse Hacocoon Environment
 -> request existing loopback-only SSH access
 -> write adapter-owned client SSH config
 -> launch standard VS Code Remote-SSH
 -> open /workspace
```

Rules:

- private SSH key remains client-side;
- do not own the user's entire SSH config;
- distinguish Windows client SSH state from WSL host state;
- do not reimplement Remote-SSH;
- do not add a Hacocoon AI chat UI;
- host/GitHub/AWS authority remains behind Policy/Capability/Audit;
- future adapters reuse generic Environment/client-access contracts rather than adding client brands to Core.

## v0.9 Per-Agent Sandbox contract

Read `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md` before changing `internal/agenthost`.

The implemented broker:

```text
opaque external session identity
        |
trusted persisted binding
        |
Environment lifecycle + WorkspaceLease
        |
Environment
```

Required rules:

- coding agents do not receive Hacocoon/Incus management authority;
- raw session IDs are not runtime names or ownership proof;
- exact reacquire is idempotent;
- Workspace/access-mode rebinding fails closed;
- release requires persisted binding proof;
- deterministic Environment-name coincidence is not enough to adopt/delete an Environment;
- parallel RW sessions normally use distinct canonical Workspace paths/worktrees;
- real Agent Host/AHP routing acceptance is separate from the broker foundation.

## v0.10 Remote Agent Host Adapter contract

v0.10 is the active integration candidate in PR #111 and is not current `main` code until merged.

The adapter may prepare a v0.9-bound Environment as a loopback-only SSH target for the VS Code Agents window.

Rules:

- the client keeps the private SSH key;
- only the public key enters the existing Hacocoon SSH preparation path;
- reuse compatible managed connections and rotate changed connections safely;
- adapter setup failure must not silently release a pre-existing v0.9 Environment;
- VS Code owns Agent Host/AHP behavior;
- Hacocoon owns trusted Environment selection and safe connection preparation;
- real Windows/WSL + Incus + current VS Code acceptance remains environment-dependent.

## v0.11 Base Images & Custom Environments contract

Read:

1. `11_v0.11_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` — authoritative minimum gate.
2. `BASE_IMAGES.md` — detailed companion design.
3. `00_REBASELINE_AND_ROADMAP.md` — architecture placement.
4. `00B_SECURITY_ARCHITECTURE.md` — trust-boundary rules.
5. `IMPLEMENTATION_STATUS.md` — what is actually implemented today.

The initial implementation should stay narrow:

```text
logical Base name
  -> resolve immutable Base revision
  -> map to existing Incus image fingerprint internally
  -> create Environment
  -> persist Base name + immutable revision
```

Required rules:

- Incus aliases/remotes/fingerprints are adapter details, not Core/public identity;
- existing Environments remain bound to their creation-time Base revision;
- prefer resolving/reusing existing Incus images before building a broad image framework;
- custom Base contents are untrusted guest data and cannot grant host authority;
- build/import code must not execute directly with host authority;
- local archives are hostile input;
- deletion/GC must preserve referenced revisions;
- create/update/remove/GC races need explicit synchronization/reference semantics;
- moving to another Base revision requires a new/recreated Environment rather than mutating the existing root starting point.

Do not implement every idea in `BASE_IMAGES.md` merely because it is described there.

## v0.12 Sandbox Resource Limits contract

Read `12_v0.12_SANDBOX_RESOURCE_LIMITS.md` before implementing resource budgets.

The first gate covers provider-neutral creation-time CPU, memory, PID/process, and safely enforceable root-storage limits.

Required rules:

- resource limits are Environment configuration, not Capabilities;
- provider-native Incus keys remain adapter details;
- requested-but-unsupported limits fail closed;
- apply limits before client/agent access;
- persist the effective creation-time budget;
- Base/image metadata cannot raise host-selected limits;
- v0.9/v0.10 agent integration cannot give coding agents authority to raise their own host-enforced limits;
- partial failure follows existing cleanup/recovery semantics.

## Implementation style

- Go remains the primary implementation language.
- Keep OS/Incus/process/client-launch side effects in narrow imperative adapter layers.
- Prefer simple values/functions in the core path.
- Introduce an interface when it improves testing or a real second implementation needs the seam.
- Do not create a cross-provider/client/image abstraction merely because two systems share a noun.
- Fail explicitly; cleanup and partial failure are part of the feature.
- Treat Environment workloads, image contents, backend output, filesystem state, client config, and external process output as potentially hostile.
- Do not expose long-lived host credentials or Hacocoon control authority inside an Environment.

## Breaking changes

Hacocoon is pre-1.0. Compatibility is not more important than a clear and safe architecture.

A breaking change is appropriate when it removes an unsafe contract, fixes ownership, reduces coupling, deletes unnecessary complexity, or repairs confusing roadmap numbering. Make incompatibility explicit, avoid silent data loss, preserve recovery where possible, and update tests/docs.

## Validation baseline

Run, as applicable:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
go build ./cmd/haco-vscode
python tools/check_docs.py
```

Also run maintained process-boundary integration and E2E suites relevant to the changed subsystem.

Real Incus, Base/image lifecycle, Windows/WSL + desktop VS Code, Agent Host/AHP, resource enforcement, and AWS/EC2/SSM/EBS acceptance require suitable external environments before being reported as passed.

## Stop conditions

Stop and escalate the design instead of improvising when a change would:

- expand Core with provider-specific or client-brand concepts;
- make Incus image aliases/fingerprints part of the Core Base identity;
- weaken fail-closed policy behavior;
- expose parent credentials to an Environment or image builder;
- silently destroy or abandon recoverable state or a referenced Base revision;
- make an experimental backend implicitly active;
- turn Hacocoon into an AI orchestrator, worktree manager, or IDE owner;
- assign a new roadmap version without reconciling `00D_VERSIONING_AND_RELEASE_STATUS.md` and active parallel work.
