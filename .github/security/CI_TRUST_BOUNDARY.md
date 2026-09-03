# GitHub Actions Trust Boundary

Hacocoon treats workflow configuration as part of the repository security boundary.

The current public-repository operating model is intentionally **solo-maintainer and contribution-closed**:

- `pull_request_creation_policy` is `collaborators_only`;
- the repository owner is the only trusted direct write actor;
- external users may read, fork, and open Issues, but cannot open Pull Requests against upstream;
- no self-hosted runner is attached to Hacocoon.

This contribution boundary is the primary reason anonymous fork code cannot currently enter Hacocoon's upstream PR CI at all. It must be re-audited before external Pull Requests or another write-capable collaborator are enabled.

Pull-request CI remains deliberately hardened as defense in depth and for owner/collaborator branches. It is constrained to disposable GitHub-hosted runners with read-only repository authority, no repository/environment secrets, and no persistent cache bridge. Experimental EC2 execution remains disabled. Privileged Incus checks live only in dedicated, explicitly reviewed workflows described below rather than in the normal `test.yml` job set.

The standalone Incus substrate acceptance in `tools/ci-incus.sh` and `test/e2e/incus_standalone.sh` runs only on a disposable GitHub-hosted Ubuntu 26.04 runner. It installs the container-only Incus 7.0 LTS packages from the pinned Zabbly signing key, requires the vendor AppArmor compatibility setting, initializes only the runner-local daemon, and uses a runner-local loopback TLS client instead of exposing the root-owned Incus Unix socket. The test creates only exact run-prefixed resources and verifies real Ubuntu 26.04 system containers across PID 1 systemd, exec, start/stop/restart/delete, managed-bridge DHCP and DNS, public IPv4 egress, peer traffic, hot NIC attach/detach, enabled systemd services, writable custom-volume restart persistence, and snapshot restore. Docker's host `FORWARD` policy is never globally weakened; when required, compatibility rules are scoped to the exact ephemeral Incus bridge and removed during cleanup. Failure diagnostics are captured before cleanup, and cleanup targets only the exact CI-owned prefix.

The Hacocoon Core lifecycle E2E in `tools/ci-incus-core.sh` is a second narrow exception and is explicitly gated with `needs: incus-standalone`. It runs on a fresh disposable Ubuntu 26.04 runner after the standalone substrate has passed, so substrate failures are separated from Hacocoon Core failures without carrying privileged state between jobs. The Core helper creates a runner-local TLS client so Hacocoon itself runs as the ordinary runner user rather than as root, grants root's subordinate-ID configuration only the single runner UID and GID needed to map the explicitly leased workspace into an unprivileged system container, and loads `br_netfilter` because the Hacocoon sandbox intentionally keeps bridge filtering enabled. The test enables `HACO_E2E_INCUS` only inside the reviewed helper, executes only `TestRealIncusWorkspaceLifecycleE2E`, and does not run the local CLI composition. Cleanup is restricted to `haco-e2e-*` projects whose instances have the expected `haco-*` prefix plus the fixed Hacocoon sandbox profile/network/ACL. No repository secrets, write token, self-hosted runner, VM/KVM path, Seed/OCI plugin, or EC2 authority is exposed.

The Incus-owned Btrfs acceptance in `.github/workflows/incus-storage-e2e.yml` is another narrow exception. It runs on a disposable Ubuntu 26.04 runner, prepares the reviewed real-Incus substrate, and executes the actual `haco` binary as the ordinary runner user. `tools/ci-incus-storage-cli.sh` verifies that normal `haco create` lazily creates and uses the real `haco-local-default` Incus-owned loop-backed Btrfs pool, `haco exec` sees a writable leased Workspace, `haco delete` removes the named Environment, and subsequent `haco run` reuses the pool and cleans up its ephemeral Environment. Storage backing-image, loop, filesystem, and mount lifecycle remain owned by Incus. Failure cleanup accepts only the exact `hacocoon` project and expected Hacocoon resources; unexpected identities fail closed instead of being force-removed. No repository secrets, write token, self-hosted runner, arbitrary command execution, or persistent root authority is provided to Hacocoon.

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

The direct Incus environment-variable ban is intentionally retained: dedicated helpers may opt into a narrowly reviewed real-Incus test, but workflow YAML itself must not turn arbitrary jobs into privileged Hacocoon integration environments.

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
