# ADR 0067: Private local GUI approval sessions

Status: accepted for the development candidate; installed GUI acceptance pending.

Date: 2026-09-13

## Context

M2 and Issue #568 require VS Code review and decisions to finish inside the GUI.
The custom-terminal presentation in [ADR 0029](0029-local-desktop-approval-review.md)
kept authority local but required typed CLI answers. Notification request IDs
remain public correlation data and must never become answer credentials.

## Decision

Replace the VS Code terminal with an isolated Webview owned by the local UI
extension. Use fixed installed CLI/WSL executables and private child-process pipes
to a bounded `_desktop-review` presentation session. The process uses the existing
operator's management access; the hidden command is not an authorization bypass.
The ordinary public `haco approve` terminal requirement remains unchanged.

The session calls the existing `approval.pending` and `approval.decide` endpoints.
Selection reads a complete sanitized request and derives each available saved
Policy from the common scope builders. An unpredictable private selection token
binds the displayed snapshot. Before submitting, reread and compare the entire
request, including Environment creation and reusable scope. Consume the selection
before fallible calls. The common service still owns Policy validation, persistence,
single-consumer claims, audit, reevaluation and provider execution. Core gains no
VS Code logic, GUI dependency, new Policy engine or public event-write endpoint.

Show the complete current target and actual saved rule without truncation. Default
to no saved Policy. Explain Environment creation versus global/future scope.
Saving ask requires an explicit current allow/deny button; opening, selecting,
refreshing, closing and notification body clicks never supply an answer.

The Webview has no network, command URI or local-file resources. A nonce-only CSP
permits the bundled renderer; all request text uses textContent, with control and
bidirectional formatting characters escaped. Recheck desktop/local/trust state on
every message. Bound requests, replies, pending counts, stderr and session lifetime.
Protocol mismatch, EOF, stale/duplicate answers and transport loss fail closed.
Never resend; an accepted decision may survive client cancellation. Display actual
receipt, saved choice and audit state separately from submitted intent.

## Alternatives and scope

Reject answers in protocol URLs or the read-only notification stream, remote
workspace execution, arbitrary executable/controller settings, web endpoints with
ambient localhost authority, and interpreting a provider string as executable HTML.

Windows notification-contained answers are a separate remaining part of #568.
This change does not claim them: its existing URI still only opens review.
Native activation must authenticate the local notification/helper route; knowing
a request ID or sending an arbitrary URI must not authorize a decision.

See the [owning contract](../design/pending-approval-review.md) and official
[VS Code Webview security guidance](https://code.visualstudio.com/api/extension-guides/webview#security).

The Windows remainder above is now implemented separately by [ADR 0071](0071-notification-contained-approval.md); fresh installed notification acceptance remains pending.
