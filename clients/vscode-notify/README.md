# Hacocoon Notifications for VS Code

This optional extension presents minimized Hacocoon `pkg/interaction` events inside desktop VS Code.

It is **not** required for `haco-vscode` or Remote-SSH. Review opens the ordinary trusted local CLI; no click approves an operation or replaces the normal Hacocoon Policy/Capability boundary.

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

## Install and review

From the repository root, build the optional extension and install it in desktop VS Code:

```sh
python tools/package_vscode_notifications.py hacocoon-notifications.vsix
code --install-extension hacocoon-notifications.vsix
```

Choose **Review** on a notification, or **Hacocoon: Review Pending Approvals** in the
command palette. Inspect the complete current request and reusable scope in the local
approval terminal, then use the ordinary yes/no or saved choices. Saving ask requires
a separate answer for this operation. The command palette works without the event bridge.

Windows uses the installed Hacocoon WSL distribution; Linux requires the installed
Physical Host CLI at /usr/local/bin/haco and the operator's normal controller access.
A different Windows distribution can be selected in the local user setting
hacocoon.review.wslDistribution. Workspace values are ignored. Web/remote extension
hosts and untrusted windows cannot run review. Never install the UI extension in an
Environment as a way to grant it management access.

Closing/Ctrl-C/Ctrl-D does not undo a submitted answer. On failure or disconnection,
inspect Policy/audit before retrying. The optional Windows notification adapter opens the same CLI; Linux native activation remains planned.
See the [approval contract](../../docs/design/pending-approval-review.md) for exact
boundaries and the distinction between repository tests and installed acceptance.
