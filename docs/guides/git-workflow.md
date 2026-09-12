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
      "target_ref": "refs/heads/main", "old_oid": "*", "new_oid": "*", "operation_id": "*"
    },
    "decision": "allow", "reason": "Read the registered branch"
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
commit IDs and operation IDs while keeping repository/ref authority fixed.
Push rules need `update_kind: fast-forward`; older rules without it fail closed.
[Policy precedence](../design/policy-and-capability-foundation.md#matching-rule-precedence)
is deny, then require-approval, then allow, regardless of rule order.

Git's trusted upstream traffic needs no guest GitHub credential or authenticated
proxy. Package downloads and application DNS require their separate network rules.

## Fetch, commit and push

In the Environment, use ordinary `git status`, `git fetch origin`,
`git pull --ff-only`, `git add`, `git commit` and `git push`.
The managed helper handles one existing SHA-1 branch and packs up to 32 MiB.
Force push, branch creation/deletion, multiple refs, LFS and submodules are deferred.

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
