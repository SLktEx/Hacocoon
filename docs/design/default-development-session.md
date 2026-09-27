# Default development session

[日本語](default-development-session.ja.md) | English

`haco repo add <URL>` followed by `haco open` is the ordinary development entry.
Open resumes the last opened Environment and starts it if needed. With no
Environments it creates one from the configured default Image, including when
no repositories are registered. New work uses `haco open --new [IMAGE]`.

[Environment creation](environment-creation.md) owns source precedence, immutable
membership, default Image persistence, Volume lifetimes and failure cleanup.
The controller catalog owns the last-opened reference. The former client-side
`~/.haco-default` preparation journal is no longer read or written. It does not
cause existing Environment data to be deleted or automatically migrated.

VS Code with Remote-SSH is the default client; `--client ssh` launches a shell.
`--client none --json` opens without a desktop client and returns Environment
metadata. Progress goes to stderr; results remain on stdout. SSH access pins the
selected Environment ownership and refuses a replaced instance before granting
access. Required permissions remain explicit; `haco approve` reviews pending
requests and `haco config --edit` changes denied Policy scopes. Open grants none.

Closing a client does not stop the Environment. Failed editor launch preserves
completed work so the same open can be retried. Existing damaged work is not
repaired or rebuilt automatically. Directory references remain an explicit
workflow for independently managed Workspaces; they do not select the default
session. See [daily use](../reference/daily-workflow.md),
[the superseded decision](../adr/0111-default-development-session.md) and
[the current decision](../adr/0113-unified-environment-creation.md).

`haco open --select` explicitly chooses an existing Environment through the same
open service and updates last-opened; ordinary open never requires a picker.
