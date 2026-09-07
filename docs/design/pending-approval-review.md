# Pending approval review

[日本語](pending-approval-review.ja.md) | English

Status: **implemented repository slice; VS Code review implemented; native activation remains planned**.
Repository tests are separate from installed network, desktop and GitHub acceptance.

## Ordinary use

In the trusted Host, run:

```sh
haco approve
```

One pending request opens directly. With several requests, choose a number. The
prompt displays current capability, action, target, Environment creation identity,
authority attributes and the separate reusable scope. Answer yes/no for this
operation, or choose an Environment/global allow, deny or ask Policy. Saving ask
still requires a separate yes/no answer for the current operation.

`haco approve --list` prints the trusted pending details as JSON without deciding.
An optional request ID selects an exact request. No new arguments are required for
ordinary Git or network operations. Existing Git approve/deny commands remain.
Use [haco config](../reference/configuration.md) to inspect or edit saved Policy.

Ordinary results show Approved/Denied and any saved Policy in readable text. Add
--json when a script needs the capability receipt. It distinguishes execution
state, saved choice and audit completion. A denied request is not executed; it
does not have a successful provider-completion audit. On transport loss or a
failed outcome, inspect Policy and audit before retrying. Never infer persistence
or successful execution from a submitted answer alone.

For network.egress, succeeded means connection authorization completed, not that a remote HTTP operation succeeded. The original application still reports connection, TLS and application failures. Installed acceptance checks the actual HTTPS response separately from the authorization receipt.

## Waiting and authority

The replaceable Standard queue admits background controller requests only after
Policy requires human approval. It holds at most 128 active sessions, 16 per
Environment name, and a 16 KiB display snapshot per request. Human selection has
a two-minute deadline and also respects the original request's cancellation.
Claimed sessions remain counted until the actual application outcome is known.
Overflow, expiry and cancellation before selection fail closed. The controller
never reads ambient stdin. Interactive control sessions retain their own callback.

One admitted session owns waiting and completion. Only one reviewer can consume
it. Stale or duplicate completion cannot release a replacement session. Completion
delivery is buffered, so a disconnected reviewer cannot block resource release.
Cancellation after submission does not undo an already accepted answer or allow
replay. Restart abandons in-memory waiting; it does not restore or retry operations.

The common review application joins the queue and existing Git proposals. It uses
their original trusted prompts and rejects ambiguous request IDs. It does not
reconstruct Git authority from summaries or own provider execution. The capability
service still validates saved scope, writes Policy durably, audits, rechecks current
Policy and Environment identity, and executes the exact prepared operation.

`approval.pending` and `approval.decide` are registered only on the existing
management Unix socket (Physical Host root/hacocoon group; trusted Host projection
root-only). No management endpoint is projected into an Environment. Decisions
contain only request ID, an explicit boolean and an optional saved-choice enum;
clients cannot replace targets, attributes or saved scope. Responses preserve the
actual result on failure but omit provider output and use fixed error categories.

## Notifications

The [interaction stream](../INTERACTION_EVENTS.md) remains read-only and minimized.
Its request ID is correlation data, never an approval token. The optional desktop
VS Code extension offers Review and a Hacocoon: Review Pending Approvals command.
Both open the ordinary CLI in a custom terminal owned by the local UI extension
host. Clicking supplies no answer. The operator sees the exact trusted prompt
and types the existing one-shot or saved choice, including a separate answer for ask.

Windows uses the installed local Hacocoon WSL distribution and its default operator
account. Linux uses the installed local Physical Host CLI. Executables are fixed
absolute paths, arguments are separate, and a small environment allowlist excludes
workspace/controller overrides. No remote shell or workspace task runs the command.
Only an explicit local user setting can select another installed WSL distribution.
Web, remote extension hosts, untrusted windows and unsupported platforms refuse review.

Duplicate panes for the same request are reused. Input and output are bounded;
subprocess control characters cannot alter terminal display. Closing, Ctrl-C/D or
fifteen-minute expiry terminates the local child and never retries or rolls back
an already submitted decision. Failed/unknown outcomes remain visibly unconfirmed.
Native OS notification activation remains planned. See [ADR 0029](../adr/0029-local-desktop-approval-review.md).

Repository JS tests cover routing, input, disposal, failures and notification clicks.
Installed GHA now probes the real custom terminal from a Remote-SSH editor with an
unpredictable stale ID, requiring the installed controller's refusal. This new probe
is pending; it does not prove an actual human's fresh approval or OS notification click.

See [ADR 0028](../adr/0028-pending-approval-sessions.md).
