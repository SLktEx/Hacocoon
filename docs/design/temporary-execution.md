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

## Output and crash ownership

The shared Host process runner retains at most 4 MiB each of stdout and stderr by
default. Excess bytes are consumed and discarded without terminating the child or
changing its exit code. Truncated output has a visible marker plus JSON
`stdout_truncated` / `stderr_truncated`; `stdout_bytes` / `stderr_bytes` report
observed bytes before truncation. Do not interpret that prefix as complete output.
Control subprocesses have the same boundary; oversized structured output must fail parsing.

The `--json` result includes `environment`, `execution` (exit code, both streams
and truncation fields), and `cleaned_up`. Execution success and cleanup success
are separate outcomes.

Before creation, a protected `ephemeral_runs` marker is persisted and a per-run
Linux `flock` remains held throughout the run. Only after the owner exits and the
lock can be acquired may reconciliation attempt bounded canonical deletion.
A `run-` name, a marker alone or PID guessing is not deletion authority.
Live-owner locks are skipped; failure retains `cleanup-required`. Unsupported
platforms fail rather than substitute weaker ownership proof. SIGINT/SIGTERM
cleanup uses a separate bounded context independent of execution cancellation.

Each new marker binds an unpredictable Environment creation identity before
canonical creation. The catalog reserves that name for the unfinished run;
cleanup checks the same ephemeral lease identity under the lifecycle lock.
It cannot delete a replacement creation. Marker removal is refused until the
Environment and lease are absent. Retained Workspace/OCI data are not removed.
See [ADR 0068](../adr/0068-ephemeral-run-creation-ownership.md).

Schema 14 retains older records without inventing ownership. A legacy run with
no creation identity and a retained Workspace remains recovery-required; retries
cannot safely choose an Env by name. Legacy temporary runs retain their exact
Workspace ownership check. This development change has repository regression
coverage; its native acceptance and stdin/TTY implementation remain pending.
