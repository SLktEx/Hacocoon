# Client TCP forwarding through controller sessions

Status: accepted; implementation candidate. [日本語](0072-client-stream-forwarding.ja.md)

## Context

An Incus proxy on the Physical Host does not provide a listener in the client
process's network namespace. Clients must not need Incus administration or
direct access to an Environment bridge to reach an application service.

## Decision

`haco env tunnel` owns a numeric loopback TCP listener and opens one private
controller byte session per accepted connection. Preparation selects the ready
catalog entry and exact creation identity; it creates no provider resource.
Every connection rechecks that identity, readiness and runtime state. The Incus
adapter reuses its verified, pinned network namespace socket path. Only an
explicit numeric loopback address inside the selected Environment is accepted.
The listener's creation identity never follows a same-name replacement.

Management socket permissions remain the authority for this Host-to-Env path.
The methods are registered only on that endpoint. The guest Git, DNS and
network endpoints cannot use them. Guest-originated connections retain their
separate Capability/Policy/approval and audit requirements. Neither an Env
name nor a returned creation identity is an authorization credential.

The existing session transport gains explicit byte-stream completion. A peer
half-close is delivered before waiting for final completion, so one side can
finish sending while receiving the rest. Upstream readiness/failure precedes
application bytes; transport EOF alone does not prove a successful operation.
Closing before both half-closes sends `_control.session.cancel` over the existing
private management endpoint; acknowledgement waits for the stream worker to
finish. This also stops a target that ignores EOF. Cancellation is not encoded
as application bytes. A lost controller connection cannot prove remote cleanup;
the controller's absolute stream deadline remains the fallback bound.
Socket creation occurs within the owned stream callback after transport setup,
avoiding resource leaks if initial acknowledgement fails.

The client limits a listener to 16 concurrent connections and 1 second–1 hour.
Controller preparation/dial is bounded to 10 seconds; each stream to 1 hour.
Cancellation closes listeners and sockets and joins copy workers. No proxy
device, persistent forwarding object, automatic restart or reconnection is
created. Data and lifecycle ownership are unchanged.

## Rejected alternatives and remaining scope

- Creating more Incus proxy devices does not place the listener in the client.
- A new TCP management server or guest management socket widens authority.
- Process-style implicit completion at EOF deadlocks valid bidirectional
  half-close protocols; byte sessions require explicit completion instead.
- Arbitrary external destinations through the target namespace would turn
  this application-access command into an additional outbound proxy. Existing
  separately authorized network connections own that use case.

Native Windows listener transport through `wsl.exe`, broader generic process
stream consolidation and VPN/DNS modes remain separate M3 work. Running a
Linux client in WSL or trusted Host places its listener there; it is not
evidence of a Windows-native listener. See the [owning contract](../design/controller-client-transport.md#client-tcp-listeners).
