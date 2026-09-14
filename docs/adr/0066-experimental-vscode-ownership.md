# Experimental VS Code configuration ownership

Status: accepted; repository implementation, real editor/provider acceptance pending.

## Decision

Keep the evolving editor integration under `experimental.vscode` in trusted Host
user YAML. All editor/file/JSON edits use the same validated subtree and atomic
writer. Apply settings in the Env's VS Code Remote settings, outside repositories.
Use the existing pinned SSH client boundary and the server CLI. Editor-specific
code remains outside Core and providers. See [semantics](../design/experimental-vscode.md)
and [configuration](../reference/experimental-vscode.md).

Release age and pre-release selection are policy for selecting versions at open
time, not a trust boundary against guest code. A complete explicit version is the
only age/pre-release exception. Dependencies follow the same selection path;
no implicit latest resolution is delegated to the server. Installation or
compatibility failures are visible and leave partial application recoverable.

## Rejected alternatives

- Writing `.vscode/settings.json` into repositories risks accidental commits and
  conflates shared Env configuration with repository-owned Folder Settings.
- A mandatory VS Code extension for external Folder Settings would prematurely
  commit to a UI architecture; repository-specific settings are deferred.
- A CLI-only DB or second authoritative JSON file would diverge from direct YAML
  editing. Applied JSON and its managed-key comment are derived Env state only.
- A `force` exception or fallback to latest would hide which release is being
  authorized. Explicit version selection makes the exception reviewable.
- Guest access to Host configuration, reusable credentials or management sockets
  is unnecessary. JSON settings go down the existing SSH stream; credentials do not.

The Experimental schema may change incompatibly. Future Git/SSH/Approval UI may
present state or references while credential material stays with its existing
trusted owner. Installed Windows/WSL and live Marketplace acceptance must be
recorded separately from repository tests.
