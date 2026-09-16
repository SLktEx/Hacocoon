# Client and interactive access

Ordinary SSH uses OpenSSH ProxyCommand through the controller UDS and generic
bidirectional byte sessions. The Environment still runs sshd on port 22. No
ordinary SSH Host port, loopback listener or Incus proxy device is allocated.
See [ADR 0065](../adr/0065-portless-ssh-controller-streams.md) for ownership and
rejected alternatives. Real-host acceptance is distinct from implementation.

## Ordinary product route

`haco ssh setup [env]` installs or reuses client-owned keys, a strict host-key pin
and a stable `haco-<name>` SSH target. `haco open [env]` uses the same setup and
launches standard VS Code Remote-SSH on `/workspace`; `--client ssh` opens a shell.
Argument-free `open` prepares the [default session](default-development-session.md); `open --select` retains existing-Environment selection. SSH setup is editor-neutral. No Hacocoon extension is required.

After one successful setup, select the alias in VS Code's standard Remote Explorer
SSH Targets or reopen the recent remote `/workspace` folder. On Windows,
ProxyCommand invokes the saved WSL distribution through `wsl.exe`, waits for the
existing controller, and requests the saved target. It does not require a terminal,
manual WSL startup, another `haco open`, or a Hacocoon-specific VS Code command.

[Getting started](../guides/getting-started.md) owns first use and
[Windows SSH](../reference/windows-environment-ssh.md) owns configuration examples.

## Connection authority

The controller validates the Environment creation ID, Workspace binding, access
mode, active lease, provider ownership and persistent SSH grant before opening a
stream. A stopped Environment is resumed under the canonical lifecycle lock.
Concurrent SSH/VS Code reconnects cannot create duplicate runtimes. Missing,
deleted, replaced, revoked or recovery-required targets fail closed. Matching a
name never authorizes adoption, recreation, repair or Policy changes.

The provider persists a pending grant before installing its managed public key.
A ready grant contains the validated public host key. Revocation disables the
grant, removes its key marker and then removes metadata. It preserves unrelated
keys and does not stop sshd. Client private keys never enter an Environment.
Controller UDS and raw Incus sockets are not projected to ordinary Environments.

Only service `ssh` is exposed by this stream adapter. Clients cannot select a Host
address or arbitrary destination. Byte sessions preserve EOF/half-close and use
separate cancellation/completion control. ProxyCommand stdout carries only SSH
bytes; safe diagnostics use stderr. Abandoned half-closed sessions drain for at
most 30 seconds.

## Configuration ownership and migration

The existing Include points to `.ssh/hacocoon/*.conf`; confined, locked, atomic
writes preserve unrelated SSH configuration. Each managed fragment binds a human
alias to an exact target. Conflicting user aliases, replacement targets and a
different WSL installation are refused. `haco env disconnect`, Environment deletion
and explicit `haco ssh cleanup` remove only identified owned fragments. Cleanup
requires positive stale evidence; transport uncertainty leaves files intact.
Shared private keys and host-key pins are retained.

Run explicit setup once to migrate an older installation. Removal-only provider
migration revokes old managed keys and removes validated legacy SSH proxy devices;
new setup generates ProxyCommand configuration. This migration never runs on cold
reconnect. Generic forwarding and [preview](development-preview.md) remain separate.

`pkg/clientadapter` retains reusable lifecycle/access contracts. The standalone
VS Code adapter now shares product SSH setup through the controller instead of
owning a second transport or configuration writer. See
[adapter API](../reference/client-adapter.md) and
[VS Code integration](client-adapters-and-vscode-integration.md).

## Incomplete lifecycle diagnostics

Status, Environment doctor and adapters share the service observation. Missing
provider resources with retained ownership, incomplete leases and cleanup-required
state report recovery-required. Unknown state never recommends automatic repair
or releases Workspace/OCI reservations.
