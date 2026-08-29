# Codex Implementation Handoff

Status: implementation and maintenance guide for the rebaselined architecture after the v0.7 implementation pass.

## Current objective

Keep the existing v0.1-v0.7 implementation coherent while Hacocoon is still pre-1.0:

- harden security and failure behavior;
- close test and acceptance gaps;
- simplify accidental complexity;
- preserve the architecture boundaries;
- make deliberate breaking changes when the current contract is unsafe or unnecessarily costly;
- avoid inventing post-v0.7 product scope without an explicit design decision.

`docs/IMPLEMENTATION_STATUS.md` records current repository reality. The versioned release specifications record the design contract for each roadmap stage.

## Required execution order

For a change or bug fix:

1. Read `docs/IMPLEMENTATION_STATUS.md` and the relevant release/design document.
2. Inspect the actual implementation and tests; do not assume documentation proves the behavior.
3. Identify the authority, ownership, lifecycle, and failure boundaries involved.
4. Reproduce the bug or define the intended behavior with a test where practical.
5. Implement the smallest coherent change.
6. Exercise retry, cancellation, concurrency, partial failure, and cleanup paths when applicable.
7. Run unit, race, vet, build, docs, integration, and E2E checks that are available in the current environment.
8. Keep real-provider acceptance claims separate when Incus/AWS infrastructure is unavailable.
9. Update `IMPLEMENTATION_STATUS.md` when code reality changes materially.

## Architecture placement

Use this placement unless an explicit architecture decision supersedes it:

```text
Workspace identity/lease       -> Core concepts + workspace boundary
Incus lifecycle                -> runtime.incus adapter
Client status/SSH/ports        -> client access boundary
Policy/approval/audit          -> Policy + Capability foundation
Git/GitHub authority           -> Git/GitHub capability boundary
Agent execution                -> generic execution; orchestration stays external
AWS external authority         -> aws.api capability boundary
EC2 lifecycle                  -> runtime.ec2, experimental/default-off
EBS replacement mechanics      -> storage.ebs adapter
Btrfs/QCOW2/storage mechanics  -> provider/adapter detail only when justified
```

Do not move provider-specific fields or `if ec2` / `if git` / IDE-brand behavior into Core merely to make wiring convenient.

## Implementation style

- Go remains the primary implementation language.
- Keep OS/Incus/process side effects in a narrow imperative shell/adaptor layer.
- Prefer simple values/functions in the core path.
- Introduce an interface when it improves testing or a real second implementation needs the seam.
- Do not create a cross-provider abstraction merely because two systems share a noun.
- Fail explicitly; cleanup and partial failure are part of the feature.
- Treat Environment workloads, backend output, filesystem state, and external process output as potentially hostile.
- Do not expose long-lived host credentials or Hacocoon control authority inside an Environment.

## Breaking changes

Hacocoon is pre-1.0. Compatibility is not more important than a clear and safe architecture.

A breaking change is appropriate when it removes an unsafe contract, fixes ownership, reduces coupling, or deletes unnecessary complexity. When making one:

- make the incompatibility explicit;
- avoid silent data loss;
- preserve recovery paths where possible;
- update tests and all affected docs;
- add migration guidance only when a migration is actually supported;
- do not maintain a shadow legacy path indefinitely just to avoid a rename or redesign.

## Validation baseline

Run, as applicable:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
python tools/check_docs.py
```

Also run maintained process-boundary integration and E2E suites relevant to the changed subsystem.

Real Incus acceptance requires a supported Incus host. Real AWS/EC2/SSM/EBS acceptance requires suitable AWS infrastructure. Do not report those as passed based only on mocks, fake CLIs, or local unit tests.

## Stop conditions

Stop and escalate the design instead of improvising when a change would:

- expand Core with provider-specific concepts;
- weaken fail-closed policy behavior;
- expose parent credentials to an Environment;
- silently destroy or abandon recoverable state;
- make an experimental backend implicitly active;
- create a new major responsibility such as AI orchestration or IDE ownership;
- introduce a post-v0.7 product direction without an explicit contract.
