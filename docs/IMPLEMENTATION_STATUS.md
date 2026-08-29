# Implementation Status

Status date: 2026-08-29 rebaseline.

The repository already contains code from the previous roadmap. The table below classifies it against the **new** roadmap; it is not a claim that v0.1 is complete.

| Area | Existing code | New target | Rebaseline action |
|---|---:|---:|---|
| Go CLI | yes | v0.1 | narrow public path to create/exec/shell/delete |
| Core Session/Manager model | yes | v0.1 migration | reuse useful lifecycle logic; migrate concepts toward Workspace/Environment where helpful |
| Incus CLI adapter | yes | v0.1 | retain and simplify around Environment lifecycle |
| arbitrary command execution | partial/yes | v0.1 | validate result/exit semantics end-to-end |
| interactive shell | existing/partial | v0.1 | validate against target Environment path |
| external workspace mount | incomplete/new target | v0.1 | make this a first-class acceptance requirement |
| real Incus vertical-slice integration test | incomplete | v0.1 | required before v0.1 alpha |
| storage abstraction | yes | later/provider detail | keep only what v0.1 actually requires; detach advanced behavior from gate |
| Btrfs grow/shrink/compact | yes/partial | later | defer; not v0.1 acceptance |
| raw/QCOW2 backing code | yes | later/optional | isolate or remove if obsolete; never require for v0.1 |
| crash reconciliation | partial | v0.1 if needed | keep only lifecycle recovery needed for create/delete correctness |
| Git/worktree workspace ownership | no/limited | v0.2 | implement only after v0.1 |
| WorkspaceLease | no | v0.2 | deferred |
| VS Code/interactive client integration | no | v0.3 | deferred |
| Policy/Capability foundation | no | v0.4 | deferred |
| Git/GitHub capability | no | v0.5 | deferred |
| Codex/Claude/Daintree/Rookery integration | no | v0.6 | deferred; orchestration remains external |
| AWS/EC2/EBS | no | v0.7 | deferred |

## Current release gate

The only current release gate is `docs/01_v0.1_SECURE_WORKSPACE_RUNTIME.md`.
