# ADR 0071: Approval inside Windows notifications

Status: accepted for the development candidate; fresh installed answers and visual acceptance pending.

Date: 2026-09-13

## Context

Issue #568 requires an answer inside the Windows notification itself. The console
entry in [ADR 0030](0030-windows-notification-review.md) does not satisfy this
requirement. Public request IDs and protocol URLs carry correlation, not authority.
Windows selection controls require native COM activation for an unpackaged client.

## Decision

Replace the console handler with a hidden Windows helper. The installer owns a
separate per-user AUMID, COM class and fixed executable command for each WSL
distribution. Validate existing ownership before replacement and record ownership
before publishing a COM launch target. Normal workloads receive no new authority.

Reuse the [private review session](0067-local-gui-approval-session.md), existing
management access and common saved-rule builders. One in-memory presentation loop
owns at most sixteen reviews; each submitted answer uses a separate private child.
No second Policy engine, public decision endpoint or provider logic is introduced.

Show complete current conditions and the actual saved rule in bounded notification
pages. Require explicit navigation through them before the final answer. Default
to no saved Policy. Explain this Environment creation and all/future Environments.
Saving ask still requires an explicit allow/deny for the current operation.

Each displayed page has a fresh random single-use nonce. Only native COM input
can deliver a choice; body clicks, dismissals, URLs and process arguments never
answer. Immediately before submission, select again through a new private session
and compare the complete request and saved options. The common service rechecks
its snapshot and still owns claims, Policy persistence, audit and execution.

Private session tokens and answers stay on anonymous child pipes. Toast XML travels
to a fixed native rendering script over stdin, not argv; untrusted values are literal
XML text with control/bidirectional characters escaped. A read-only duplicate launch
acknowledges successful Show, not queueing or process creation. Notifications disabled
by the user or policy fail delivery without changing settings or granting permission.

Expire/remove stale reviews, invalidate all page nonces on restart, bound processes,
messages and deadlines, and cancel/reap owned children on exit. Do not reconnect or
resend an ambiguous decision. Report known saved Policy separately from failed or
unconfirmed execution. Cleanup failure remains an error; nonce invalidation does
not depend on successful removal of an old Windows notification.

## Rejected alternatives and verification

Reject terminal, browser or separate management-window fallbacks as completion of
this requirement, answers in URLs/public events, restored answer tokens, automatic
retry, truncated authority fields and native selection text used to rebuild Policy.

Native COM ABI and Windows notification-history tests are useful component evidence.
They are not proof of visible layout or fresh installed human decisions. Keep those
gaps explicit in [acceptance evidence](../status/acceptance-evidence.md).

The native contract follows Microsoft's
[activation interface](https://learn.microsoft.com/en-us/windows/win32/api/notificationactivationcallback/nf-notificationactivationcallback-inotificationactivationcallback-activate)
and [unpackaged registration implementation](https://github.com/microsoft/WindowsAppSDK/blob/main/dev/AppNotifications/AppNotificationUtility.cpp).
