# GitHub Actions Trust Boundary

Hacocoon treats workflow configuration as part of the repository security boundary.

The current public-repository operating model is intentionally **solo-maintainer and contribution-closed**:

- `pull_request_creation_policy` is `collaborators_only`;
- the repository owner is the only trusted direct write actor;
- external users may read, fork, and open Issues, but cannot open Pull Requests against upstream;
- no self-hosted runner is attached to Hacocoon.

This contribution boundary is the primary reason anonymous fork code cannot currently enter Hacocoon's upstream PR CI at all. It must be re-audited before external Pull Requests or another write-capable collaborator are enabled.

Pull-request CI remains deliberately hardened as defense in depth and for owner/collaborator branches. It is constrained to disposable GitHub-hosted runners with read-only repository authority, no repository/environment secrets, and no persistent cache bridge. Experimental EC2 execution remains disabled in normal PR CI. Real Incus system-container acceptance is permitted only through the reviewed `tools/ci-incus.sh` path described below.

The guarded Incus path verifies that it is running in GitHub Actions on a disposable GitHub-hosted Ubuntu 26.04 Linux runner. It installs the container-only Incus 7.0 LTS packages from Zabbly's package repository and verifies the repository signing key against the published fingerprint `4EFC 5906 96CB 15B8 7C73 A3AD 82CC 8797 C838 DCFD` before trusting it. The helper requires Incus 7.0.1 or newer within the 7.0 LTS series and requires the vendor AppArmor compatibility sysctl to be active; it does not weaken an instance profile with an unconfined AppArmor setting.

The helper initializes only the disposable runner-local daemon. The test process receives a loopback-only TLS client configuration held under the runner temporary directory, while the root local-socket client is isolated under `/root`; the root-owned Incus Unix socket is not exposed to the unprivileged test process. If the preinstalled Docker firewall leaves IPv4 forwarding at policy `DROP`, the helper adds only CI-owned bridge-specific `DOCKER-USER` rules for outbound traffic and related/established return traffic, and removes those exact rules during cleanup rather than globally changing the host forwarding policy.

Acceptance is deliberately phased. `incus-standalone` first proves the Incus substrate without running Hacocoon: real Ubuntu 26.04 system containers, systemd, managed networking/DNS, public egress, hot NIC lifecycle, storage persistence, snapshots, and explicit instance lifecycle. Only after that job succeeds does `haco-core-e2e` run on a fresh disposable runner, recreate the same guarded substrate, and enable `HACO_E2E_INCUS=1` inside `tools/ci-incus.sh` for the reviewed Hacocoon Core lifecycle tests. Diagnostics are captured on failure and cleanup targets only explicitly owned resources.

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
- no direct `HACO_E2E_INCUS=1` or experimental EC2 enablement in workflow YAML.

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
