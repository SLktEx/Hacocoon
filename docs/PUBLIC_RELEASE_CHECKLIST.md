# Public repository launch checklist

[**日本語**](PUBLIC_RELEASE_CHECKLIST.ja.md) | English

This is the fail-closed launch procedure for converting Hacocoon from a private repository to a public repository. It is part of the release trust boundary, not a convenience checklist.

Hacocoon must **not publish an official public release** until every mandatory item below is complete and `tools/check_public_release_readiness.py` reports `PUBLIC RELEASE READINESS OK` against the live public repository.

## Why Actions are disabled during conversion

GitHub Free does not expose the required repository rulesets and required Environment reviewers while this repository is private. That creates a configuration-order problem: those controls can only be finalized after public conversion.

To avoid an interval in which an external fork can run workflows before the public trust boundary is configured:

1. finish and merge all private-repository hardening;
2. confirm normal CI and secret scanning are green;
3. **disable GitHub Actions for this repository**;
4. convert the repository to public;
5. configure and verify every server-side control below while Actions remain disabled;
6. run the readiness checker;
7. re-enable Actions only after the checker passes.

Do not approve or merge external contributions, create official releases, or re-enable Actions during an incomplete conversion.

## 1. Pre-public private-repository gate

Before changing visibility:

- [ ] `main` CI is green.
- [ ] `secret-scan / gitleaks` is green against complete reachable Git history.
- [ ] There are no known unresolved Critical/High public-release code blockers.
- [ ] `.github/CODEOWNERS` exists and covers workflows, release configuration, installers, policy checkers, and this checklist.
- [ ] `.github/workflows/release.yml` uses the dedicated `release` Environment for the privileged `publish` job.
- [ ] Official release tags are accepted only when they resolve to current trusted `main` HEAD.
- [ ] Linux and Windows public installers fail closed on provenance verification.
- [ ] GitHub Actions is disabled immediately before visibility is changed to public.

## 2. Convert to public with Actions still disabled

Change repository visibility to public. Keep Actions disabled until all remaining sections pass.

Immediately confirm that no workflow run started as a side effect of public conversion.

## 3. Configure `protect-main` repository ruleset

Create one active repository ruleset named exactly:

```text
protect-main
```

Target the default branch (`main` / `~DEFAULT_BRANCH`). Required settings:

- [ ] enforcement: `active`;
- [ ] target: branch;
- [ ] no bypass actors;
- [ ] block branch deletion;
- [ ] block non-fast-forward updates / force pushes;
- [ ] require pull requests before merging;
- [ ] require at least 1 approving review;
- [ ] dismiss stale approvals when new commits are pushed;
- [ ] require approval of the most recent reviewable push;
- [ ] require all review threads to be resolved;
- [ ] require status checks against the latest base branch;
- [ ] require all current Hacocoon checks listed below.

Required status contexts:

```text
docs
workflow-policy
release-config
test (1.26.x)
test (1.27.x)
race
e2e
gitleaks
```

`require_code_owner_review` should be enabled once there is a second trusted reviewer capable of approving owner-authored changes. Until then, CODEOWNERS still records trust-root ownership, but enabling mandatory CODEOWNER review with only one maintainer can deadlock the repository.

A public launch that intends to accept external PRs still needs at least one independent trusted reviewer to satisfy the mandatory approval rule without bypassing it.

## 4. Configure `protect-release-tags` repository ruleset

Create one active repository ruleset named exactly:

```text
protect-release-tags
```

Target:

```text
refs/tags/v*
```

Required settings:

- [ ] enforcement: `active`;
- [ ] target: tag;
- [ ] restrict tag creation;
- [ ] block tag deletion;
- [ ] block non-fast-forward tag updates / movement;
- [ ] define exactly one explicitly reviewed bypass actor that is allowed to create official release tags;
- [ ] do not grant release-tag bypass to ordinary write collaborators.

The readiness checker accepts a single bypass actor of type `RepositoryRole`, `Integration`, or `Team`, and prints it for human review. For a personal repository, the narrowest practical choice is the repository administrator/owner role. Re-evaluate this if the repository moves to an organization or a dedicated release integration is introduced.

## 5. Configure the `release` GitHub Environment

Configure the Environment named exactly:

```text
release
```

Required protection:

- [ ] at least one required reviewer;
- [ ] prevent self-review enabled;
- [ ] reviewer set is narrower than ordinary repository write access;
- [ ] no secrets are needed for the normal release workflow unless separately audited.

The workflow YAML reference alone is not sufficient. The server-side required-reviewer rule is the human authorization boundary that separates `repository_dispatch` authority from official publication authority.

## 6. Require approval for every external contributor workflow

In fork pull-request workflow settings, configure:

```text
approval_policy = all_external_contributors
```

Do not use a first-time-contributor-only policy. A previously accepted innocuous contribution must not permanently authorize later attacker-controlled workflow execution.

## 7. Prove public fork PRs cannot reach self-hosted runners

For the public repository:

- [ ] repository self-hosted runner count is exactly zero;
- [ ] no persistent runner carrying SSH, AWS, GitHub, Incus, Docker/containerd, internal-network, or other privileged authority is selectable by fork PRs;
- [ ] if the repository is ever organization-owned, no organization runner group is visible to Hacocoon unless a separately audited design explicitly replaces this zero-access rule.

After Actions is eventually re-enabled, validate with a harmless external fork PR that attempts both:

```yaml
runs-on: self-hosted
```

and any known custom self-hosted label. Neither job may start on a persistent/private runner.

The repository-side `tools/check_workflow_policy.py` guard remains defense in depth. It cannot replace server-side runner isolation because attacker-controlled PR jobs can be scheduled in parallel with policy jobs.

## 8. Enable Immutable Releases

Before the first official public release, enable GitHub Immutable Releases so published release assets and their release tags cannot be replaced or deleted through supported release mutation paths.

See `RELEASE_SECURITY.md` for provenance and release-binding details.

## 9. Run the live fail-closed checker

With authenticated GitHub CLI access that can read rulesets, Environment protection, Actions fork policy, and self-hosted runner inventory:

```bash
python3 tools/check_public_release_readiness.py --repo SLktEx/Hacocoon
```

Expected result:

```text
PUBLIC RELEASE READINESS OK
```

Any API permission failure is `UNVERIFIED`, not success. Any missing or weaker setting is failure.

The checker expects the exact names `protect-main`, `protect-release-tags`, and `release` so that a similarly named but unrelated configuration cannot satisfy the gate accidentally.

## 10. Re-enable Actions and run adversarial validation

Only after the live checker passes:

1. re-enable GitHub Actions;
2. verify the required CI jobs run on GitHub-hosted `ubuntu-24.04` runners;
3. open a harmless external fork PR and verify workflow approval is required;
4. attempt harmless `self-hosted` and custom-runner-label jobs and verify no persistent/private runner starts them;
5. rerun `tools/check_public_release_readiness.py`;
6. rerun the public-repository adversarial security audit against the exact public configuration;
7. close public-launch blocker issues only after the live evidence is recorded.

## Publication decision

A public repository can exist temporarily with Actions disabled while the server-side controls are being configured. An **official public release is forbidden** until this checklist and the live readiness checker pass.
