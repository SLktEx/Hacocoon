# Internal temporary execution

[日本語](temporary-execution.ja.md) | English

Temporary execution is an internal facility for Image building and OCI maintenance.
The public daily entry is [open/create](environment-creation.md). Internal callers
still use canonical creation, exact generation ownership and bounded cleanup.

A temporary Workspace contains only disposable internal work. Retained user data
must never be adopted by name or deleted merely because cleanup was attempted.
Provider absence is positively verified before ownership and leases are released.
Unknown cleanup keeps recovery-required ownership. Cancellation uses a bounded
cleanup context independent of execution cancellation. Live-owner locks prevent
startup reconciliation from deleting work still in progress.

Captured output remains bounded and redacted under the shared logging rules.
Internal process execution keeps command failures distinct from cleanup failures.
See [temporary ownership](../adr/0020-runtime-owned-temporary-workspaces.md) and
[lifecycle ownership](../adr/0002-environment-lifecycle-ownership.md).
