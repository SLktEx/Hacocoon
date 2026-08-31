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

VS Code's Agents window can start sessions for remote SSH workspaces and choose among available harnesses. The current stable public extension surface does not provide Hacocoon with a supported hook to replace the built-in `New` lifecycle or programmatically choose Copilot/Claude/Codex.

Therefore this extension provisions and opens the isolated Hacocoon workspace, while the built-in Agents window owns the final harness/session creation. A future public Agent Host/AHP lifecycle hook can make that last handoff automatic without moving AHP or model scheduling into Hacocoon Core.
