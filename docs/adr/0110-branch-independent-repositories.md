# ADR 0110: Register repositories independently of Workspace branches

Status: accepted. Updates the source-branch assumptions in
[ADR 0008](0008-managed-repository-workspaces.md) and
[ADR 0055](0055-offline-workspace-routing.md).

## Decision

One registered source identifies a remote repository, not a branch. The canonical
command is `haco repo add`; the former `repo clone --branch` command is removed.
The trusted source stores all advertised branches. Workspace preparation resolves
and fetches one existing branch, defaulting to remote HEAD, before an independent
copy. That selected branch belongs to the protected Workspace route.

The Workspace branch records initial checkout provenance, not an authorization
limit. All-head discovery and each exact-ref read still require their existing
Policy checks. Push approval binds the requested ref and old/new OIDs. Source
owner, URL and Environment identity remain pinned. A guest ref never grants authority.
Different Workspaces may use different branches of the same source without
duplicating registration or credentials. Offline data remains offline.

Existing source records may retain their legacy branch field for read compatibility;
it is ignored for branch selection and authorization. Workspace and snapshot/transfer
records retain their branch semantics. No owned data or Core schema is migrated.

Repository registration has a streaming management method and an explicit final
receipt. Git transfer counters go to stderr through a bounded line filter; only
numeric native progress and fixed credential-free failure diagnostics are exposed.
Raw remote messages, paths and URLs are not forwarded or logged. A silent Git
process is canceled by request lifetime, with controller disconnect propagated through the Incus exec signal channel
to the signal-aware agent, rather than a blanket five-minute timeout. Repository registration is
idempotent for the same repository ID and remote: retries reuse the durable owner, reconcile an
ambiguous Incus volume create against that exact owner, resume population from a created volume,
and accept an already-ready registration without replacing it. A different remote is never adopted
under an existing ID. Recovery-required remains only when the provider state cannot be verified.

## Rejected alternatives

A deprecated branch-bearing registration alias would preserve the wrong model.
Using the Host clone's checked-out branch as authority would couple independent
Workspaces. Trusting a guest-selected ref would broaden approved access. Replacing
source metadata or volumes during upgrade would risk existing ownership and data.
Passing raw Git stderr through a generic token regex would miss arbitrary remote
credential echoes. A longer fixed timeout alone would still fail healthy transfers
and would not cancel silent work on client disconnect.

## Consequences

Single-repository Workspace creation supports explicit branch selection. Collections
and path preparation use each source's remote default. All-head fetch, independent
ref approval, new-branch push and the 16 GiB per-pack streaming limit remain.
Bounded progress frames share the broker response with pack frames, but never
count as pack data or replace the final receipt. Fetch has no elapsed-time cutoff;
header and stalled-output deadlines still bound transport setup and backpressure. Installed Incus disconnect handling and large/private repository acceptance
must be measured separately from repository tests.
