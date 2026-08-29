# Implementation Status

Status date: 2026-08-29, after Secure Workspace Runtime rebaseline.

This file reports **current code reality**, not desired architecture. The current release specification is `01_v0.1_SECURE_WORKSPACE_RUNTIME.md`.

The repository still contains substantial code from the previous roadmap. Existing code does not automatically belong to the new v0.1 gate.

| Area | Current repository reality | Target | Rebaseline action |
|---|---|---:|---|
| Go CLI | legacy commands exist: `init`, `doctor`, `new`, `list`, `status`, `start`, `stop`, `rm`, `exec`, `shell`, `storage` | v0.1 | converge the public v0.1 path on `create` / `exec` / `shell` / `delete`; keep diagnostics only if useful |
| Core model | `Session`, Runtime, Storage, Manager and SessionStore are still central | v0.1 migration | preserve useful lifecycle logic but do not deepen legacy public architecture |
| Incus CLI adapter | exists | v0.1 | retain/simplify around Environment lifecycle |
| arbitrary command execution | exists in legacy Session path | v0.1 | validate exit/output semantics on the new vertical slice |
| interactive shell | exists in legacy Session path | v0.1 | validate on the new Environment path |
| external workspace mount | not yet the public v0.1 create path | v0.1 | make requested host directory mount a first-class acceptance requirement |
| `haco create --workspace` | not implemented as the target public command | v0.1 | implement |
| `haco delete` | legacy `rm` exists; target command not implemented | v0.1 | implement/rename around Environment cleanup semantics |
| real Incus vertical-slice integration test | incomplete | v0.1 | required before v0.1 alpha |
| storage abstraction | extensive historical implementation exists | later/provider detail | detach from v0.1 path unless strictly needed by Incus lifecycle |
| Btrfs grow/shrink/compact | historical code exists | uncommitted future detail | do not treat as roadmap requirement without a new release need/ADR |
| raw/QCOW2 backing code | historical code exists | no current commitment | isolate/remove when proven obsolete; never require for v0.1 |
| crash reconciliation | partial historical implementation | v0.1 only if required | retain only lifecycle recovery needed for create/delete correctness |
| WorkspaceProvider / WorkspaceLease | not implemented | v0.2 | deferred until v0.1 passes |
| VS Code/client connection layer | not implemented | v0.3 | deferred |
| Policy/Capability foundation | not implemented | v0.4 | deferred |
| Git/GitHub capability | not implemented | v0.5 | deferred |
| Codex/Claude/Daintree/Rookery integration | not implemented | v0.6 | deferred; orchestration remains external |
| AWS/EC2/EBS | not implemented | v0.7 | deferred |

## Current release gate

Only the v0.1 vertical slice is an implementation gate:

```text
external workspace -> Incus Environment -> exec/shell -> delete/cleanup
```

Future-release documents are planning references and must not be used to justify adding scope to v0.1.
