# ADR 0066: Explicit Git branch creation authority

Status: accepted for the development candidate; native acceptance pending.

Date: 2026-09-13

## Context

M2 and Issue #586 require ordinary development-branch push after all-branch
reads. The registered checkout branch is provenance, not write authority.
[ADR 0064](0064-git-read-scope-and-push-authority.md) keeps reads independent of
push; its existing-branch-only implementation limit is extended here.

## Decision

The Standard helper accepts one literal `refs/heads/…` target and a fixed commit.
The broker obtains a fresh fetch decision before preparation and a separate push
decision after object validation. Existing source identity, Environment creation,
Workspace ownership, cancellation, single-use approval and audit checks remain
in the common services. Core gains no Git-specific rules.

An absent target uses the all-zero SHA-1 old OID, which is never accepted as an
advertised existing head. The trusted agent rechecks absence, imports only the
bounded object pack, verifies the new object is a commit and prepares a bounded
whole-tree summary. External execution uses Git's explicit empty expectation
`--force-with-lease=<ref>:`. Existing-target updates instead require a
fast-forward from the exact observed old OID and a lease on that OID.

The broker sends the reviewed new OID rather than a mutable local branch name.
Git must return one porcelain receipt for that exact ref and commit with the
expected creation/update status. Git can exit successfully with an up-to-date
result when another actor created the identical branch during approval; this
does not establish that the expected-absent creation happened and is refused.
No automatic retry follows an ambiguous external result; inspect the upstream.

Reusable choices preserve the exact ref and `update_kind` (`create` or
`fast-forward`). Saving creation never grants later updates, another branch or
main. A main require-approval rule retains precedence. Fetch/clone creates no
allow-push rule. Guest configuration, paths, credentials and remote URLs cannot
replace the registered trusted source. No history rewrite, deletion or multi-ref
push is accepted. The 32 MiB pack limit remains.

## Rejected alternatives

Deriving push authority from clone/fetch, saving all-branch write access from one
review, treating an absent ref as a wildcard lease, pushing a moving local branch,
and trusting process exit status alone can broaden or misreport the approved
operation. Giving Env credentials or management access is unnecessary.

Real Git component tests cover denied creation, fixed commits, saved scope
separation, concurrent different/identical creation and unsupported mutations.
They do not establish installed Environment, authenticated GitHub, GUI approval
or giant-repository acceptance. General ambiguous-push recovery remains #470.
