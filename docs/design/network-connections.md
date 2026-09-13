# TCP and UDP connections

Status: implemented Standard relay and CLI; scoped real Incus acceptance below. Windows UI, VPN and production-service acceptance remain separate.

## Ordinary development clients

The Standard connection relay shares the existing guarded Environment endpoint.
An Environment uses `haco network tcp` or `haco network udp` to expose an explicit
loopback port. Ordinary database, SSH and datagram clients use that local endpoint;
they need no proxy environment support. Each TCP client connection and each UDP
client association requests its own authority. The listener itself grants none.
The existing HTTP/HTTPS proxy and name-resolution paths keep their contracts.

The existing private management transport provides inspection, revocation and administrator-owned
Host service registration. Guests receive only the guarded data-plane API.
Remote Hacocoon administration, generic Host forwarding, management sockets,
Host credentials and agent scheduling are out of scope.

## Authority and identity

`network.connect/connect` binds the source Environment creation identity,
destination kind and name, destination creation identity when applicable,
protocol, port, pinned addresses and maximum duration. It uses the existing
Policy, pending approval and durable audit services. A completed approval without
completed audit never opens the upstream connection. Connection permission
does not grant database, SSH or other application permissions.

External targets may be public or LAN names/addresses. Host services are selected
from an administrator-owned exact endpoint registration. Environment targets
use sockets created inside a pinned, provider-verified destination network namespace
and bound to their creation identity;
a same-name replacement is a different destination. An external target must not
masquerade as a local management service or managed Environment. External sockets
are bound to their observed outgoing device so newly installed managed routes
cannot redirect an existing UDP association. Host registration rejects managed
Env addresses and the Standard endpoint/common privileged daemon ports. A socket
inside an Env namespace carries only application bytes; it exposes no namespace
descriptor or management authority to the source guest.

No session or single-use approval is inherited by Workspace copies. Existing
administrator rules keep their explicit scope; an Environment-scoped saved
choice applies only to its original creation. Global Policy continues to mean
an explicit administrator choice for current and future Environments.

## Resolution and address changes

DNS names use the existing separately authorized `network.resolve/lookup`
operation. Its answer is not a connection grant. The subsequent connect request
contains the actual pinned address set. The relay dials numeric addresses only,
without a second implicit resolver lookup. IPv4 and IPv6 destinations follow
the same authorization rules even when the guest transport uses IPv4.

An open TCP stream remains on its original peer until it closes, is revoked or
expires. DNS changes cannot redirect it. New connections resolve again and obtain
fresh authority. UDP associations likewise retain one connected peer; a new
answer never silently retargets an association. VPN names use the configured
platform resolver, without a public-DNS fallback. Application and OS caches are
separate from the uncached Hacocoon resolver.

## Lifetime and failure

TCP sessions have an absolute lifetime of 1–3600 seconds. UDP associations have
an absolute lifetime of 1–300 seconds and a shared 30-second idle lifetime and preserve datagram boundaries and one exact response
peer. Unrelated response sources are never forwarded. Expiry closes both relay
legs, including established streams. Explicit revocation closes the active
session. Controller shutdown closes all sessions; startup never replays them.

Policy edits, unreadable Policy, source/destination replacement or revoked Host
registration invalidate affected authority. Active authority is checked every 500 ms with a further 500 ms check bound;
failed or stalled checks close the connection. Socket deadlines enforce absolute
expiry independently. Data already accepted by the peer cannot be withdrawn.
Any Policy edit conservatively invalidates existing sessions, including unrelated
edits. A saved approval binds its newly authorized association to the post-save
revision after the existing Capability service has re-evaluated it. Rule expiry is evaluated independently of a session's maximum lifetime.
The durable audit retains the authorization history; active session inspection
does not invent persistence or reconnection after a controller restart.

Inspection reports protocol, destination, creation identities, request ID,
state, phase (resolve/authorize/connect/active), expiry and an allowlisted
termination/failure category. Pending requests
remain in the existing approval view. Observed DNS failure, Policy refusal,
unreachable-network errors, connection refusal and timeouts are distinguished.
UDP silence cannot prove remote refusal or delivery; it is reported as an
observation timeout. Payloads, credentials and raw backend output are not logged.

## Acceptance

Required repository coverage includes real loopback TCP/UDP data, denial before
dial, approval/audit failures, expiry, active revocation, generation replacement,
DNS rebinding, malformed frames, bounded association counts and cancellation.
Real Incus and installed CLI journeys separately cover external/LAN fixtures,
registered Host services, Environment peers and retained HTTP/HTTPS behavior.
Windows access uses dedicated files and ports. VPN/NRPT and large-repository
performance must be reported unverified unless those actual fixtures run.

See [managed networking](managed-sandbox-network.md),
[name resolution](name-resolution.md), [Policy foundation](policy-and-capability-foundation.md)
and [the relay decision](../adr/0061-policy-bound-development-connections.md).


## Minimal commands

On the trusted management side, select one service and create a creation-scoped,
time-bounded rule. A registration itself grants no connection permission:

```bash
haco network host add --address 127.0.0.1 --port 5432 database
haco network rule --env demo --kind host --target database --protocol tcp --duration 5m --decision ask --ttl 1h
haco network list
haco approve
```

Inside that Env, point an ordinary client at the returned/local listener:

```bash
haco network tcp --kind host --target database --listen 127.0.0.1:15432 --duration 5m
# Another terminal:
psql -h 127.0.0.1 -p 15432
```

Use `--kind external --target db.example --port 5432` for an external/LAN
service, or `--kind environment --target other-env --port 5432` for a peer.
External names need a separate `network.resolve/lookup` rule through
`haco config`. Use `udp` with an explicit port for datagram clients. UDP reply
silence is inconclusive; the relay never reports it as delivery success.

`haco network rule` defaults to the source creation scope. Its explicit
`--scope environment` follows a source name across recreation and `global`
covers any source; destination generations and resolved addresses remain exact.
Rule lifetime is at most 31 days. `ask` maps to the existing
`require-approval` decision. Saved approvals use the existing review commands.
`haco network revoke <id>` closes one association; a later connection still
requests fresh authority and can be allowed by a remaining rule. Remove/restrict
that Policy too when denying future connections.

Host-to-Env access uses the existing private management API and Incus loopback
proxy lifecycle:

```bash
haco env forward --protocol udp --target-port 9000 demo
haco env disconnect demo <connection-id>
```

TCP remains the default, including existing SSH/HTTP preview behavior. Explicit
forward devices last until disconnect or Env deletion; they are not guest
Capability sessions.

## Development acceptance

See [commit-bound acceptance and unresolved Windows-service failures](../status/acceptance-evidence.md#development-branch-integration).

## Client-side application access

`haco env tunnel --target-port 8080 demo` creates a client-owned loopback TCP
listener through the private controller stream, without a persistent Incus proxy.
Its location, defaults, creation binding, authority and Windows limits are owned
by [client TCP listeners](controller-client-transport.md#client-tcp-listeners).
Guest-originated network permissions remain the separate contract above.
