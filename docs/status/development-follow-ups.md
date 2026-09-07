# Development follow-ups

## Revised B4 and daily-development follow-ups

- **Partial:** independent offline Store copy now exists; implement trusted Host
  image acquisition/publication without mounting guest-populated Stores there
  or copying Host credentials/live daemon state. Accept containerd/nerdctl image
  reuse and independence, and separately assess Docker Store compatibility.
- **Planned:** bounded interrupted-copy recovery must first prove the exact
  asynchronous provider operation has finished. Until then, failed copies retain
  `copy_source` and block attachment/deletion; catalog editing is not a user flow.
- **Planned:** complete B's manual VS Code Remote - SSH development acceptance;
  C then adds repeatable SSH setup and optional VS Code selection. The revised
  [development order](architecture-and-roadmap.md#user-facing-development-order)
  takes precedence over older D+ labels below for daily connection convenience.
- **Planned:** C Host/project setup, Windows DNS and restricted preview precede
  D approval-policy/AWS and E environment-recreation work. `switch-base` stays
  disabled/on hold without a scheduled return.


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
  The trigger for an earlier native WSL binfmt registration disappearance was
  not established. Setup/entry repairs only absence through WSL's own service;
  external changes during an already-open session are not continuously watched.
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