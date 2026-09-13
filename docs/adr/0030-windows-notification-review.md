# Windows notification review

## Trusted Host subscription

Normal setup delivers the notification companion with the existing source,
digest and Host-ownership checks. The already trusted management transport is
reused for events; the public notification schema is projected in the client.
A failed controller subscription must not silently change to local audit reading.
The Windows installer captures a validated, persistent distribution identity for
Host projection; conflicting existing identities are rejected. This keeps routing
out of workload input without adding another user command or raw-state mount.


Status: accepted; Windows adapter implementation and installed acceptance in progress.

## Context

Roadmap D2 makes OS notifications the primary review entry while keeping VS Code optional.
A notification may select a request but must not carry a command, answer, credentials or
management routing. Multiple installed WSL distributions must not replace one another's route.

## Decision

The optional Windows adapter registers one user protocol and notification identity per
distribution. Its scheme derives from the normalized distribution name; owned registration
and application files must agree with that exact distribution before an update.
The installer supplies a checksum-verified native helper and a local distribution configuration.

The helper accepts exactly one canonical URI containing a 32-character lowercase request ID.
It rejects alternative authorities, escaping, query strings, fragments, extra arguments and
unsupported platforms. It launches fixed Windows System32 WSL, then fixed Linux haco approve,
with separate arguments and a minimal environment. It never runs a shell or supplies an answer.
The operator inspects the existing trusted scope and explicitly answers in the console.

Only approval-required events offer activation, and only when the matching local registration
exists. Other events and malformed IDs remain presentation-only. A link is correlation, not
authorization: websites or other apps may invoke it, but cannot approve or change the request.
The existing controller rejects stale requests and owns saved Policy and execution.

## Rejected alternatives

- Arbitrary command strings in toast launch attributes or shell interpreters.
- One global registration silently redirected by installing a second test distribution.
- A public HTTP decision endpoint attached to the read-only notification bridge.
- Treating activation, notification delivery or missing clients as approval.
- Bypassing disabled native WSL interop with /init, or changing binfmt registrations from the notification adapter. Missing native registration recovery stays with the existing WSL setup.

## Limits and acceptance

Windows helper installation and protocol registration are separate from Core and guest authority.
Linux desktop activation remains a follow-up. Native presentation, protocol activation and fresh
decision acceptance require distinct real-desktop evidence; command success alone does not prove
a visible notification. Preserve earlier failed runs and report absent evidence explicitly.

## Native client state ownership

A native client pins its private state directory and holds an OS process lock
through observation and delivery. Save uses an exclusive random file and synced
atomic replacement. Predictable temporary names can redirect writes through a
link; unlinking the lock can split ownership across inodes. Both are rejected.
This is client-owned presentation state, not a capability authority or an
exactly-once delivery guarantee. Windows setup now owns optional background startup after desktop registration.

Notification startup inspects failure state before resetting it. A fresh inactive
unit has no failed state and may be unloaded by systemd; it must still start
normally. Existing failed units retain the explicit reset path. Unknown inspection
results fail setup rather than being treated as a successful start.

Repeated setup reuses an active owned notification service when its full unit
configuration and executable revision match. The unit records a SHA-256 revision
of the installed, protected Physical Host companion; a new binary or configuration
therefore still restarts the service. Healthy refresh does not consume systemd
start-rate limits. Inactive/failed service recovery and opt-out remain explicit.

The console presentation is historical and superseded by [ADR 0071](0071-notification-contained-approval.md). The trusted subscription and distribution ownership boundaries remain.
