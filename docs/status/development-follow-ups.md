# Development follow-ups

Status: remaining work after the revised Stage B contract. Acceptance evidence
and any current blockers are owned by [implementation status](../IMPLEMENTATION_STATUS.md).

- **Stage D+ Base UX:** `switch-base` is currently disabled. Reconsider whether
  it is needed, whether a recreate helper is useful, its Workspace/OCI/snapshot
  relationships and CLI UX. Historical ADR 0011/code/tests remain. It blocks
  neither Stage A-C completion nor ordinary delete/create/attach lifecycle.
- **Stage D+ SSH/IDE UX:** config auto-installation, `.ssh/config` edits,
  creation-time SSH registration, VS Code Remote SSH/haco-vscode changes,
  Windows haco.exe and final SSH UX are deferred. Manual Windows native SSH is
  tracked by [#476](https://github.com/SLktEx/Hacocoon/issues/476).
- **Windows breadth:** drive hotplug/removal/reconnection, Windows executable
  compatibility beyond tested CLI tools, additional Windows/WSL configurations,
  interrupted upgrades and generic recovery remain unverified. C and non-C
  acceptance of this change is recorded with the actual package, not assumed.
- **Persistent OCI breadth:** Docker Store compatibility, runtime version
  migration, simultaneous RW sharing, snapshots/compaction, automatic updates,
  large images/performance and live migration are deferred. Registry proxy,
  credential broker and local registry are Stage D+ topics. No Host runtime
  authority or credentials are exposed by the current Store attachment.
- **SSH package proxy convenience:** [#469](https://github.com/SLktEx/Hacocoon/issues/469).
  Automatic proxy exports and IDE launch remain separate from manual SSH access.
- **Git interruption/results:** [#470](https://github.com/SLktEx/Hacocoon/issues/470).
  Interrupted approvals, ambiguous external push completion, large packs,
  additional auth methods, LFS, submodules and multiple refs remain follow-up.
- **Collections/recovery:** membership editing, interrupted preparation,
  concurrent lifecycle operations and generic recovery remain out of scope.
- **Incus version policy:** [#479](https://github.com/SLktEx/Hacocoon/issues/479)
  remains a separate support-policy change; this task records the installed
  runtime version it actually tests.

The old distribution-only implementation and acceptance are historical, as is
[PR #473](https://github.com/SLktEx/Hacocoon/pull/473). Related docs-only
[PR #474](https://github.com/SLktEx/Hacocoon/pull/474) was reviewed; the new request
also disables the public command. Other open CI/localization work is preserved.
Stage C is not fully implemented here; macOS is entirely out of scope.