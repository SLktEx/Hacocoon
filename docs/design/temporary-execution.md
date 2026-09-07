# Temporary execution

[日本語](temporary-execution.ja.md) | English

Status: **implemented product CLI; real-Incus acceptance passed at 4adfe19**.

Run one command without first naming or creating an Environment:

```sh
haco run --rm -- uname -a
haco run -- sh -c 'echo hello > message.txt; cat message.txt'
```

Removal is the default, so --rm is optional. Commands start in /workspace.
When --workspace is omitted, that directory belongs to the temporary Environment;
its contents disappear with the Environment. The default configured Hacocoon Base
is used; --base selects another Base. This is not a Docker image selector.

To keep project files, select an existing Workspace:

```sh
haco run --workspace managed:dev -- sh -c 'echo result > result.txt'
haco run --workspace managed:dev --read-only -- ls
```

The selected Workspace and its OCI Store survive. A retained Environment already
leasing it must be deleted before lending it to another Environment; stop retains
the lease. Alternatively use an independent Workspace copy.

Published OCI content is copied automatically, including for a temporary
Workspace; --no-oci opts out. With no published content there is nothing to copy.
Only the default copy bound to a temporary Workspace is removed after runtime
deletion. Host Docker/nerdctl publication and real image-use acceptance retain their
separate implementation limits in [status](../IMPLEMENTATION_STATUS.md).

Command stdout/stderr and exit status are preserved. Output is captured with the
existing size limits, not streamed interactively; truncation is reported.
--json returns execution metadata and cleaned_up. A nonzero command exit remains
a failure even when cleanup succeeds. Cleanup failure is reported separately and
is never converted into success. --rm=false and interactive stdin/TTY are unsupported.

Ctrl+C requests controller cancellation and returns 130. Because the client has
disconnected, it does not claim deletion was confirmed. Use haco env list and
haco env status <name> to inspect remaining work. The controller retains recovery
markers on incomplete cleanup and retries them on startup or the next run.
Do not remove state files to silence recovery errors. Incomplete OCI copies may
require provider-specific inspection before their existing reservations can clear.

The canonical lifecycle preserves ownership and leases until runtime absence.
Temporary resource cleanup compares Workspace ownership atomically. Reusing an
Environment name cannot redirect cleanup to retained work. See
[ADR 0020](../adr/0020-runtime-owned-temporary-workspaces.md) and
[connection cancellation](../adr/0018-ephemeral-run-cancellation.md).

Repository tests cover argument preservation, default temporary selection,
retained Workspace ownership, cleanup failure/retry and OCI source retention.
The maintained Incus GHA fixture passed at 4adfe19 (run 34115004878, job
101719650209), using ordinary product commands for success, exit 17, retained file
writes and actual cancellation followed by verified provider absence. Interactive sessions, local installed acceptance and a populated OCI
image execution are not claimed by these checks.
