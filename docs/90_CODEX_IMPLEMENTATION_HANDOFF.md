# Codex Implementation Handoff

Status: implementation and maintenance guide for the rebaselined architecture after the v0.8 Client Adapters & VS Code Integration pass.

## Current objective

Keep the existing v0.1-v0.8 implementation coherent while Hacocoon is still pre-1.0:

- harden security and failure behavior;
- close test and acceptance gaps;
- simplify accidental complexity;
- preserve architecture boundaries;
- make deliberate breaking changes when the current contract is unsafe or unnecessarily costly;
- keep client-specific convenience outside Core;
- avoid inventing post-v0.8 product scope without an explicit design decision.

`docs/IMPLEMENTATION_STATUS.md` records current repository reality. Versioned release specifications record the design contract for each roadmap stage.

## Required execution order

For a change or bug fix:

1. Read `docs/IMPLEMENTATION_STATUS.md` and the relevant release/design document.
2. Inspect actual implementation and tests; documentation is not proof of behavior.
3. Identify authority, ownership, lifecycle, client, and failure boundaries.
4. Reproduce the bug or define intended behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise retry, cancellation, concurrency, partial failure, and cleanup paths when applicable.
7. Run unit, race, vet, build, docs, integration, and E2E checks available in the current environment.
8. Keep real-provider/client acceptance claims separate when Incus/Windows/VS Code/AWS infrastructure is unavailable.
9. Update `IMPLEMENTATION_STATUS.md` when code reality changes materially.

## Architecture placement

```text
Workspace identity/lease       -> Core concepts + workspace boundary
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

Do not move provider-specific fields, `if ec2`, IDE-brand behavior, or orchestrator-brand behavior into Core merely to make wiring convenient.

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

## Implementation style

- Go remains the primary implementation language.
- Keep OS/Incus/process/client-launch side effects in narrow imperative adaptor layers.
- Prefer simple values/functions in the core path.
- Introduce an interface when it improves testing or a real second implementation needs the seam.
- Do not create a cross-provider/client abstraction merely because two systems share a noun.
- Fail explicitly; cleanup and partial failure are part of the feature.
- Treat Environment workloads, backend output, filesystem state, client config, and external process output as potentially hostile.
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

Real Incus acceptance requires a supported Incus host. Real Windows/WSL + desktop VS Code Remote-SSH acceptance requires that client environment. Real AWS/EC2/SSM/EBS acceptance requires suitable AWS infrastructure. Do not report those as passed based only on mocks, fake CLIs, or local unit tests.

## Stop conditions

Stop and escalate the design instead of improvising when a change would:

- expand Core with provider-specific or client-brand concepts;
- weaken fail-closed policy behavior;
- expose parent credentials to an Environment;
- silently destroy or abandon recoverable state;
- make an experimental backend implicitly active;
- turn Hacocoon into an AI orchestrator, worktree manager, or IDE owner;
- create a post-v0.8 product direction without an explicit contract.
