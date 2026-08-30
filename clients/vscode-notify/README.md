# Hacocoon Notifications for VS Code

This optional extension presents minimized Hacocoon `pkg/interaction` events inside desktop VS Code.

It is **not** required for `haco-vscode` or Remote-SSH. It does not approve capabilities, execute privileged actions, carry approval tokens, or replace the normal Hacocoon Policy/Capability boundary.

## Run the local bridge

On the trusted Hacocoon Host (for example inside WSL):

```bash
haco-notify web --listen 127.0.0.1:18081
```

The extension defaults to `http://127.0.0.1:18081` and rejects non-loopback endpoints.

## Notifications

The extension surfaces:

- `approval-required`
- `recovery-required`
- `operation-failed`
- `policy-denied`
- `approval-denied`
- optionally `operation-completed`

Cursor and recent stable event IDs are stored in VS Code `globalState`, so reconnect/reload can resume without replaying committed notifications.

Only minimized public interaction fields are consumed: event/request identity, kind, Environment, capability, action, closed failure code, attention flags, and cursor. Raw capability resources, attributes, credentials, approval tokens, provider output, and free-form audit reasons are not available through the bridge.
