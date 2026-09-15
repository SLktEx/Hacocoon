# ADR 0100: Notification session readiness

Status: accepted for the development candidate.

## Context

The per-user/per-distribution mutex covers the complete native review lifetime,
including initial private WSL readiness and final owned cleanup. A busy mutex
therefore does not mean its COM presentation endpoint exists. Dispatch during
those windows may ask Windows to start a second server which cannot take ownership.
CI recorded COM creation failures; this observable race is a correction candidate,
not proof that every recorded failure has the same cause.

## Decision

Keep the mutex as the lifetime owner and use a separate manual-reset Windows event
for initialized presentation readiness. Wait for ownership or readiness together,
preferring ownership if both are signaled, and clear stale readiness when taking an
abandoned or released mutex. Publish only after initialization. Withdraw before
COM revocation and hold the mutex through cleanup. Bound predecessor waiting and
own startup separately; the notifier covers both. Keep the same-thread mutex rule.

The event only coordinates availability. Installed registration validation, private
controller transport, single-use page nonces, final request reselection and actual
Show acknowledgement remain authoritative. An event signal never approves or
acknowledges an operation. No automatic decision retry is introduced.

## Rejected alternatives

A longer COM call timeout still attempts dispatch to an unpublished endpoint.
Releasing ownership before cleanup permits overlapping history/peer cleanup and
new presentation. Registering COM before private readiness exposes an endpoint
that cannot yet serve its request. Retrying a failed approval risks duplicate effects.

See the [owning contract](../design/pending-approval-review.md).
