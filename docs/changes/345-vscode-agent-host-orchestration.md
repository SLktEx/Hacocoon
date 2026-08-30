# #345 VS Code Agent Host orchestration

Hacocoon's existing per-session Agent Host adapter now exposes a machine-readable session descriptor, a read-only lookup path, and a direct VS Code remote-workspace URI.

The orchestration boundary remains intentionally split:

- VS Code / another trusted orchestrator owns agent sessions, scheduling, UI, model/harness routing, and AHP;
- Hacocoon owns the per-session Environment, Workspace lease, loopback client access, and authority boundary;
- Git worktree creation remains outside Core and a writable top-level agent session should normally receive its own linked worktree.

For automation, the intended shape is:

```text
haco-agent-host prepare --session <id> --json --no-launch <worktree>
haco-agent-host lookup --session <id> --json
haco-agent-host release --session <id>
```

For interactive VS Code use, `prepare` launches the Agents window with the Hacocoon `vscode-remote://ssh-remote+.../workspace` folder URI instead of requiring a second manual Remote-SSH workspace selection.
