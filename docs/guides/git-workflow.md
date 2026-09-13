# Managed Git operations

[日本語](git-workflow.ja.md) | English

Start with [your first Environment](getting-started.md). This page covers Git Policy,
multiple repositories and saved approvals. Authentication stays in trusted
`haco-host`; the Environment gets only the Git-specific broker endpoint.

<a id="configure-git-policy"></a>
## Configure Git Policy

Run `haco config --edit` on the trusted management side. Add the following
**rule entries** to `policy.rules`, preserving `revision`, the existing rules,
saved choices and default. This is not a replacement configuration.
Use your registered URL, repository ID, Environment and existing branch.
The public example permits reads; it does not confer GitHub push rights.

```json
[
  {
    "capability": "git.repository", "action": "fetch",
    "environment": "sample-dev", "resource": "https://github.com/SLktEx/Hacocoon.git",
    "attributes": {
      "repository": "sample", "remote": "https://github.com/SLktEx/Hacocoon.git",
      "target_ref": "refs/heads/*", "old_oid": "*", "new_oid": "*", "operation_id": "*"
    },
    "decision": "allow", "reason": "Read all branches of this registered repository"
  },
  {
    "capability": "git.repository", "action": "push",
    "environment": "sample-dev", "resource": "https://github.com/SLktEx/Hacocoon.git",
    "attributes": {
      "repository": "sample", "remote": "https://github.com/SLktEx/Hacocoon.git",
      "target_ref": "refs/heads/main", "old_oid": "*", "new_oid": "*", "operation_id": "*",
      "update_kind": "fast-forward"
    },
    "decision": "require-approval", "reason": "Review the exact push"
  }
]
```

