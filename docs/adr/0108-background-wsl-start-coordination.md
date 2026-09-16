# ADR 0108: Coordinate background WSL starts with reclamation

[日本語](0108-background-wsl-start-coordination.ja.md) | English

Status: accepted for the implementation candidate; installed acceptance pending.

## Evidence and decision

PR #707 at c0843ed4, Windows job104622907624, observed all WSL launch/host
processes disappear after reclamation requested shutdown. At 23.8 seconds two
launches returned with notification ancestry. Compaction remained refused because
the disk was attached. This identifies a Hacocoon background-start race, not every
cause of attachment: #708 also retained host processes without new observed starts.

A shared Windows coordination primitive now owns kernel-name reservations.
Reclamation retains its existing user-SID/exact-GUID reservation and additionally
holds a user-SID/distribution-name launch reservation through identity checks,
Linux discard, stop, compaction and resume. Native notification initial and answer
peers acquire that same launch reservation until a bounded read-only handshake
completes. Failure closes and reaps the child before releasing the reservation.
A contender refuses without starting WSL, waiting for a decision, or replaying an
answer. A later explicit review can establish a fresh session.

The name is lowercased and hashed because Windows distribution names match without
case. This extra name scope coordinates starts only. It is never disk ownership,
installation identity, user approval or permission to stop. Exact registration,
installation, pinned file and detached-disk checks remain mandatory. Unrelated
distributions use separate launch reservations. The primitive is outside Core.

## Native contract and limits

[CreateMutexExW](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createmutexexw)
returns an existing-object handle together with ERROR_ALREADY_EXISTS. Contenders
close that handle and refuse. An error or a colliding object type also refuses.
New noninheritable handles retain the reservation until close; no thread-bound
mutex ownership is used. Current-user/System ACLs and per-user global names reuse
the existing continuation behavior. Precreation can deny service, never grant
mutation authority. No credentials or guest authority are introduced.

This coordinates cooperating Hacocoon notification launches. Raw WSL commands,
external editors and already running sessions can still keep a disk attached;
compaction continues to fail closed. The gate is transient; durable reclamation
records retain interrupted/failed outcomes across crashes. No automatic retry,
forced disk detach, other-distribution stop or longer compaction budget is added.

## Rejected alternatives and checks

A check followed by immediate release races with process creation. Releasing just
after Start races with a delayed WSL child. Holding the gate throughout a human
approval session prevents reclamation indefinitely. Name-only disk authorization
would discard the established ownership contract. Killing all notification/editor
processes or changing test setup would hide the ordinary product conflict.

Native regressions cover cross-process exclusion, case-insensitive scope,
unrelated distributions, release after a losing contender, successful private-peer
startup after release and failed-readiness cleanup. Installed reclamation and
actual notification behavior remain separately recorded acceptance checks.
