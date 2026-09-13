# Pending approval review

[日本語](pending-approval-review.ja.md) | English

Status: **VS Code GUI and Windows notification review implemented on the development candidate; fresh installed-answer acceptance partial**.
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

## Shared Git and network decisions

Git proposals and HTTPS/network requests use the same `haco approve` choices,
readable receipts, saved Policy, audit and pre-execution reevaluation.

| Choice | Current request | Later matching requests |
|---|---|---|
| One-shot allow / deny | Explicit answer | No saved change |
| Environment allow / deny / ask | Apply saved decision; ask needs another answer | Same Environment creation identity only |
| Global allow / deny / ask | Apply saved decision; ask needs another answer | All Environments, including future creations |

Recreating an Environment with the same name never inherits its Environment-scoped
choices. Global choices intentionally apply to future Environments. Saved decisions
do not widen the target: Git retains repository, remote, ref and update kind;
network retains canonical hostname, protocol and port. Only a trusted Git provider
may generalize changing operation/old/new commit identifiers for its supported
reusable scope. Explicit deny and mandatory isolation still take precedence.

HTTPS CONNECT authorizes a connection, not each HTTP request carried inside it.
Encrypted URL paths and methods are not visible or enforceable at this boundary;
they are not presented as saved constraints. The original client reports the
actual TLS/HTTP outcome. Domain-level approval does not imply URL-level approval.

Common regression tests compare both capability identities across ten CLI answers
and six saved choices, including audit scope, reevaluation, changed targets and
same-name recreation. These tests exercise the shared boundary; they do not execute
Git pushes or establish network connections.

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

## VS Code GUI review

Status: **implemented development candidate; installed GUI acceptance pending**.
The optional local UI extension opens a Webview from Review or **Hacocoon: Review
Pending Approvals**. Inspect the complete current operation, choose whether to save
Policy, then click **Allow this operation** or **Deny this operation**. No terminal
input is required. Labels follow the VS Code English/Japanese display language.
The command palette works without the notification bridge.

No saved Policy is the default. The dropdown offers only scopes returned by the
common Policy builders. The full rule is displayed, including exact Git ref/update
kind or network hostname/protocol/port. Environment scope binds this creation;
global scope includes future Environments. Saving ask still requires an explicit
current answer. Selecting, refreshing and opening a notification do not answer.

A fixed local installed CLI uses private child-process pipes to the existing
management API. Windows selects its installed Hacocoon WSL distribution using only
the local user setting; Linux uses the local Physical Host. Web/remote extension
hosts, untrusted windows and unsupported platforms refuse review. Workspace values,
provider output and public events cannot choose executable paths, credentials or
controller sockets. No management authority is projected into an Environment.

A private selection token binds the complete displayed request. Immediately before
submission, the adapter compares a fresh snapshot including Environment creation
and saved scope, then calls the common decision service. That service retains its
single-consumer claim, Policy validation/persistence, audit and execution. Tokens
never enter argv, logs, URLs or the read-only event bridge. They are not a replacement
for the management endpoint's existing authorization.

One local panel is reused. Pending requests refresh every five seconds when idle;
submission consumes its selection before a fallible call. Closing or the fifteen-minute
presentation deadline stops the local child without retry or rollback. The queue's
shorter request deadlines remain unchanged. Read/protocol/transport failures disable
or invalidate the selection; an unknown submitted outcome remains unconfirmed.
The receipt distinguishes denial, actual successful execution, saved Policy and
incomplete audit. Inspect Policy and audit before retrying an uncertain result.

The Webview denies network/command/local-file resources, uses a nonce CSP and renders
untrusted values as literal text without truncating authority fields. Display snapshots,
pending counts, input/output and stderr are bounded. The private protocol is an internal
presentation interface, not a public plugin API. See [ADR 0067](../adr/0067-local-gui-approval-session.md).

Repository tests exercise actual renderer clicks, saved scopes, stale/changed requests,
trust revocation, duplicate answers, malformed output and cancellation. Installed
acceptance observes the real Webview handshake and the installed controller's stale
request refusal without injecting decisions. Its new result is pending. Earlier custom
terminal successes are historical evidence, not GUI acceptance; see
[acceptance evidence](../status/acceptance-evidence.md). Windows notification-contained
fresh answers remain a separate open part of Issue #568.

## Answering inside Windows notifications

Status: **implemented on the development candidate; fresh installed answers and
visual layout acceptance remain pending**.

The Windows installer registers the hidden helper, protocol correlation entry,
notification identity and COM activator for its own WSL distribution. Owned Host
setup subscribes through the existing controller transport. `-SkipDesktopReview`
still skips registration and disables the owned notification service.

A pending notification opens the complete review in the OS notification itself.
Use Next/Previous to inspect every current condition, choose whether to save Policy,
then inspect the actual saved rule before allowing or denying the current operation.
No saved Policy is the default. This Env means this creation only; all Envs includes
future creations. Saving ask still needs an explicit current answer. Body clicks,
display, closing, reopening and URLs never answer. No terminal, browser or separate
management window is required by this implementation.

Long authority values continue onto further pages without truncation. Text is literal,
with control/bidirectional characters escaped. Selection controls contain only common
saved choices. Each successfully shown page gets a private, single-use native nonce.
The final intent is reselected through a separate private child and compared with the
complete displayed request and saved options before the common service decides.
Controller tokens/answers never enter argv, URLs or public events. Core gains no
Windows-specific Policy or execution logic; ordinary Environments gain no authority.

The helper validates installed per-user/per-distribution ownership and fixed launch
paths. Duplicate launches ask the existing COM server to show that read-only request
and acknowledge actual Show. At most sixteen reviews are active; pending state refreshes
every three seconds, admitting at most one new notification per refresh. Removed requests
lose their buttons, and the native notification also has a two-minute expiry. The helper
and private peers have a fifteen-minute maximum lifetime; submitted children have a
five-minute bound within it. Shutdown cancels/reaps owned children and clears owned
review notifications. Old nonces remain invalid even if native cleanup fails.

Display, protocol, registration and transport failures never imply approval. A submission
is consumed before any fallible call and is not automatically retried. Receipts distinguish
denial, successful operation with audit, known saved Policy, failed operation and uncertain
results. Check current Policy and audit before retrying an uncertain result. Native
diagnostics expose only fixed stages/status values, not raw subprocess output.

See [ADR 0071](../adr/0071-notification-contained-approval.md). Windows COM callbacks and
English/Japanese selection XML in native history have component coverage. This does not
prove visible layout, human clicks or fresh installed decisions. Previous console/URI
acceptance, the historical checksum failure and remaining native gaps stay in
[acceptance evidence](../status/acceptance-evidence.md). Linux native activation remains planned.
