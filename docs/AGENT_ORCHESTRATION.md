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
    "stderr": "..."
  },
  "cleaned_up": true
}
```

A non-zero command result remains a non-zero `haco` process result. Infrastructure and cleanup failures are not converted into successful command results.

## Security events

Security-sensitive authority remains in the v0.4+ Policy/Capability boundary. External clients can read its audit-derived event stream without receiving capability parameters or credentials:

```bash
haco events --json
```

The JSON form is JSON Lines: one event per line. Events include the capability `request_id` for correlation plus safe policy attributes, decisions, approval decisions, and completion state. Capability `Parameters` are not present because they are intentionally excluded from the audit source.

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
