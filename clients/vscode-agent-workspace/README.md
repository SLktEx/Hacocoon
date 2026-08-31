# Hacocoon Agent Workspaces for VS Code

This UI-side VS Code extension creates one linked Git worktree and one Hacocoon Environment for an independently writable top-level coding-agent session.

## Commands

- `Hacocoon: New Agent Workspace`
- `Hacocoon: Release Agent Workspace`

On Windows the intended flow is:

```text
Windows VS Code
  -> Hacocoon: New Agent Workspace
  -> wsl.exe -d Hacocoon -- git ...
  -> linked Git worktree
  -> wsl.exe -d Hacocoon -- haco-agent-host prepare --session <opaque-id> --json --no-launch <worktree>
  -> Windows-side managed Remote-SSH alias
  -> vscode.openFolder(vscode-remote://ssh-remote+haco-agent-.../workspace)
```

No shell command string is constructed. `wsl.exe`, Git, and `haco-agent-host` are invoked with explicit argument arrays.

After the remote workspace opens, start the desired Copilot, Claude, Codex, or other supported harness from the VS Code Agents window. Hacocoon does not use private/undocumented VS Code command IDs to select or create an Agent Host session.

## Agent-invoked orchestration

The same trusted provisioning backend is also exposed through VS Code's public Language Model Tool API:

- `hacocoon_createAgentWorkspace`
- `hacocoon_listAgentWorkspaces`
- `hacocoon_releaseAgentWorkspace`

This lets a parent VS Code agent allocate isolated workspaces for parallel work without asking the user to run Hacocoon commands manually.

Conceptually:

```text
VS Code parent agent
  -> create Hacocoon workspace tool
  -> linked Git worktree
  -> isolated Hacocoon Environment
  -> Remote-SSH /workspace
```

`create` requires a new Git branch name and accepts `open=true` when the returned Remote-SSH workspace should be opened immediately. It defaults to not opening a new window so one parent agent can provision several workspaces in a batch. `list` returns safe orchestration metadata. `release` selects an exact VS Code-owned branch and reuses the same fail-closed cleanup path as the UI command.

Create and release use VS Code tool confirmation messages because they mutate host-side Git/Hacocoon state. List is read-only.

Language-model-visible results intentionally omit the raw Hacocoon session identity, internal ownership token, managed SSH alias, and private-key/credential material. The tool also does not start Codex/Claude/Copilot CLI inside the Environment or copy reusable provider credentials there.

## Repository path

The repository path passed to Git is an absolute Linux path in the Hacocoon execution host.

On Windows the extension uses the configured WSL distro, `Hacocoon` by default. If the current VS Code workspace is already a matching Remote-WSL workspace, its Linux path is reused. Otherwise set `hacocoon.agentWorkspace.repositoryPath` or enter the Linux path when prompted.

On Linux, the extension can operate directly on a local `file:` workspace without a WSL wrapper.

The current trusted-host architecture still has an open seam around the long-term physical placement/export of repositories that logically live in `haco-host`. This extension does not pretend that seam is solved: the repository must currently be visible to the execution host that runs Git and `haco-agent-host`. It also does not expose raw Incus or Hacocoon control authority to ordinary coding-agent Environments.

## Why the extension is UI-side

`haco-agent-host` owns the client-side Remote-SSH setup. When Windows VS Code invokes it through `wsl.exe`, the existing Hacocoon WSL interop path can manage the Windows user's SSH configuration while Hacocoon/Incus operations still execute inside the dedicated WSL distribution.

This avoids running the extension inside the final untrusted coding-agent Environment merely to allocate that same Environment.

## Worktree ownership and cleanup

By default worktrees are created under a `.hacocoon-worktrees/<repo>` directory beside the main checkout. Override this with an absolute Linux `hacocoon.agentWorkspace.worktreeRoot`.

Release is fail-closed:

1. `haco-agent-host release --session <id>` runs first.
2. The extension refuses to remove the main checkout or a path outside the recorded Hacocoon-owned worktree root.
3. A dirty worktree is retained even after the Environment is released.
4. A clean linked worktree may be removed with `git worktree remove`.
5. The Git branch is never deleted automatically, preserving committed agent work.

If Environment preparation fails immediately after a new worktree is created, only that newly-created worktree is force-removed as rollback.

## Settings

- `hacocoon.agentWorkspace.gitExecutable` — default `git`
- `hacocoon.agentWorkspace.adapterExecutable` — default `haco-agent-host`
- `hacocoon.agentWorkspace.worktreeRoot` — optional absolute Linux worktree root
- `hacocoon.agentWorkspace.wslDistro` — default `Hacocoon`
- `hacocoon.agentWorkspace.repositoryPath` — optional absolute Linux repository path in that distro

## VS Code / Agent Host limitation

VS Code's Agents window can start sessions for remote SSH workspaces and choose among available harnesses. As of 2026-08-31, the stable public extension surface does not provide Hacocoon with a supported full Agent Host provider-registration hook to replace/intercept the built-in `New` lifecycle. The upstream request is `microsoft/vscode#325827`, tracked in Hacocoon by #365.

Therefore the supported implementation has two entry points sharing one backend:

1. a human uses `Hacocoon: New Agent Workspace`; or
2. a parent VS Code agent invokes the Hacocoon Language Model Tools.

Both provision the isolated workspace through supported APIs. The built-in Agents window still owns actual Copilot/Claude/Codex session creation. When VS Code exposes a stable native provider lifecycle hook, #365 can connect that hook to the same provisioning backend without moving AHP or model scheduling into Hacocoon Core.
