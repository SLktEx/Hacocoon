# Agent and orchestrator integration

Hacocoon is an execution boundary **below** agent/orchestration systems. It does not own task graphs, model selection, retries, budgets, code review, or merge workflow.

Two integration styles now coexist:

1. the v0.6 generic machine/orchestrator path, where a trusted external caller may invoke `haco run`;
2. the v0.9 per-agent sandbox path, where a trusted integration layer binds an agent session to an Environment without requiring the coding agent itself to invoke `haco`.

## Short-lived agent execution

Use `haco run` when the **trusted caller** needs one isolated command and does not need to keep the Environment afterwards:

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

## v0.9 per-agent sandbox path

For long-lived or independently routable agent sessions, v0.9 adds a different control-plane model:

```text
VS Code Agents window / trusted client
              |
      trusted integration
              |
    opaque session identity
              |
      per-session broker
              |
         Environment
              |
            Incus
              |
      Agent Host / agent
```

The coding agent is deliberately **not** the caller of Hacocoon in this model.

The trusted integration may allocate/release the Environment on the agent's behalf, but the agent must not receive Incus administrator authority, Hacocoon management state/control access, or broad host credentials just so it can manage its own sandbox.

The existing human/operator CLI remains available; it simply is not the agent protocol.

### Parallel agents and worktrees

Parallel read/write sessions should normally use separate Git worktrees:

```text
repo
  +-- worktree/a -> Environment A -> Agent A
  +-- worktree/b -> Environment B -> Agent B
```

The existing WorkspaceLease rule still rejects conflicting RW use of the same canonical Workspace. Hacocoon does not weaken that rule for multi-agent convenience.

Worktree creation remains outside Core. A trusted client/orchestrator may create the worktree and pass its path to Hacocoon.

### VS Code Agent Host / AHP

The preferred VS Code direction is to run the Agent Host next to the assigned Workspace inside the Environment and keep Agent Host Protocol details in the client-integration layer.

AHP-specific types and versions do not become Core vocabulary. Lifecycle hooks may assist observation/cleanup but are not themselves proof of sandbox isolation.

The current repository contains the v0.9 session-to-Environment broker foundation. Real VS Code Agent Host/AHP + Incus per-session routing remains an environment-dependent acceptance path; see `09_v0.9_PER_AGENT_SANDBOX_AND_AGENT_HOST.md`.

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
  -> trusted Hacocoon integration
       -> haco run for short-lived execution
       or
       -> v0.9 per-session Environment binding
  -> inspect result/events
  -> decide retry/review/merge
```

If Daintree creates the worktree itself, Hacocoon consumes it through the existing external Workspace provider. Hacocoon does not need to know that the directory came from Git worktree machinery.

Security approval remains separate. Daintree may observe `haco events --json`, while Hacocoon policy still decides whether privileged capabilities are allowed, denied, or require approval.

## Rookery integration recipe

A Rookery-style system follows the same boundary:

```text
Rookery task
  -> materialize workspace
  -> invoke trusted Hacocoon execution/binding path
  -> consume result/state
  -> retain orchestration state outside Hacocoon
```

Rookery can run multiple independent workspaces concurrently. v0.2 WorkspaceLease rules continue to protect accidental conflicting read/write use of the same Workspace.

## What stays outside Hacocoon

Hacocoon intentionally does not implement:

- DAG/task decomposition;
- agent or model choice;
- token/model budgets;
- retry policy;
- development approval queues;
- PR acceptance or merge decisions;
- an IDE-specific agent UI;
- Git branch/worktree orchestration as a Core responsibility;
- agent-visible Incus/Hacocoon sandbox-management authority.

Those responsibilities belong to the caller. Hacocoon provides the secure Workspace/Environment/Execution and Capability boundary underneath them.
