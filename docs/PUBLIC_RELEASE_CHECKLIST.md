# Public repository security checklist

[**日本語**](PUBLIC_RELEASE_CHECKLIST.ja.md) | English

Hacocoon is public, but the current repository policy is intentionally **solo-maintainer and contribution-closed**.

The security goal is not “every public project must accept external PRs.” The current goal is:

```text
anyone can read / fork / open issues
        |
        X  no external PR into upstream
        |
only repository owner has trusted write authority
        |
protected main + required CI
        |
trusted release workflow
```

`tools/check_public_release_readiness.py` validates this exact operating model. If the maintainer model changes, update the threat model before weakening or replacing these checks.

## 1. Repository contribution boundary

Required current settings:

- [ ] repository is public;
- [ ] `pull_request_creation_policy = collaborators_only`;
- [ ] there are no non-owner direct collaborators;
- [ ] external users cannot create pull requests against the upstream repository;
- [ ] Issues may remain public for bug reports and discussion.

This is intentionally stricter than the earlier public-launch design that assumed external fork PRs would be accepted.

Before enabling external PR creation, perform a new security review covering fork workflow approval, token permissions, secrets, runner selection, workflow-file changes, and review ownership.

## 2. `Protect main` ruleset

The active branch ruleset is named exactly:

```text
Protect main
```

It must target the default branch and enforce:

- [ ] active enforcement;
- [ ] no bypass actors;
- [ ] deletion blocked;
- [ ] non-fast-forward / force push blocked;
- [ ] changes reach `main` through a Pull Request;
- [ ] stale review state is dismissed after new pushes;
- [ ] `require_last_push_approval = false` while the repository is solo-maintainer;
- [ ] review threads must be resolved;
- [ ] required status checks run against the latest base branch.

Required status contexts:

```text
docs
workflow-policy
release-config
test (1.26.x)
test (1.27.x)
race
e2e
```

`gitleaks` is strongly recommended as a required status check as well. The dedicated secret-scan workflow must remain enabled regardless.

### Why required approvals may be zero

Hacocoon currently has one trusted maintainer. Requiring an independent approving review would make the repository impossible to maintain without adding a second trusted person or a bypass.

Therefore `required_approving_review_count = 0` is an intentional solo-maintainer exception, not a general recommendation. For the same reason, `require_last_push_approval` must remain `false`: GitHub requires that approval to come from someone other than the last pusher, so enabling it deadlocks self-authored maintenance in a one-person repository.

This exception is valid only while:

- external PR creation remains disabled; and
- there are no non-owner direct collaborators.

If either condition changes, required human review and latest-push approval semantics must be redesigned before the new actor is trusted.

## 3. `Protect release tags` ruleset

The active tag ruleset is named exactly:

```text
Protect release tags
```

and targets:

```text
refs/tags/v*
```

Required current protections:

- [ ] active enforcement;
- [ ] no bypass actors;
- [ ] tag deletion blocked;
- [ ] tag update/movement blocked;
- [ ] non-fast-forward movement blocked.

Tag creation is not separately restricted in the current solo-maintainer model because no non-owner collaborator has repository write authority.

This does **not** make a tag sufficient release authority. The release workflow independently requires the requested release tag to resolve to the current trusted `main` HEAD and revalidates tag/main identity immediately before publication.

If another write-capable collaborator or release bot is added, tag-creation authority must be redesigned before that actor is trusted.

## 4. `release` GitHub Environment

The privileged publish job continues to use the Environment named:

```text
release
```

For the current single-maintainer repository, a required reviewer is not mandatory: there is no independent second human who can provide a distinct trust decision.

The Environment is still useful as a named privilege boundary and a future attachment point for stronger protection.

If a second trusted maintainer is added, configure an independent required reviewer and prevent self-review where supported.

## 5. Self-hosted runners

Current policy:

- [ ] repository self-hosted runner count is exactly zero;
- [ ] normal CI uses approved GitHub-hosted runners only;
- [ ] no persistent host carrying SSH, AWS, GitHub, Incus, Docker/containerd, or internal-network authority is selectable by repository workflows;
- [ ] if the repository becomes organization-owned, visible organization runner groups require a fresh audit.

Do not add a self-hosted runner merely to run privileged Incus/cloud E2E. If privileged E2E is needed later, put it behind a separately trusted execution boundary.

## 6. Release workflow trust

Official releases must preserve all of these properties:

- [ ] `repository_dispatch` executes the trusted workflow from the default branch;
- [ ] requested release tag must resolve to current `main` HEAD;
- [ ] build/test job has read-only repository authority;
- [ ] publish job is separate and minimal;
- [ ] publish revalidates current main/tag identity;
- [ ] release payload receives GitHub/Sigstore attestations;
- [ ] Actions used in trusted workflows remain pinned to immutable commit SHAs.

See [Release security](RELEASE_SECURITY.md).

## 7. Live readiness check

Run with authenticated GitHub CLI access capable of reading rulesets, collaborators, Environment state, and runner inventory:

```bash
python3 tools/check_public_release_readiness.py --repo SLktEx/Hacocoon
```

Expected result:

```text
PUBLIC RELEASE READINESS OK
```

Warnings document accepted solo-maintainer tradeoffs or recommended defense in depth. API permission failures remain `UNVERIFIED`, never success.

## 8. Re-audit triggers

The current policy must be revisited before any of the following:

- external pull requests are enabled;
- a non-owner direct collaborator is added;
- a second trusted maintainer gains write authority;
- a bot/app gains broad write or release authority;
- a self-hosted runner or runner group becomes visible;
- the repository moves to an organization;
- the release trigger or publisher authority model changes.

Public visibility itself is not the trust boundary. **Who can mutate trusted history and where untrusted code can execute** is the trust boundary.