Every request attribute must be represented. The wildcards above allow changing
commit IDs and operation IDs while keeping repository authority fixed. Reads
cover all branches; pushes remain limited to the exact main ref and still ask.
An older read rule limited to main does not authorize all-branch fetch: review
that scope explicitly before updating it. Clone does not save an allow-push rule.
An exact-ref read denial also refuses the whole listing. Object fetch rechecks
each exact ref through the ordinary Policy service.
Push rules need `update_kind: fast-forward` for updates or `create` for a new
branch; older rules without it fail closed. To request review of a development
branch, add a push rule for its exact `target_ref` with `decision: require-approval`
and the appropriate update kind. Keep the main rule. With default deny, a missing
branch/kind rule is denied; default require-approval asks through the common review.
[Policy precedence](../design/policy-and-capability-foundation.md#matching-rule-precedence)
is deny, then require-approval, then allow, regardless of rule order.

Git's trusted upstream traffic needs no guest GitHub credential or authenticated
proxy. Package downloads and application DNS require their separate network rules.

## Inspect an interrupted push

On the trusted management side, use the Env name after a failed or interrupted push:

```bash
haco git status sample-dev
haco git reconcile sample-dev
```

`status` reads the latest requested push's durable record without contacting the
remote. `reconcile` makes a fresh, separately authorized exact-branch read. It
shows whether the current remote matches the proposed commit, the old commit,
has no such branch, or has diverged. Equality alone does not establish who changed
the remote: an unconfirmed original push stays unconfirmed. The original common
capability completion and the Git confirmation receipt remain separate JSON facts.
Neither command repeats a push or restores an interrupted approval.

Use `--json` for recorded identities/OIDs and `--request <request-id>` to inspect
an older request. Put options before the Env name. While a request is active,
wait for it to end. A missing dispatch record, changed Env generation/source
owner, missing current connection or corrupt audit refuses reconciliation;
inspect retained evidence rather than guessing another target. Older records
without dispatch ownership cannot be reconciled through this command.

Then fetch inside the Env, review your local branch against the remote, and submit
a new ordinary push if still needed. It receives a new Policy decision and exact
remote lease. Main push approval is preserved. This feature is implemented on the
development branch; fresh authenticated external/installed-client acceptance is
pending. See [the evidence contract](../adr/0070-git-push-reconciliation-evidence.md).

## Fetch, commit and push

In the Environment, use ordinary `git status`, `git fetch origin`,
`git pull --ff-only`, `git add`, `git commit` and `git push`.
The development candidate fetches up to 1024 SHA-1 branch heads per batch, with
an aggregate 32 MiB pack limit. Per-head transfers can repeat shared history;
large-pack optimization remains pending. Use `git branch -r` to see them, then for example
`git switch --track origin/feature/example` to work on an existing branch.
The development candidate supports one new branch or one existing fast-forward
target per push. For example, create local work with `git switch -c feature/work`,
commit it, then `git push -u origin feature/work`. Creation requests its own
approval and displays the exact new commit; subsequent updates are separate.
Force push, branch deletion, multi-ref push, LFS and submodules
remain deferred. A moved or deleted head is rejected; fetch again to inspect the
current upstream state. This does not establish large-repository acceptance.

New Workspaces use a wildcard origin fetch mapping. To opt an existing independent
Workspace into all branches, run the following **inside that Environment** after
reviewing the read Policy. This updates only its local Git configuration:

```bash
git config --replace-all remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'
git fetch origin
```

While a push waits, use a second trusted Host terminal:

```bash
haco git pending
haco git approve <id>
# Or reject that same request:
haco git deny <id>
```

Choose one decision. Review the repository, URL/ref, Environment and creation
identity, old/new commits and summary first. Approval applies to the fixed
proposal, not whatever a branch points to later. A denial leaves the remote
unchanged; another push creates another proposal.

After a timeout or transport/audit failure, inspect the upstream ref from the
authenticated trusted side before retrying. The external write may have completed.
A failed or missing receipt is not proof that nothing happened.

## Save a reviewed choice

```bash
haco git approve --save env <id>
haco git approve --save all <id>
haco git deny --save env <id>
haco git approve --save ask-env <id>
```

These are alternatives. `env` binds to this Environment creation;
`all` explicitly includes other and future Environments.
`deny --save all` saves global denial. `ask-env` and `ask-all` save future
require-approval while approve/deny separately answers the present request.
Omitting `--save` changes no Policy.

A saved choice retains the exact target branch and distinguishes creation from
fast-forward updates. Saving creation of `feature/work` does not allow its next
update or a push to main. Creating a branch never overwrites an existing remote
ref, even if it appeared while approval was pending. Inspect and fetch the remote
after a competing change; approval is not retried automatically.

Saved allow cannot override an administrator deny or require-approval rule.
The example deliberately asks on every push. If reusable allow is intended,
review the administrator restriction rather than weakening it implicitly.
Remove a saved entry through [haco config](../reference/configuration.md) to revoke it.
A saved-choice receipt requires durable storage and audit; execution failure
afterward may leave the saved Policy in place.

## Multiple repositories

Register each upstream/branch, then create one collection:

```bash
haco repo clone --branch first-branch first https://github.com/OWNER/REPO.git
haco repo clone --branch second-branch second https://github.com/OWNER/REPO.git
haco workspace create --repo first,second both
haco env create --workspace managed:both both-dev
haco git connect both-dev
```

Work in `/workspace/first` and `/workspace/second`; each has independent `.git`.
Add Policy for each registered source. One lease owns the entire collection;
members cannot be leased separately. Files outside the member mounts are
Environment-only. Membership editing and interrupted collection recovery are deferred.

## Retention and imported data

Stop retains work and leases. Environment deletion retains the Workspace;
explicit Workspace deletion destroys its Git data too.
[Data lifetime](data-lifetime.md) explains source/Workspace/Store cleanup.
A source repository cannot be deleted while any Workspace record uses its route.

Imported GitHub routes can reconnect only to an explicitly registered source with
the same ID, URL and branch. File-route and legacy imports remain offline;
a same-name source must not activate them.
See [imported Git reconnection](../design/git-and-github-capability.md#reconnect-an-imported-github-workspace).
Native imported fetch/push acceptance remains pending.

The authority details are in [Git design](../design/git-and-github-capability.md).
Commit-bound successes, failures and retained fixture evidence belong in
[acceptance evidence](../status/acceptance-evidence.md), not this procedure.

For path-based reopen and independent data forks, see [Workspace workflow](../design/workspace-workflow.md).
