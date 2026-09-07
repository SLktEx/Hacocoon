# Windows notification review

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
