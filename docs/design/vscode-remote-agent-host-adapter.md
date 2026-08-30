# VS Code Remote Agent Host Adapter

**Status:** v0.10 foundation implemented on `main` via PR #137; direct orchestration descriptor/remote-workspace launch added by #345.  
**Compatibility:** pre-1.0; helper CLI and integration details may change incompatibly.  
**Host acceptance:** real Windows/WSL + Incus + current VS Code Agents-window behavior remains environment-dependent and is covered separately by the product E2E work.

## Goal

Hacocoon connects its trusted per-agent Environment broker to VS Code's Remote Agent Host workflow without giving the coding agent Hacocoon or Incus management authority.

VS Code remains the orchestration/UI/AHP layer. Hacocoon remains the isolated execution/runtime layer.

```text
VS Code Agents window / trusted orchestrator
                 |
        opaque session identity
                 |
          haco-agent-host
                 |
       Hacocoon Environment
                 |
      dedicated Git worktree
                 |
           /workspace
                 |
     VS Code remote Agent Host
                 |
      coding-agent harness
```

A separately routable top-level agent session can therefore be assigned one Hacocoon Environment and one worktree while VS Code continues to own session orchestration, model/harness selection, UI, and AHP.

## Implemented surface

```text
haco-agent-host prepare --session <opaque-id> [--json] [options] [workspace]
haco-agent-host lookup  --session <opaque-id> [--json]
haco-agent-host release --session <opaque-id>
```

### `prepare`

`prepare`:

- acquires/reuses the Environment through `internal/agenthost`;
- requires the Environment to be running;
- hashes the opaque session identity before deriving the SSH alias;
- keeps the SSH private key on the client side and passes only the public key through the existing Hacocoon SSH access path;
- writes only an adapter-owned SSH config fragment under `~/.ssh/hacocoon/`;
- binds SSH to an existing Hacocoon loopback-only client connection;
- reuses a compatible managed SSH connection or rotates it only after the replacement is ready;
- emits a session descriptor containing the bound Environment, SSH alias, `/workspace`, and VS Code remote-folder URI;
- launches the VS Code Agents window directly on the Hacocoon remote workspace unless `--no-launch` is requested.

The default launch shape is conceptually:

```text
code --agents --folder-uri vscode-remote://ssh-remote+<managed-alias>/workspace
```

This removes the previous manual `New -> Remote -> SSH -> <alias>` handoff after preparation.

For trusted automation/orchestrators, use:

```text
haco-agent-host prepare --session <id> --json --no-launch <worktree>
```

The JSON descriptor includes:

```json
{
  "session_id": "opaque-session-id",
  "environment": "agent-...",
  "workspace_path": "/trusted/host/worktree",
  "remote_workspace": "/workspace",
  "ssh_alias": "haco-agent-...",
  "host_port": 2222,
  "folder_uri": "vscode-remote://ssh-remote+haco-agent-.../workspace"
}
```

The raw session identity is intentionally available only in this trusted machine-readable response; it is still not used as the Environment name or SSH alias.

### `lookup`

`lookup` is read-only orchestration introspection. It resolves an existing persisted session binding and emits the same descriptor shape without creating, adopting, rebinding, or deleting an Environment.

```text
haco-agent-host lookup --session <id> --json
```

The persisted binding remains the ownership proof. Unknown or stale sessions fail closed.

### `release`

`release` releases the persisted per-session binding and removes the managed client SSH fragment. Cleanup ambiguity follows the existing recovery-required discipline rather than silently claiming success.

## Worktree ownership

Hacocoon does not make Git worktree creation a Core concern. A trusted client/orchestrator should normally allocate a distinct linked worktree for each independently writable agent session and pass that path to `prepare`.

```text
repository
  +-- worktree/session-a -> Environment A -> Agent session A
  +-- worktree/session-b -> Environment B -> Agent session B
```

Git worktrees isolate code changes; Hacocoon Environments provide OS/runtime isolation.

## Security boundary

The adapter does not make a coding agent a Hacocoon client. Environment allocation, SSH preparation/revocation, Workspace ownership, and release remain on the trusted side.

Raw session IDs are not used as persisted/public SSH host aliases. Private SSH keys are not copied into the Environment. The adapter uses the same loopback-only client-access boundary as the existing VS Code adapter.

AHP is an external VS Code integration protocol and does not become Core vocabulary. Hacocoon also does not own task decomposition, model routing, retries, token budgets, or the Agents UI.

## Other orchestrators

The descriptor is deliberately client-neutral enough for a trusted non-VS-Code orchestrator to allocate/reconnect a Hacocoon Environment and then use the existing generic client-access boundary. VS Code-specific folder-URI launching remains in this adapter rather than Core.

This allows tools such as future task schedulers or multi-agent orchestrators to reuse the same per-session runtime boundary instead of Hacocoon growing a competing scheduler.

## Packaging

`haco-agent-host` is part of the standard Linux release archives and installer alongside `haco` and `haco-vscode`.

## Validation

Repository CI covers helper behavior, descriptor serialization, remote-folder URI construction, SSH-config injection rejection, connection reuse/rotation helpers, Go tests/vet/race, release packaging, installer checks, and existing host-independent E2Es.

Real VS Code Agent Host behavior, Windows/WSL path translation, real Incus SSH, and multi-session routing remain real-host acceptance work. #344 tracks the composed fresh-Windows -> `haco-host` -> worktree -> Environment -> VS Code user journey.

> **VS Code owns agent orchestration; Hacocoon owns the isolated per-session workspace runtime and authority boundary.**
