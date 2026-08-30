# Agent and orchestrator integration

Hacocoon v0.6 is an execution boundary **below** agent/orchestration systems. It does not own task graphs, model selection, retries, budgets, code review, or merge workflow.

## Short-lived agent execution

Use `haco run` when the caller needs one isolated command and does not need to keep the Environment afterwards:

```bash
haco run --workspace /path/to/worktree -- codex
haco run --workspace /path/to/worktree -- claude
```

Read-only workspaces are explicit:

```bash
haco run --read-only --workspace /path/to/worktree -- some-agent
```

`run` is only syntactic sugar for:

```text
create ephemeral Environment
        -> execute exact argv
        -> delete Environment
```

Cleanup is attempted after execution even when the command fails. A cleanup failure is surfaced rather than hidden.

### Output memory boundary

Hacocoon treats child process output as untrusted input. The shared host process runner retains at most **4 MiB of stdout and 4 MiB of stderr per subprocess** by default. Output beyond those limits is consumed and discarded so a noisy or malicious child cannot make the trusted Hacocoon process retain an unbounded `bytes.Buffer`.

Reaching the capture limit does not terminate the child and does not change its exit code. A truncated stream ends with an explicit marker such as:

```text
[haco: output truncated; total-bytes=7340032]
```

Machine clients should also inspect the structured truncation fields described below. The hard host-runner limit applies to control subprocesses as well as agent execution; if a structured control command unexpectedly exceeds the limit, downstream parsing should fail rather than allowing unbounded host memory growth.

For machine clients:

```bash
haco run --workspace /path/to/worktree --json -- codex
```

The JSON result has a stable shape:

```json
{
  "environment": "run-...",
  "execution": {
    "exit_code": 0,
    "stdout": "...",
    "stderr": "...",
    "stdout_truncated": false,
    "stderr_truncated": false,
    "stdout_bytes": 123,
    "stderr_bytes": 0
  },
  "cleaned_up": true
}
```

`stdout_bytes` and `stderr_bytes` are the observed stream sizes before truncation. When a `*_truncated` field is true, the corresponding string contains only the retained prefix plus the visible truncation marker; callers must not interpret it as complete command output.

A non-zero command result remains a non-zero `haco` process result. Infrastructure and cleanup failures are not converted into successful command results.

## Security events

Security-sensitive authority remains in the v0.4+ Policy/Capability boundary. External clients can read its audit-derived event stream without receiving capability parameters or credentials:

```bash
haco events --json
```

The JSON form is JSON Lines: one event per line. Events include the capability `request_id` for correlation plus safe policy attributes, decisions, approval decisions, completion state, and a `next_offset` byte cursor. Capability `Parameters` are not present because they are intentionally excluded from the audit source.

`haco events` streams records one at a time instead of first loading the entire audit history into memory. A full read still performs O(N) I/O, but memory use is bounded by one audit record plus the caller/output buffers.

### Incremental consumption

A polling orchestrator should persist `next_offset` from the **last event it successfully consumed** and pass that value back on the next read:

```bash
haco events --json --since-offset 12345
```

Only complete records beginning at that byte offset are decoded and emitted. For example:

```text
poll 1: offset 0
  event A -> next_offset 812
  event B -> next_offset 1537
  persist 1537

new audit records are appended

poll 2: --since-offset 1537
  event C -> next_offset 2274
  persist 2274
```

Cursor rules are intentionally strict:

- `0` means start at the beginning of the current audit file;
- a cursor must point to a JSONL record boundary;
- a cursor beyond the current file size is rejected because the file may have been truncated or rotated;
- if output/consumer handling fails while delivering an event, do not advance past that undelivered event;
- appending records to the same audit file preserves existing cursors;
- raw byte offsets identify positions only within the **current audit file generation**. If an external process rotates or replaces the audit file, reset the cursor to `0` explicitly. A replacement file that happens to be at least as large as the old cursor cannot be distinguished reliably using a raw offset alone.

Audit corruption remains fail-closed. Hacocoon emits only trustworthy records before the first malformed/incomplete record and then returns an error. The corruption byte offset is absolute within the audit file. When reading from a non-zero cursor, the reported corruption line number is relative to that resumed stream.

The event surface is observational. It does not turn Hacocoon into a development-review or orchestration engine.

## Daintree integration recipe

A Daintree-style orchestrator can own the higher-level loop:

```text
Daintree
  -> choose task/model/budget
  -> create or select worktree
  -> haco run --workspace <worktree> -- <agent command>
  -> inspect structured result
  -> decide retry/review/merge
```

If Daintree creates the worktree itself, Hacocoon consumes it through the existing external Workspace provider. Hacocoon does not need to know that the directory came from Git worktree machinery.

Security approval remains separate. Daintree may observe `haco events --json`, while Hacocoon policy still decides whether privileged capabilities are allowed, denied, or require approval.

## Rookery integration recipe

A Rookery-style system follows the same boundary:

```text
Rookery task
  -> materialize workspace
  -> invoke Hacocoon run
  -> consume exit/stdout/stderr/cleanup result
  -> retain orchestration state outside Hacocoon
```

Rookery can run multiple independent workspaces concurrently. v0.2 WorkspaceLease rules continue to protect accidental conflicting read/write use of the same Workspace.

## What stays outside Hacocoon

Hacocoon v0.6 intentionally does not implement:

- DAG/task decomposition;
- agent or model choice;
- token/model budgets;
- retry policy;
- development approval queues;
- PR acceptance or merge decisions;
- an IDE-specific agent UI.

Those responsibilities belong to the caller. Hacocoon provides the secure Workspace/Environment/Execution and Capability boundary underneath them.
