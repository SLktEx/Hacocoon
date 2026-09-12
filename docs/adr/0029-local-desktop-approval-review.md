# ADR 0029: Local desktop approval review

[日本語](0029-local-desktop-approval-review.ja.md) | English

Status: accepted; repository implementation, installed acceptance pending.

A Remote-SSH window normally runs development terminals in its untrusted Environment.
Approval must run with the local operator's existing controller authority instead.
The optional UI extension uses a custom VS Code Pseudoterminal and a directly spawned
local child, never a shell terminal, remote command or workspace task. The UI host
identity, desktop mode and workspace trust are checked before starting it.

The installed absolute CLI/WSL paths and minimal environment are fixed by the adapter.
The default Windows distribution is Hacocoon; only the local user setting may select
another installed distribution. Workspace settings, event endpoints and notification
payloads cannot select executable paths, credentials or controller sockets. The ID
only selects an existing trusted request. All approval rendering, saved-scope checks,
Policy persistence and actual execution remain in the ordinary CLI/controller path.

A click opens review without answering. Control/escape sequences and oversized input
are rejected, output is bounded and control characters escaped. Closing or losing the
local child is not proof of rollback: the controller may already have accepted an
answer, and the UI never retries. Existing queue expiry and execution bounds still apply.

Rejected: ordinary createTerminal with shellPath in a remote window, remote workspace
execution of management commands, workspace-configurable binaries, inherited controller
overrides, approval POSTs to the read-only event bridge, and automatic answers from
notification clicks. Optional integration adds no dependency to Core or ordinary haco.

See the [owning contract](../design/pending-approval-review.md) and official
[VS Code Pseudoterminal API](https://code.visualstudio.com/api/references/vscode-api#Pseudoterminal).
