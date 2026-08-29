# Codex Implementation Handoff

Status: implementation and maintenance guide for the rebaselined architecture after the v0.8 Client Adapters & VS Code Integration pass, with v0.9 Base Images & Custom Environments established as the next implementation gate.

## Current objective

Keep the existing v0.1-v0.8 implementation coherent while implementing the explicit v0.9 contract and while Hacocoon is still pre-1.0:

- harden security and failure behavior;
- close test and acceptance gaps;
- simplify accidental complexity;
- preserve architecture boundaries;
- make deliberate breaking changes when the current contract is unsafe or unnecessarily costly;
- keep client-specific convenience outside Core;
- implement v0.9 Base selection with immutable revision semantics without leaking Incus image mechanics into Core;
- avoid inventing post-v0.9 product scope without an explicit design decision.

`docs/IMPLEMENTATION_STATUS.md` records current repository reality. Versioned release specifications record the design contract for each roadmap stage. At the time of this handoff, implementation reality still reaches v0.8 while v0.9 is the next scheduled gate.

## Required execution order

For a change or bug fix:

1. Read `docs/IMPLEMENTATION_STATUS.md` and the relevant release/design document.
2. Inspect actual implementation and tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, Base, client, and failure boundaries.
4. Reproduce the bug or define intended behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise retry, cancellation, concurrency, partial failure, and cleanup paths when applicable.
7. Run unit, race, vet, build, docs, integration, and E2E checks available in the current environment.
8. Keep real-provider/client acceptance claims separate when Incus/Windows/VS Code/AWS infrastructure is unavailable.
9. Update `IMPLEMENTATION_STATUS.md` when code reality changes materially.

## Architecture placement

```text
Workspace identity/lease       -> Core concepts + workspace boundary
Base name/revision             -> Environment starting-point domain contract
Incus image mapping            -> runtime.incus adapter
Incus lifecycle                -> runtime.incus adapter
Client status/SSH/ports        -> generic client access boundary
VS Code SSH config/launch      -> haco-vscode Client Adapter, outside Core
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

Do not move provider-specific fields, `if ec2`, Incus alias/fingerprint details, IDE-brand behavior, or orchestrator-brand behavior into Core merely to make wiring convenient.

## v0.8 Client Adapter contract

The first adapter is `haco-vscode`.

The intended path is:

```text
Workspace
 -> create/reuse Hacocoon Environment
 -> request existing loopback-only SSH access
 -> write adapter-owned client SSH config
 -> launch standard VS Code Remote-SSH
 -> open /workspace
```

Rules:

- Private SSH key remains client-side; only the public key enters Hacocoon's SSH preparation path.
- Do not regenerate or own the user's entire SSH config; isolate Hacocoon-managed entries.
- In WSL + Windows desktop VS Code deployments, distinguish Linux host state from Windows client SSH state.
- Do not reimplement Remote-SSH.
- Do not add a Hacocoon AI chat UI; use the client's AI/coding-agent UX.
- Coding agents may have broad freedom inside the isolated Environment, but Host/GitHub/AWS authority remains behind Policy/Capability/Audit.
- Future client adapters should reuse generic Environment/client-access contracts instead of adding client brands to Core.

## v0.9 Base Images & Custom Environments contract

Read these before implementing the v0.9 feature:

1. `09_v0.9_BASE_IMAGES_AND_CUSTOM_ENVIRONMENTS.md` — authoritative minimum gate.
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

- Incus aliases/remotes/fingerprints are adapter details, not Core/public identity.
- A logical Base may move to a new revision, but an existing Environment must remain bound to the revision it recorded at creation.
- Prefer resolving/reusing existing Incus images first; do not start with a large image-build framework.
- A custom Base is untrusted guest content. It cannot grant devices, mounts, privileged-container mode, Linux capabilities, host networking, credentials, or external authority.
- If build/import is added, arbitrary build commands must not execute directly with host authority.
- Local archives must be treated as hostile input, including traversal, symlink, metadata, size/resource, and partial-failure concerns.
- Deletion/GC must not physically remove a revision still referenced by a running or recoverable Environment.
- Concurrent create/update/remove/GC operations must use explicit synchronization/reference semantics rather than timing assumptions.
- Recreate an Environment to move to another Base revision; do not mutate the starting Base of an existing Environment in place.

Do not implement every idea in `BASE_IMAGES.md` just because it is described there. The first v0.9 slice should satisfy the minimum versioned acceptance contract before build/history/rollback/GC convenience is expanded.

## Implementation style

- Go remains the primary implementation language.
- Keep OS/Incus/process/client-launch side effects in narrow imperative adaptor layers.
- Prefer simple values/functions in the core path.
- Introduce an interface when it improves testing or a real second implementation needs the seam.
- Do not create a cross-provider/client/image abstraction merely because two systems share a noun.
- Fail explicitly; cleanup and partial failure are part of the feature.
- Treat Environment workloads, Base/image contents, backend output, filesystem state, client config, and external process output as potentially hostile.
- Do not expose long-lived host credentials or Hacocoon control authority inside an Environment.

## Breaking changes

Hacocoon is pre-1.0. Compatibility is not more important than a clear and safe architecture.

A breaking change is appropriate when it removes an unsafe contract, fixes ownership, reduces coupling, or deletes unnecessary complexity. Make the incompatibility explicit, avoid silent data loss, preserve recovery where possible, update tests/docs, and add migration guidance only when a migration is actually supported.

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

Real Incus acceptance requires a supported Incus host. Real Base/image lifecycle acceptance that depends on Incus must also run on a suitable Incus host before being reported as passed. Real Windows/WSL + desktop VS Code Remote-SSH acceptance requires that client environment. Real AWS/EC2/SSM/EBS acceptance requires suitable AWS infrastructure. Do not report those as passed based only on mocks, fake CLIs, or local unit tests.

## Stop conditions

Stop and escalate the design instead of improvising when a change would:

- expand Core with provider-specific or client-brand concepts;
- make Incus image aliases/fingerprints part of the Core Base identity;
- weaken fail-closed policy behavior;
- expose parent credentials to an Environment or image builder;
- silently destroy or abandon recoverable state or a referenced Base revision;
- make an experimental backend implicitly active;
- turn Hacocoon into an AI orchestrator, worktree manager, or IDE owner;
- create a post-v0.9 product direction without an explicit contract.
