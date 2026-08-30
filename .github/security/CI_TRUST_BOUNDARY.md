# GitHub Actions Trust Boundary

Hacocoon treats workflow configuration as part of the repository security boundary.

The current public-repository operating model is intentionally **solo-maintainer and contribution-closed**:

- `pull_request_creation_policy` is `collaborators_only`;
- the repository owner is the only trusted direct write actor;
- external users may read, fork, and open Issues, but cannot open Pull Requests against upstream;
- no self-hosted runner is attached to Hacocoon.

This contribution boundary is the primary reason anonymous fork code cannot currently enter Hacocoon's upstream PR CI at all. It must be re-audited before external Pull Requests or another write-capable collaborator are enabled.

Pull-request CI remains deliberately hardened as defense in depth and for owner/collaborator branches. It is constrained to disposable GitHub-hosted runners with read-only repository authority, no repository/environment secrets, and no persistent cache bridge. Real Incus **system-container** acceptance may run on those disposable GitHub-hosted Linux runners through the guarded `tools/ci-incus.sh` path. Experimental EC2 execution remains disabled in normal PR CI.

The Incus CI path does not attach a self-hosted runner, expose Host credentials to an Environment, or hand the test process the root-owned Incus Unix socket. The helper verifies that it is running in GitHub Actions on a `github-hosted` Linux runner, installs the Ubuntu-packaged Incus daemon, initializes only the disposable runner, and gives the unprivileged test process a loopback-only TLS client for that daemon. `HACO_E2E_INCUS=1` is enabled inside that guarded helper only after those checks. The workflow always attempts exact-name cleanup and captures daemon diagnostics on failure; the runner itself is discarded after the job.

`tools/check_workflow_policy.py` encodes defense-in-depth invariants for every file under `.github/workflows/`. It currently requires:

- no `pull_request_target` trigger;
- no `workflow_run` trust bridge;
- only explicitly approved GitHub-hosted runner labels;
- immutable full-SHA pins for external Actions;
- `actions/checkout` with `persist-credentials: false` in PR workflows;
- read-only/none permissions in PR workflows, including no OIDC write authority;
- no repository/environment secret injection into PR workflows;
- no `actions/cache` use in untrusted PR workflows;
- no cross-run/external artifact downloads from PR workflows;
- `actions/setup-go` caching disabled in PR workflows;
- no direct `HACO_E2E_INCUS=1` enablement in workflow YAML; real Incus acceptance must use the guarded repository helper;
- experimental EC2 disabled in normal PR CI.

The direct Incus environment-variable ban is intentionally retained: adding another privileged-looking workflow stanza must not silently become a second acceptance path. The reviewed helper is the single place that establishes the disposable-runner preconditions and enables real Incus tests.

`tools/check_public_release_readiness.py` additionally validates the live repository assumptions used by the current solo-maintainer model, including contribution-closed PR policy, absence of non-owner direct collaborators, protected `main`, protected release tags, and zero repository self-hosted runners.

The workflow checker is deliberately small and conservative. If a new workflow pattern is required, update the checker and add positive/negative regression coverage in the same change instead of weakening the policy implicitly.

## Re-audit triggers

Do not treat the current result as permanent. Re-audit this boundary before any of the following:

- changing Pull Request creation from `collaborators_only` to an external-contribution policy;
- adding a non-owner collaborator with write/maintain/admin authority;
- adding a GitHub App or bot with broad repository write authority;
- registering a self-hosted runner or making an organization runner group visible;
- moving the repository into an organization;
- introducing a trusted workflow that executes artifacts or source originating from a less-trusted workflow.

Public visibility itself is not permission to mutate trusted history. The security boundary is determined by who can create trusted changes and where attacker-controlled code can execute.
