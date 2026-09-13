# Git / GitHub Capability

Status: **partial, including development-branch changes.** Brokered host-side Git push exists; Hacocoon remains pre-1.0 and the concrete capability/CLI contract may still change. See [implementation status](../IMPLEMENTATION_STATUS.md) for current scope, separately from main integration and distribution.

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
branch, under a separate exact-ref decision. Registration selects checkout
provenance, not the set of permitted push targets.
Fetch revalidates each requested ref/OID against a fresh Host observation;
unknown, moved, duplicate or excessive refs are refused. At most 1024 heads and
a 32 MiB aggregate pack are accepted per helper batch. Discovery is checked
against both the all-heads scope and each exact ref; object fetch executes under
a new exact-ref decision. An exact-ref deny cannot be bypassed by broad discovery.
Separate head transfers may repeat shared history, a remaining M4 optimization.
The initial checkout branch never grants
push permission. See [ADR 0064](../adr/0064-git-read-scope-and-push-authority.md).
HTTPS GitHub authentication uses the trusted Host's `gh` credential store.
Creation records absence as the zero OID and requires an empty expected-value
lease at the remote. A competing creation is refused, including Git's successful
but unchanged result when the competing OID is identical. The returned porcelain
receipt must confirm this exact ref/commit mutation. Existing refs require
ancestry validation and the observed old-OID lease. Saved decisions retain the
exact ref and distinguish `update_kind: create` from `fast-forward`; neither
choice authorizes the other. See [ADR 0066](../adr/0066-git-branch-creation-authority.md).
Force push, branch deletion, multiple-ref pushes, LFS and submodules are
**deferred**. A transport failure after an external write can leave its result
unknown. The development candidate records exact dispatch/confirmation evidence
in the common capability audit and exposes read-only status/reconciliation. A
fresh exact-ref fetch decision and the original source owner/Environment generation
are required; matching remote content never upgrades an unknown push to success.
No approval or write is replayed. See [ADR 0070](../adr/0070-git-push-reconciliation-evidence.md)
and the [recovery commands](../guides/git-workflow.md#inspect-an-interrupted-push).
Generic retry/recovery and audit repair remain deferred.

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
and branch against its exact current Host source, alongside existing generation,
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
register the saved repository ID, URL and branch before connecting:

```bash
haco repo clone --branch main sample https://github.com/OWNER/REPO.git
haco git connect dev-imported
```

Here `sample`, the URL and `main` must match the saved Workspace route. If that
matching source is already registered, only `haco git connect` is needed. This
does not replace the imported checkout or its uncommitted/untracked/unpushed work.
Current Policy and approval still apply; imported data grants no credentials.
A missing source, mismatched URL/branch, or replaced Env/source identity cannot
reuse a connection. An offline import stays offline even if a same-name source
appears. Assigning a new route to offline data remains unimplemented.

Status: existing service composition verified by a component test combining
Workspace import, explicit source clone and broker connection, including mismatch,
offline and same-name replacement refusal. Native imported Git fetch/push remains
unverified; this is not a real-provider or network acceptance result.
