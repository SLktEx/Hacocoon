# GitHub Actions Trust Boundary

Hacocoon treats workflow configuration as part of the repository security boundary.

The current public-repository operating model is intentionally **solo-maintainer and contribution-closed**:

- `pull_request_creation_policy` is `collaborators_only`;
- the repository owner is the only trusted direct write actor;
- external users may read, fork, and open Issues, but cannot open Pull Requests against upstream;
- no self-hosted runner is attached to Hacocoon.

This contribution boundary is the primary reason anonymous fork code cannot currently enter Hacocoon's upstream PR CI at all. It must be re-audited before external Pull Requests or another write-capable collaborator are enabled.

Pull-request CI remains deliberately hardened as defense in depth and for owner/collaborator branches. It is constrained to disposable GitHub-hosted runners with read-only repository authority, no repository/environment secrets, and no persistent cache bridge. Experimental EC2 execution and Hacocoon's real privileged integration suites remain disabled in normal PR CI.

One narrower exception exists for the standalone Incus substrate smoke test in `tools/ci-incus-smoke.sh`. That test runs only on a disposable GitHub-hosted Ubuntu 26.04 runner, installs the native container-only Incus packages, initializes the runner-local daemon, launches one fixed-name system container, verifies systemd and `incus exec`, and deletes that exact test instance. It does not run Hacocoon binaries, expose repository credentials or secrets, attach a self-hosted runner, exercise EC2, or enable `HACO_E2E_INCUS`. The helper verifies its GitHub-hosted-runner preconditions before privileged setup, diagnostics are captured on failure, cleanup is attempted with an exact instance name on every run, and the runner is discarded after the job.

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
- no direct `HACO_E2E_INCUS=1` or experimental EC2 enablement in normal PR CI.

The direct Incus environment-variable ban is intentionally retained: the standalone substrate smoke is not permission to turn normal PR CI into a Hacocoon privileged integration environment. Real Hacocoon/Incus acceptance remains a separately reviewed path.

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
