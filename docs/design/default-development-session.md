# Default development session

[日本語](default-development-session.ja.md) | English

Status: implemented for initial registration and repeat open. Installed Incus,
Windows/WSL and desktop acceptance of this combined path remain pending.

## User contract

After installation, register the repositories to work on, then open Haco:

```bash
haco repo add api https://github.com/OWNER/API.git
haco repo add web https://github.com/OWNER/WEB.git
haco open
```

An argument-free `open` selects all registered sources, prepares independent
project copies and creates or resumes their Environment. The provider resolves
and obtains the default Base during normal creation. Configured Workspace OCI
initialization, mounts, network guards and Git broker wiring use their existing
owners. Desktop SSH setup automatically installs sshd when absent through the
normal package/Policy route. There is no required manual Base build, Workspace
creation, Environment creation or OCI command.

VS Code with Remote-SSH is the default client; `--client ssh` launches a shell.
`--client none --json` prepares/resumes the session and returns its names without
launching or preparing desktop access. One repository occupies `/workspace`;
collections occupy `/workspace/<repository>`. Source files and Git metadata remain
independent of the development copies. Existing dirty files are never refreshed
from the sources during repeat open.

## Ownership and retry

The client stores its navigation reference in `~/.haco-default`, using the same
locked, owner-checked, atomic and fsynced format as explicit directory opens.
It saves a random preparation name and sorted source IDs before the first
mutation, then pins the returned Workspace owner and selected OCI owner. This
file is not a credential or a controller catalog. Guests receive neither it nor
management sockets. A symlink, hardlink, foreign owner or writable reference
directory is refused.

All remote changes use `workspace.workflow` and canonical repository/Environment
lifecycle operations. Concurrent opens on the client serialize through the
reference lock; controller leases remain authoritative across clients. A lost
reply retains the preparation name for idempotent retry. Incomplete native
copies, stale owners, incompatible Base/OCI choices and unknown cleanup fail
closed. No retry deletes retained data, releases uncertain ownership or replaces
an existing Environment. After explicit Environment deletion, `open` recreates
the runtime from retained work through the same lifecycle.

## Permissions, progress and failure

Progress reports repository selection, file preparation, Environment preparation
and shell/editor access on stderr, including a heartbeat during long stages.
JSON results stay on stdout. Raw provider output and credentials are not progress.
Cancellation propagates to the current request; an interrupted result is not
proof of resource absence.

Policy and Approval remain authoritative. Pending network permissions can be
reviewed with `haco approve` in another trusted terminal; waiting requests
continue after an authorized answer. Default deny creates no approval request:
explicitly review the required permissions with `haco config --edit`, then rerun
`haco open`. No wildcard rule, saved approval or permission is added by open.
Failures identify the high-level stage, preserve work and offer diagnosis/retry
guidance. Missing client software is addressed on the desktop, then the same
`open` can be retried.

## Explicit operations and limits

`haco open <environment>`, directory opens, preview, explicit Workspace/Env/Base,
storage, network and configuration commands remain available. `haco open --select`
retains interactive selection of existing Environments; `haco ssh setup` retains
its existing selection behavior. `--base` and `--oci` can explicitly override the
default session's preferences, subject to existing compatibility checks.

The current collection limit is eight sources. Empty registration gives a
`repo add` instruction. Incomplete sources are not silently omitted. Membership
is immutable once prepared: adding/removing registrations later is reported as a
selection conflict, never treated as permission to replace edited work. Use the
[explicit fork workflow](workspace-workflow.md#choose-the-copys-repositories) for
a changed collection. Automatic membership extension remains planned.

Repository tests cover shipped CLI registration/open over Unix RPC, independent
multi-repository ownership, normal Base selection, stop/resume with retained
edits, duplicate opens, cancellation, lost replies and unsafe references. Native
Incus, package downloads/approvals and desktop startup require separate acceptance.
See [ADR 0109](../adr/0109-default-development-session.md).
