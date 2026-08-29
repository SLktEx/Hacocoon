# Codex Implementation Handoff

Status: implementation guide for the rebaselined roadmap.

## Current objective

Finish **v0.1 Secure Workspace Runtime MVP** before extending later releases.

The repository contains historical code from a broader design. Do not equate existing code with current scope.

## Required execution order

1. Inventory the current CLI, Core types, Incus adapter, storage code, and tests.
2. Map each piece to `docs/IMPLEMENTATION_STATUS.md`.
3. Establish the minimal Workspace -> Incus Environment -> Execution vertical slice.
4. Implement/fix `haco create --workspace`.
5. Implement/fix `haco exec`.
6. Implement/fix `haco shell`.
7. Implement/fix `haco delete` and cleanup.
8. Add/repair real-Incus integration tests.
9. Stop at the v0.1 acceptance gate.

## Preserve vs defer

Preserve useful historical code, but do not wire later-release behavior into v0.1.

Typical disposition:

```text
Incus lifecycle        -> keep/adapt for v0.1
Workspace mount        -> implement now
Command execution      -> keep/adapt for v0.1
Storage shrink/QCOW2   -> defer behind later adapter boundary
Git/worktree ownership -> defer to v0.2
VS Code                -> v0.3 docs/integration
Policy/approval        -> v0.4
GitHub authority       -> v0.5
Agent orchestration    -> external; integration in v0.6
AWS/EC2/EBS            -> v0.7
```

## Implementation style

- Go remains the primary implementation language.
- Keep OS/Incus/process side effects in a narrow imperative shell/adaptor layer.
- Prefer simple values/functions in the core path.
- Do not introduce an interface until it improves testing or a second implementation needs the seam.
- Do not create a cross-provider generic abstraction merely because two future systems sound similar.
- Fail explicitly; cleanup and partial failure are part of the feature.

## v0.1 acceptance checklist

- [ ] `go test ./...` passes for the maintained codebase.
- [ ] `go build ./cmd/haco` passes.
- [ ] create succeeds on a supported Incus host.
- [ ] requested workspace is mounted read/write.
- [ ] host-created file can be read inside.
- [ ] environment-created file is visible on host.
- [ ] command exit status and output are correct.
- [ ] interactive shell works.
- [ ] delete removes the environment.
- [ ] failed/partial operations do not leave unexplained resources.
- [ ] host HOME/credentials/Incus control socket are not exposed as shortcuts.

After this checklist is satisfied, stop and report the v0.1 result before starting v0.2.
