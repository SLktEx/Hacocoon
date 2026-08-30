# v0.10 VS Code Remote Agent Host Adapter

**Status:** implemented on `main` via PR #137.  
**Compatibility:** pre-1.0; helper CLI and integration details may change incompatibly.  
**Host acceptance:** real Windows/WSL + Incus + current VS Code Agents-window behavior remains environment-dependent.

## Goal

v0.10 connects the v0.9 trusted per-agent Environment broker to VS Code's Remote Agent Host workflow without giving the coding agent Hacocoon or Incus management authority.

```text
VS Code Agents window
        |
    Remote SSH
        |
Hacocoon-managed loopback alias
        |
  haco-agent-host
        |
 v0.9-bound Environment
        |
    /workspace
```

## Implemented surface

```text
haco-agent-host prepare --session <opaque-id> [options] [workspace]
haco-agent-host release --session <opaque-id>
```

`prepare`:

- acquires/reuses the Environment through `internal/agenthost`;
- requires the Environment to be running;
- hashes the opaque session identity before deriving the SSH alias;
- keeps the SSH private key on the client side and passes only the public key through the existing Hacocoon SSH access path;
- writes only an adapter-owned SSH config fragment under `~/.ssh/hacocoon/`;
- binds SSH to an existing Hacocoon loopback-only client connection;
- reuses a compatible managed SSH connection or rotates it only after the replacement is ready;
- launches `code --agents` unless `--no-launch` is requested.

`release` releases the v0.9 binding and removes the managed client SSH fragment. Cleanup ambiguity follows the existing recovery-required discipline rather than silently claiming success.

## Security boundary

The adapter does not make a coding agent a Hacocoon client. Environment allocation, SSH preparation/revocation, Workspace ownership, and release remain on the trusted side.

Raw session IDs are not used as persisted/public SSH host aliases. Private SSH keys are not copied into the Environment. The adapter uses the same loopback-only client-access boundary as the existing VS Code adapter.

## Packaging

`haco-agent-host` is part of the standard Linux release archives and installer alongside `haco` and `haco-vscode`.

## Validation

Repository CI covers helper behavior, SSH-config injection rejection, connection reuse/rotation helpers, Go tests/vet/race, release packaging, installer checks, and existing host-independent E2Es.

Real VS Code Agent Host behavior, Windows/WSL path translation, and real Incus SSH remain host acceptance work and are not implied by repository CI.

## Relationship to later gates

- v0.11 Base selection changes the guest starting filesystem but does not expand adapter authority.
- v0.12 Resource Budgets constrain the assigned Environment but do not give agents authority to raise their own limits.

> **v0.10 is the thin trusted bridge from VS Code's Agent Host UI to the v0.9-owned Environment; VS Code still owns the agent UI/protocol and Hacocoon still owns the sandbox/authority boundary.**
