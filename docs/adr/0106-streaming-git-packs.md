# ADR 0106: Stream Git packs through the existing authority boundary

[日本語](0106-streaming-git-packs.ja.md) | English

Status: accepted for implementation; installed acceptance remains separate.

## Context

The previous JSON/base64 transport stored whole packs at each boundary and refused
one pack over 32 MiB. Incremental history and sequential heads help existing
checkouts, but a larger new change still cannot transfer. Raising the memory
ceiling to repository size is not a suitable default.

## Decision

Standard Git uses bounded binary frames through the existing guest Unix HTTP
endpoint and the verified Host agent's stdin/stdout. Request metadata precedes
pack bytes; response metadata is a mandatory final receipt after bytes. Each
frame is at most 64 KiB, metadata is at most 2 MiB, and a single pack has a finite
16 GiB transfer bound. Memory for transport buffers is independent of pack size.
There are no transfer files, lookup tokens, guest-selected Host paths or new
management endpoints. Git objects are indexed in their existing repositories.

The broker retains its per-Environment slot, exact source/Workspace/incarnation
checks, per-head fetch decision and registry lock through agent completion.
The HTTP header bound remains five seconds; request bodies share the existing
ten-minute transport window because approval and streaming can outlast the old
30-second upload bound. The broker's nine-minute operation and agent's five-minute
execution deadlines remain. No background replay or resumable transfer is added.

Preparation imports objects with strict index-pack and the same finite size
bound, consumes the complete framed input and EOF, and validates commit ancestry.
The later push requires its separate exact-ref approval and audit, supplies no
pack, and retains expected-old/expected-absent leases and porcelain confirmation.
Clone, listing, fetch, hints and streamed bytes grant no push authority.

A truncated frame, excess length, missing terminator, trailing data, failed command,
failed output or mismatched final byte count fails the operation. A partial fetch
can leave ordinary unreachable Git objects; the helper reports success only after
both the final receipt and local index-pack succeed. It never updates refs on its
own. The source lock does not escape in a lazy response reader. Owned pipes are
closed and owned children are waited; no other user process is stopped.

## Rejected alternatives and acceptance

Do not increase whole-pack RAM buffers, spool duplicate complete packs on the
Physical Host, expose Host files/sockets, add chunk-token lifecycle state, or
relax read/push decisions. The old base64 protocol is replaced together on the
pre-1.0 client/controller/agent path; no compatibility fallback is retained.

Real Git and loopback/pipe regressions over 32 MiB establish the functional limit
correction, not giant-repository speed/capacity or installed Incus acceptance.
Large measurements remain deferred under the current user priority. Existing
LFS/submodule/force/delete/multi-ref push exclusions remain. This supersedes only
the whole-pack transport limit in [ADR 0102](0102-incremental-git-history.md);
its history and authority rules and [ADR 0081](0081-git-read-and-push-authority.md)
remain in force.
