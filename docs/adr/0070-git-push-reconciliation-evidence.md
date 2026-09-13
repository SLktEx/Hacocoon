# Durable Git push evidence and read-only reconciliation

Status: accepted on the development candidate.

## Decision

A remote write and its local acknowledgement cannot be committed atomically.
The Standard Git broker records `git-push-started` in the common, synchronized,
fsynced capability audit before invoking the authenticated backend. It records
`git-push-confirmed` only after the exact single-ref porcelain result has been
validated. Both receipts carry the common execution request identity, exact
Environment generation, source owner, registered remote, ref and old/new OIDs.
Request identity in an execution context is correlation metadata, not authority.
Source-owner receipt metadata does not change Policy attributes or saved scopes.

A failed start-record write prevents dispatch. An acknowledgement or subsequent
audit failure remains visible; neither failure permits automatic replay. The
common completion event and the provider confirmation receipt are distinct facts.
An interrupted approval is not recreated. A historical request without a start
record is displayed as unconfirmed and cannot select a remote for reconciliation.
This conservative rule also covers older versions without provider receipts.

The trusted `git.status` and `git.reconcile` APIs default to the latest requested
push for one Environment; an optional request identity selects older evidence.
They use the existing bounded audit reader, with a fixed file-size boundary and
complete newline-terminated JSONL records. Corruption produces an error, not a
partial successful status. There is no second journal or guest recovery endpoint.

Reconciliation refuses an active original operation and requires the original
Environment generation, source owner and currently valid binding. It performs a
new exact-ref fetch Policy/approval request, checks the generation again after
that wait, then uses the common source registry lock and backend. Only a fixed
`ls-remote --heads` observation is allowed. The observation is separately audited
and correlated to its new read request. No write, approval restoration, credential
export or lifecycle mutation is part of reconciliation.

## Interpretation and rejected alternatives

Remote equality means only that the ref currently has that value. Another actor
could have written the identical commit, or changed and restored the ref. An
observation therefore reports matches-new, matches-old, absent or diverged while
preserving the original confirmed/unconfirmed state. It never converts an unknown
push into success. Retrying a new push must use the ordinary fresh proposal,
Policy and exact old-OID/absence lease.

Rejected: retrying after transport loss, treating matching remote OIDs as our
receipt, rebuilding pending approvals from audit records, rebinding historical
names to current resources, and adding a separate mutable recovery state machine.
The immutable audit supplies evidence; current registration and Policy supply
authority. Force/multi-ref push, deleted-resource recovery and audit repair remain
outside this slice.

## Validation boundary

Local real-Git fixtures cover a write followed by lost acknowledgement, restarted
broker state, identical competing creation, fresh read denial, exact generation
and source ownership, and start/confirmation/observation persistence failures.
Repository tests are not authenticated external Git or installed Windows/WSL
acceptance. See the [Git guide](../guides/git-workflow.md).
