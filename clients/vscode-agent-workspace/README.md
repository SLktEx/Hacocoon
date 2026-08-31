# Hacocoon Agent Workspaces for VS Code

This workspace-side VS Code extension creates one linked Git worktree and one Hacocoon Environment for an independently writable top-level coding-agent session.

## Commands

- `Hacocoon: New Agent Workspace`
- `Hacocoon: Release Agent Workspace`

`New Agent Workspace` runs on the VS Code extension host next to the repository:

```text
Git repository
  -> git worktree add -b <branch> <owned-worktree> HEAD
  -> haco-agent-host prepare --session <opaque-id> --json --no-launch <owned-worktree>
  -> validate Hacocoon descriptor
  -> vscode.openFolder(vscode-remote://ssh-remote+haco-agent-.../workspace)
```

After the remote workspace opens, start the desired Copilot, Claude, Codex, or other supported harness from the VS Code Agents window. Hacocoon does not use private/undocumented VS Code command IDs to select or create an Agent Host session.

## Execution boundary

The extension is `extensionKind: workspace`. `git` and `haco-agent-host` therefore run on the VS Code extension host and must be available there. This is intentional: the process that can see the repository/worktree is also the trusted process that requests the Hacocoon Environment.

For Windows/WSL deployments where the repository and trusted Hacocoon adapter live behind another client boundary, configure `hacocoon.agentWorkspace.adapterExecutable` to a trusted wrapper executable. Do not use a shell command string; the extension invokes the configured executable directly with an argument array.

The current trusted-host design still has a separate open seam around the long-term physical placement/export of repositories that logically live in `haco-host`. This extension does not weaken that boundary or mount raw Incus/Hacocoon authority into an ordinary Environment.

## Worktree ownership and cleanup

By default worktrees are created under a `.hacocoon-worktrees/<repo>` directory beside the main checkout. Override this with `hacocoon.agentWorkspace.worktreeRoot`.

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
- `hacocoon.agentWorkspace.worktreeRoot` — empty means the default owned sibling directory

## VS Code / Agent Host limitation

VS Code's Agents window can start sessions for remote SSH workspaces and choose among available harnesses. As of the current public extension API, Hacocoon does not depend on an API for intercepting the built-in `New` action or programmatically selecting Copilot/Claude/Codex. The extension provisions and opens the isolated workspace; the built-in Agents window owns harness/session creation. A future public Agent Host/AHP lifecycle hook can make that final handoff automatic without moving AHP into Hacocoon Core.
