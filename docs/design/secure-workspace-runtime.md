# v0.1 — Secure Workspace Runtime MVP

Status: **baseline roadmap contract; implementation exists on `main`.** Real Incus acceptance remains environment-dependent. This document records the v0.1 gate and does not freeze the current pre-1.0 CLI/API/state surface.

## Goal

Prove the smallest useful Hacocoon path end-to-end:

```text
external workspace
  -> Incus system container
  -> execute / interactive shell
  -> cleanup
```

v0.1 is complete when a host directory can be mounted read/write into an Incus Environment, commands can run against it, an interactive shell can be opened, and the Environment can be deleted cleanly.

## Public CLI target

```text
haco create --workspace <path> <environment>
haco exec <environment> -- <command...>
haco shell <environment>
haco delete <environment>
```

Optional diagnostic commands are allowed only when they directly support this path and do not expand the architecture.

## In scope

- Go CLI `haco`.
- External workspace path resolution.
- Incus system-container creation.
- Read/write workspace mount.
- Environment identification and minimal persisted metadata.
- Command execution with exit code/stdout/stderr propagation.
- Interactive shell.
- Environment deletion and metadata cleanup.
- Unit tests for deterministic logic.
- Real-Incus integration test for the vertical slice.

## Explicitly out of scope

- Hacocoon-owned Git worktree creation.
- Git/GitHub capability mediation.
- AWS, EC2, EBS, cloud runtime.
- Policy engine and approval workflow.
- MCP.
- AI-specific agent management.
- Daintree/Rookery-specific integration.
- VS Code extension, Web UI, browser notifications.
- Advanced Btrfs lifecycle, loop-image shrink, QCOW2, storage compaction.
- A general plugin marketplace/framework.

These exclusions define what was required to prove v0.1. Later roadmap stages may implement these concerns without retroactively expanding the v0.1 gate.

## Implementation order

### Step 1 — Minimal domain

Define only what the vertical slice needs, typically:

```text
Workspace
Environment
ExecutionResult
```

Avoid speculative interfaces.

### Step 2 — Thin Incus adapter

Wrap the required `incus` CLI operations. Prefer a narrow process boundary before introducing a heavy client library.

### Step 3 — `haco create`

- validate workspace path;
- create/start Incus system container;
- mount workspace at a predictable path;
- persist only the metadata required to find/delete the Environment;
- return the environment identifier.

### Step 4 — `haco exec`

- run arbitrary command in the Environment;
- preserve argument boundaries;
- return command exit code and streams correctly.

### Step 5 — `haco shell`

Open an interactive shell without turning Hacocoon into a terminal emulator.

### Step 6 — `haco delete`

Stop/delete the Incus Environment as needed and remove Hacocoon metadata. Repeated cleanup should be safe or produce an explicit, understandable result.

### Step 7 — Integration acceptance

On a supported Incus host:

1. create an Environment from a test workspace;
2. read a host-created file inside the Environment;
3. write a file inside the Environment and observe it on the host;
4. run a command and verify exit/output behavior;
5. verify interactive shell connectivity manually or with the narrowest practical automated check;
6. delete the Environment;
7. verify no Hacocoon-owned runtime resource remains unintentionally.

## Security acceptance

- Only the requested workspace is mounted for development data.
- Host HOME and common credential stores are not mounted.
- Incus authority remains host-side.
- Failure paths do not silently broaden access.

## Gate record

This scope was intentionally frozen while v0.1 was being established so later abstractions could not delay the first vertical slice. The repository has since progressed through later roadmap stages; this section is historical acceptance context, not an instruction to remove v0.2-v0.7 functionality.
