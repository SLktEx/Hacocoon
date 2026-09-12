# ADR 0028: Pending approval sessions

[日本語](0028-pending-approval-sessions.ja.md) | English

Status: accepted; repository implementation, installed acceptance pending.

Background network requests need human review without daemon stdin or ambient
permission. A replaceable Standard queue provides bounded waiting. Core's capability
application owns admission after Policy evaluation, the decision, persistence,
audit, final Policy/identity checks and execution.

An admitted approval session binds waiting and completion by object identity.
Completion is delivered only after the capability service has its actual return
value. A rejected duplicate never receives a session, and stale completion cannot
release a replacement. Bounded channels let the application finish after the
reviewer disconnects. Requests are not replayed after process restart. Review
cancellation after submitting a decision is an ambiguous-outcome case, not rollback.

The common review application joins independent sources behind a small interface.
Git retains its exact prepared operation and existing pending-decision ownership;
it exposes the original approval prompt rather than reconstructing scope. Duplicate
request IDs across sources fail closed. No provider-specific Core conditionals or
new Environment/Workspace lifecycle mutations are introduced.

Only the existing authorized management socket accepts review operations. Its
principal controls Hacocoon already; an Environment and the read-only event bridge
do not receive that authority. A notification request ID identifies an operation
but is never a bearer token. No browser credential, permissive localhost POST,
guest management socket or notification-driven automatic decision is added.

Rejected: unbounded waiting, background reads of stdin, queue acknowledgement as
proof of Policy persistence, ID-only completion ownership, restoring unfinished
approvals after restart, reconstructing Git authority from UI fields, forwarding
raw provider output to review clients, and accepting decisions through the public
presentation stream. See the [feature contract](../design/pending-approval-review.md).
