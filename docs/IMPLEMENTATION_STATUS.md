# Implementation Status

Status date: 2026-08-29, after the Secure Workspace Runtime v0.1 implementation pass.

This file reports **current code reality**, not desired architecture. The current release specification is `01_v0.1_SECURE_WORKSPACE_RUNTIME.md`.

The repository still contains historical code from the previous roadmap. Existing code does not automatically belong to the new v0.1 gate.

| Area | Current repository reality | Target | Rebaseline action |
|---|---|---:|---|
| Go CLI | public path is `create`, `exec`, `shell`, `delete`, plus `doctor`; legacy command implementations may remain in historical packages | v0.1 | implemented for the current vertical slice |
| Core model | minimal `Workspace`, `Environment`, and `ExecutionResult` values plus a focused Environment lifecycle service exist; legacy `Session`, Runtime, Storage, Manager code still remains | v0.1 migration | new public path uses Workspace/Environment/Execution; do not deepen legacy architecture |
| Incus CLI adapter | concrete Incus Environment path exists | v0.1 | creates an Incus system container, mounts only the requested workspace, starts it, executes commands/shell, and deletes it |
| arbitrary command execution | implemented on the Environment path | v0.1 | unit and process-boundary integration cover argv, exit status, stdout, and stderr; supported-host Incus run still pending |
| interactive shell | implemented on the Environment path | v0.1 | process-boundary integration passes; supported-host Incus shell acceptance still pending |
| external workspace mount | implemented as a read/write Incus disk device at `/workspace` | v0.1 | unit and process-boundary integration verify host-to-Environment reads and Environment-to-host writes |
| `haco create --workspace` | implemented | v0.1 | validates/canonicalizes the host directory, creates the Environment, persists minimal metadata, and cleans partial failures |
| `haco delete` | implemented | v0.1 | deletes the runtime resource before metadata and retains metadata when runtime deletion fails |
| Environment metadata | JSON store under `HACO_ROOT/state/environments.json` | v0.1 | implemented with restricted permissions and temp-file rename |
| process-boundary integration | real child-process boundary using an `incus` executable shim on `PATH` | v0.1 | implemented and passing in ordinary test runs |
| real Incus vertical-slice integration test | opt-in Go acceptance test and CLI E2E script exist behind `HACO_E2E_INCUS=1` | v0.1 | test path implemented; actual supported-host pass remains pending because the current sandbox has no Incus daemon/CLI |
| security acceptance checks | E2E asserts requested workspace exposure and checks common host credential stores / Incus control socket are not mounted | v0.1 | automated test exists; supported-host verification pending |
| storage abstraction | extensive historical implementation exists | later/provider detail | detached from the new public v0.1 path |
| Btrfs grow/shrink/compact | historical code exists | uncommitted future detail | not a v0.1 requirement |
| raw/QCOW2 backing code | historical code exists | no current commitment | not used by the new public v0.1 path |
| crash/partial-failure cleanup | legacy reconciliation exists; new Environment path has explicit create/persist cleanup and delete ordering | v0.1 only as required | deterministic paths are unit-tested; real-host failure recovery remains part of integration acceptance |
| WorkspaceProvider / WorkspaceLease | not implemented | v0.2 | deferred until v0.1 passes |
| VS Code/client connection layer | not implemented | v0.3 | deferred |
| Policy/Capability foundation | not implemented | v0.4 | deferred |
| Git/GitHub capability | not implemented | v0.5 | deferred |
| Codex/Claude/Daintree/Rookery integration | not implemented | v0.6 | deferred; orchestration remains external |
| AWS/EC2/EBS | not implemented | v0.7 | deferred |

## Current release gate

The v0.1 implementation path now exists:

```text
external workspace -> Incus Environment -> exec/shell -> delete/cleanup
```

Automated unit and process-boundary integration tests pass. The remaining v0.1 acceptance item is a successful run of the opt-in tests on a supported real Incus host, including the real interactive-shell check. Do not tag v0.1 alpha until that supported-host acceptance passes.

Future-release documents are planning references and must not be used to justify adding scope to v0.1.
