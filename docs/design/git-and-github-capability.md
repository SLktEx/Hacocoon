# Git / GitHub Capability

Status: **implemented in the repository; acceptance is scoped separately.** Brokered host-side Git push exists; Hacocoon remains pre-1.0 and the concrete capability/CLI contract may still change.

## Goal

The WSL PoC managed-repository workflow is implemented separately from
the historical shared-path broker below. It uses registered trusted-host
repositories, independent Incus volume copies and a Git-only remote helper
endpoint. Approval binds the registered upstream and exact target ref plus old/new OIDs;
the Environment never supplies trusted Git configuration or credentials. See
[ADR 0008](../adr/0008-managed-repository-workspaces.md). The implementation and
real-host acceptance status remain separate in
[implementation status](../IMPLEMENTATION_STATUS.md).

The product commands and manual setup are documented in the
[managed repository workflow](../guides/git-workflow.md).
This Standard integration implements `git.repository` through the existing
Policy/Approval/Capability/audit service. `fetch` authorization precedes remote
reads; `push` authorization follows object validation and proposal preparation.
Each proposal exposes the Environment, registered repository/upstream, ref,
old/new commit OIDs, operation and a bounded diff summary. The trusted client
decides its opaque, single-use ID. Approval cancellation or a changed remote
ref cannot silently authorize another push. Only Git objects cross from the
Environment; authenticated Git runs in the registered trusted repository.

The development candidate reads all upstream heads under an explicit all-heads
fetch scope. A push can create one absent branch or fast-forward one existing
branch, under a separate exact-ref decision. Workspace preparation selects checkout
provenance, not the set of permitted push targets.
Fetch revalidates each requested ref/OID against a fresh Host observation;
unknown, moved, duplicate or excessive refs are refused. At most 1024 heads and
one pack per head are accepted per helper batch. Packs stream through bounded
64 KiB frames and are indexed sequentially, with a finite 16 GiB per-pack limit
and 2 MiB metadata bound. Whole packs are no longer buffered as JSON/base64.
The exact final byte-count receipt and local index-pack must both succeed.
Source ownership locks remain held through the agent's stream completion; push
preparation consumes complete input before the separate approved write.
See [streaming transport](../adr/0106-streaming-git-packs.md). The head-count bound
remains in force. Discovery is checked
against both the all-heads scope and each exact ref; object fetch executes under
a new exact-ref decision. An exact-ref deny cannot be bypassed by broad discovery.
Fetch accepts at most 32 distinct local commit hints. Only ancestors of the freshly
verified authorized head are excluded from its pack; missing or unrelated hints
are ignored and never select another Host ref. Existing-target pushes exclude the
listed old commit when it exists locally; preparation still fetches and verifies
that exact remote target. A new-target push can reuse one advertised ancestor
after a separate exact-ref fetch confirms and retains it on the Host. A denied or
moved basis stops the operation before push. The target's old OID remains zero;
the normal creation approval and expected-absent lease still apply. Without an
available advertised ancestor, the helper retains the bounded complete pack. See
[incremental transport](../adr/0102-incremental-git-history.md) and
[new-branch history reuse](../adr/0104-new-branch-git-history.md).
The initial checkout branch never grants
push permission. See [ADR 0081](../adr/0081-git-read-and-push-authority.md).
HTTPS GitHub authentication uses the trusted Host's `gh` credential store.
Creation records absence as the zero OID and requires an empty expected-value
lease at the remote. A competing creation is refused, including Git's successful
but unchanged result when the competing OID is identical. The returned porcelain
receipt must confirm this exact ref/commit mutation. Existing refs require
ancestry validation and the observed old-OID lease. Saved decisions retain the
exact ref and distinguish `update_kind: create` from `fast-forward`; neither
choice authorizes the other. See [ADR 0081](../adr/0081-git-read-and-push-authority.md).
Force push, branch deletion, multiple-ref pushes, LFS and submodules are
**deferred**. A transport failure after an external write can leave its result
unknown; use the explicit status/reconcile operations below before a fresh proposal. Automatic write replay is unsupported.

Allow tools and agents inside a Hacocoon Environment to participate in Git/GitHub workflows without receiving broad, long-lived parent credentials.

## In scope

- Git/GitHub Capability provider.
- Repository, organization, branch, and operation policy dimensions.
- Allow / deny / human-approval rules for push-like privileged operations.
- GitHub App or another short-lived/scoped credential adapter when suitable.
- Auditable request/decision/result flow.
- Compatibility with ordinary Git/`gh` usage where a standard boundary can be preserved.

## Principle

Do not build a Hacocoon wrapper for every Git or `gh` command.

Prefer a narrow capability/credential boundary that lets normal tools keep their UX while Hacocoon controls the authority needed to cross the protected boundary.

The current implementation uses a host-side brokered push path with policy-visible repository/ref authority and does not export a broad host credential into the Environment.

## Worktree relationship

Worktree creation remains a Workspace concern from v0.2. GitHub authority is independent of who created the worktree.

## Human-in-the-loop

A policy can automatically allow a narrowly scoped push and require explicit human approval for another repository/branch/operation.

## Compatibility note

Git/GitHub authority must stay explicit even if the exact CLI, policy attributes, credential adapter, or transport changes. Pre-1.0 compatibility must not force broad ambient credentials or preserve an unsafe authority model.

## Branch-independent repository registration

`haco repo add <id> <remote>` registers one trusted source without a branch.
The Host clone stores all advertised branches. Existing source JSON
records with a `branch` field remain readable, but that legacy value no longer
selects a Workspace or grants branch authority. New registrations omit it;
Workspace, snapshot and transfer routing records keep their existing branch.
No Core database schema or owned volume is replaced.

