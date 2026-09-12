# ADR 0007: Keep installed Standard egress inside the controller

Status: accepted  
Date: 2026-09-06

## Context

The canonical Environment provider denies direct traffic, but the installed
controller constructed the Standard proxy without serving it. Running the legacy
foreground command separately would create another Policy and state composition.
A plain HTTP server shutdown also leaves hijacked HTTPS CONNECT sockets outside
its connection lifecycle.

## Decision

The installed Physical Host controller unit enables `--standard-egress`. The
same composition owns Policy, audit, persisted source identity and the Standard
proxy. The Incus adapter prepares and verifies its existing proxy-only guards
before the controller binds the fixed IPv4 endpoint. Preparation has a 30-second
deadline. Preparation or bind failure prevents the control service from starting.
The installer explicitly supplies nftables and waits for the controller socket.

A bare controller remains an isolated control-transport entry, without the
Standard listener. This is an explicit deployment choice, not an Environment
fallback. The installed unit always enables Standard egress. No client flag,
guest service, second controller, arbitrary listen address, NAT or firewall
disablement is introduced.

Both listeners share a cancellation scope. An independent exit of either
service stops its peer and is a process failure for systemd restart. Shutdown
closes all accepted proxy sockets, including hijacked tunnels, and cancels
request contexts. CONNECT also closes the client and upstream when its request
is canceled, including during ClientHello and prefix writes. The service retains
at most 256 accepted connections, bounds headers to 16 KiB, and keeps the existing
ClientHello/SNI and public DNS pinning checks.

The daemon composition has no ambient approval provider. Missing Policy denies
access; an exact allow remains scoped and audited; require-approval fails closed
without reading stdin or emitting request details to the journal. Existing
interactive control sessions may supply their scoped approval callback. This
does not add a proxy approval UI.

The HTTP server's error sink emits only a fixed structured failure message.
Arbitrary panic text, headers and stacks are not copied to logs.

## Rejected alternatives

- A legacy foreground broker or a second composition inside haco-host.
- Direct/NAT access while the proxy is unavailable.
- Treating daemon stdin or another connection's answer as approval.
- HTTP Shutdown alone, which does not close hijacked CONNECT sockets.
- Continuing to serve controller requests after the installed proxy has exited.

## Validation and limits

Component regressions exercise service coupling, real socket shutdown at each
CONNECT blocking stage, ambient approval refusal, exact Policy decisions,
selected error logs and the installer's actual unit-generation function.
These are repository checks. Installed Windows Environment allow/deny traffic,
firewall reload/startup order and ordinary policy management require their own
acceptance; they are not established by trusted-host connectivity.

See [egress authorization](../EGRESS_AUTHORIZATION.md) and
[implementation status](../IMPLEMENTATION_STATUS.md).