`workspace create --repo <id> [--branch <branch>] <workspace>` resolves and fetches
an existing branch under the source lock before making the independent volume
copy. Without `--branch`, resolve remote HEAD. Collections/path preparation use
each remote default, as do new sources added during restore and local tree imports.
Tree imports preserve the supplied HEAD/index while resolving the registered
upstream default for broker discovery; saved members keep their saved routing.
The selected branch records initial checkout provenance;
it does not limit all-head discovery or replace independent exact-ref Policy.
The broker still pins source owner/URL, Workspace and Environment identity,
approval and old/new OIDs. An arbitrary guest ref grants no authority.

Registration uses the trusted `repository.add` stream with bounded individual
frames and an explicit final receipt. Sanitized native Git progress travels from
agent stderr through the Incus adapter and controller to client stderr; it never
enters protocol stdout or structured logs. Only native numeric progress and fixed
failure diagnostics are exposed, not arbitrary remote stderr, paths or URLs.

The agent has no blanket five-minute deadline. Client stream disconnect cancels
silent operations; the Incus adapter forwards interruption to the signal-aware
agent, which terminates Git and its helper process group. Broker fetch likewise
uses request cancellation without an elapsed-time cutoff. Its response interleaves
bounded sanitized progress frames with pack frames, preserving pack limits and
the explicit final receipt. Header and stalled-write deadlines remain. Provider ownership stays
reserved on incomplete creation. This is repository-level implementation;
installed Incus cancellation and large authenticated clones need separate
acceptance. See [ADR 0110](../adr/0110-branch-independent-repositories.md).

## Explicit source repository deletion

Implemented: `haco repo list [--json]` and `haco repo delete [--yes] <id>` expose
retained Host source checkouts and reviewed deletion. A source remains part of
current brokered Git routing even though Workspace filesystem copies are
independent. Every Workspace record with a configured Git source, including intermediate records,
therefore blocks deletion. This preserves local Git data and the ability to use
the existing approved transport. It is not an automatic unused-data collector.

The registry lock serializes clone, Workspace copy and source deletion. The service
rechecks the reviewed owner, refuses incomplete preparation, preflights native
storage, then uses the existing `deleting` record until positive native absence.
Git execution rechecks the exact source under the same registry lock after approval; queued requests cannot use a same-name replacement. The Incus adapter takes the existing Host lifecycle lock. It respects pending Host OCI-copy records, checks exact native volume
ownership and the specific current Host device, and detaches only that device.
Native snapshots, backups, schedules, extra users and changed devices fail closed.
A failed detach/delete keeps its receipt for explicit retry; no backup or rollback
is created. Remote repositories, credentials, independent Workspace/OCI/snapshot
data and Env permission generations are unchanged.

Schema 13 and existing repository records are preserved. No data migration is
required. Current snapshot Git provenance remains independent of a source checkout;
a restored Workspace may require explicit Git reconnection under current policy.
See [ADR 0045](../adr/0045-explicit-source-repository-deletion.md).

## Offline Workspace routing

Offline Workspace members have empty managed remote/branch fields. They never
select a Host source by name or appear in a Git broker binding. Mixed collections
bind only their configured members; every request rechecks the registered remote
against its exact current Host source, and pins the Workspace branch, alongside existing generation,
ownership and approval checks. Entirely offline Workspaces do not create a Git
endpoint. Guest Git configuration stays data. See
[ADR 0055](../adr/0055-offline-workspace-routing.md).

## Real push acceptance target

The legacy Git transport fixture is manually dispatched on trusted main and may
push only to `SLktEx/Hacocoon-test`. It uses the dedicated
`HACO_TEST_REPOSITORY_TOKEN` secret; without it, the workflow reports SKIP and does
not execute the push. Per-run test branches are retained with their commit receipt.
This fixture is not installed-product/import approval acceptance. See
[ADR 0059](../adr/0059-dedicated-git-push-test-target.md).

## Reconnect an imported GitHub Workspace

An imported Workspace with a saved GitHub route can use the existing commands.
On a new installation, authenticate GitHub in trusted Host as usual and explicitly
register the saved repository ID and URL before connecting:

```bash
haco repo add sample https://github.com/OWNER/REPO.git
haco git connect dev-imported
```

Here `sample` and the URL must match the saved Workspace route; its saved branch remains fixed. If that
matching source is already registered, only `haco git connect` is needed. This
does not replace the imported checkout or its uncommitted/untracked/unpushed work.
Current Policy and approval still apply; imported data grants no credentials.
A missing source, mismatched URL, or replaced Env/source identity cannot
reuse a connection. An offline import stays offline even if a same-name source
appears. Assigning a new route to offline data remains unimplemented.

Status: existing service composition verified by a component test combining
Workspace import, explicit source registration and broker connection, including mismatch,
offline and same-name replacement refusal. Native imported Git fetch/push remains
unverified; this is not a real-provider or network acceptance result.

## Push receipts and reconciliation

The Standard broker persists a dispatch receipt before an authenticated push and
a confirmation only after validating the exact porcelain result. The common
execution request identity correlates audit facts; it grants no authority.
`haco git status <env>` reads the latest push, while `haco git reconcile <env>`
performs a new exact-ref read under current Policy, original Environment creation
and source ownership. It never repeats a push or recreates an approval.
A matching remote OID remains an observation and cannot erase a failed original
operation. See [ADR 0086](../adr/0086-git-push-reconciliation-evidence.md).
