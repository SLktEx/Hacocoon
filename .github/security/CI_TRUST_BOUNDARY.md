# GitHub Actions Trust Boundary

Hacocoon treats workflow configuration as part of the repository security boundary.

The current public-repository operating model is intentionally **solo-maintainer and contribution-closed**:

- `pull_request_creation_policy` is `collaborators_only`;
- the repository owner is the only trusted direct write actor;
- external users may read, fork, and open Issues, but cannot open Pull Requests against upstream;
- no self-hosted runner is attached to Hacocoon.

This contribution boundary is the primary reason anonymous fork code cannot currently enter Hacocoon's upstream PR CI at all. It must be re-audited before external Pull Requests or another write-capable collaborator are enabled.

Pull-request CI remains deliberately hardened as defense in depth and for owner/collaborator branches. It is constrained to disposable GitHub-hosted runners with read-only repository authority, no repository/environment secrets, and no persistent cache bridge. Experimental EC2 execution remains disabled. Privileged Incus and managed-storage checks live only in dedicated, explicitly reviewed workflows described below rather than in the normal `test.yml` job set.

The standalone Incus substrate acceptance in `tools/ci-incus.sh` and `test/e2e/incus_standalone.sh` runs only on a disposable GitHub-hosted Ubuntu 26.04 runner. It installs the container-only Incus 7.0 LTS packages from the pinned Zabbly signing key, requires the vendor AppArmor compatibility setting, initializes only the runner-local daemon, and uses a runner-local loopback TLS client instead of exposing the root-owned Incus Unix socket. The test creates only exact run-prefixed resources and verifies real Ubuntu 26.04 system containers across PID 1 systemd, exec, start/stop/restart/delete, managed-bridge DHCP and DNS, public IPv4 egress, peer traffic, hot NIC attach/detach, enabled systemd services, writable custom-volume restart persistence, and snapshot restore. Docker's host `FORWARD` policy is never globally weakened; when required, compatibility rules are scoped to the exact ephemeral Incus bridge and removed during cleanup. Failure diagnostics are captured before cleanup, and cleanup targets only the exact CI-owned prefix.

The Hacocoon Core lifecycle E2E in `tools/ci-incus-core.sh` is a second narrow exception and is explicitly gated with `needs: incus-standalone`. It runs on a fresh disposable Ubuntu 26.04 runner after the standalone substrate has passed, so substrate failures are separated from Hacocoon Core failures without carrying privileged state between jobs. The Core helper creates a runner-local TLS client so Hacocoon itself runs as the ordinary runner user rather than as root, grants root's subordinate-ID configuration only the single runner UID and GID needed to map the explicitly leased workspace into an unprivileged system container, and loads `br_netfilter` because the Hacocoon sandbox intentionally keeps bridge filtering enabled. The test enables `HACO_E2E_INCUS` only inside the reviewed helper, executes only `TestRealIncusWorkspaceLifecycleE2E`, and does not run the local CLI composition or managed Btrfs storage path. Cleanup is restricted to `haco-e2e-*` projects whose instances have the expected `haco-*` prefix plus the fixed Hacocoon sandbox profile/network/ACL. No repository secrets, write token, self-hosted runner, VM/KVM path, Seed/OCI plugin, or EC2 authority is exposed.

The linked-worktree / VS Code developer-flow acceptance extends that real-Incus path with `tools/ci-vscode-e2e.sh` and `test/e2e/vscode.sh`. The workflow YAML does not set `HACO_E2E_INCUS`; the reviewed helper opts in only after requiring GitHub Actions, a GitHub-hosted runner, and Linux. The test creates a local fixture repository plus standard linked worktrees, opens the selected worktree through `haco-vscode`, provisions the existing loopback-only SSH client path, and uses a real SSH client to prove `/workspace`, reconnect/reuse, cleanup, and private-key retention. The test intentionally requires ordinary Git operations to work from `/workspace` so linked-worktree metadata regressions are not hidden by file-only checks.

A separate optional acceptance slice uses the pinned GitHub-hosted `windows-2025` runner label to probe the real Windows/WSL boundary. It receives the same read-only PR authority and no secrets, never uses a self-hosted runner, and creates only an ephemeral test WSL distribution. The test attempts the actual supported primitives in order: current WSL, named Ubuntu 26.04 installation, WSL2, systemd PID 1, then the reviewed standalone Incus substrate. Because nested virtualization and WSL capabilities are properties of the hosted image, this Windows job is diagnostic/non-blocking; Linux real-Incus developer-flow coverage remains the mandatory gate. `windows-latest`, self-hosted labels, and arbitrary runner expressions remain rejected by workflow policy.

The managed Btrfs privilege-boundary acceptance in `.github/workflows/storage-helper-e2e.yml` is another narrow exception with two ordered jobs. `managed-btrfs-helper` first runs `tools/ci-storage-helper.sh` on a disposable Ubuntu 26.04 runner: the Go test process remains the ordinary runner user, the job builds `haco-storage-helper`, installs only that binary as root-owned mode `0755` under `/usr/local/libexec/hacocoon`, and uses the runner's existing `sudo` only to invoke the helper's typed storage operations. It does not install a passwordless sudo rule or expose a root control socket. This first stage proves real loop attach, Btrfs format, `compress=zstd:3` mount, inspection, unmount, loop detach, backing-image deletion, and idempotent reuse under an exact runner-temporary Hacocoon root.

`managed-btrfs-cli` is gated with `needs: managed-btrfs-helper` and runs on a fresh runner. It independently prepares the same reviewed real-Incus substrate and root-owned helper, then executes the actual `haco` binary as the ordinary runner user with default local composition. `tools/ci-storage-cli.sh` proves that normal `haco create` lazily creates and uses the real `haco-local-default` Btrfs-backed Incus pool, `haco exec` sees a writable leased Workspace, `haco delete` removes the named Environment, and subsequent `haco run` reuses the managed storage and cleans up its ephemeral Environment. The helper independently revalidates caller-owned root/image/mount paths, hardlink/symlink state, loop `BACK-FILE` plus `BACK-INO` identity, existing filesystem signatures before format, mount source/postconditions, resize targets, and fixed balance filters. Failure cleanup accepts only the exact `hacocoon` project, expected Hacocoon instance prefixes, the exact `haco-local-default` pool whose source matches the runner-temporary mount, and loop/mount paths under that same root; unexpected identities fail closed instead of being force-removed. No repository secrets, write token, self-hosted runner, arbitrary mount target, arbitrary block device, arbitrary command execution, or persistent root authority is provided.

`tools/check_workflow_policy.py` encodes defense-in-depth invariants for every file under `.github/workflows/`. It currently requires:

- no `pull_request_target` trigger;
- no `workflow_run` trust bridge;
- only explicitly approved GitHub-hosted runner labels (`ubuntu-26.04` and the pinned optional acceptance label `windows-2025`);
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

The workflow checker is deliberately small and conservative. Its implementation remains in `tools/check_workflow_policy_base.py`; the public `tools/check_workflow_policy.py` entry point extends only the explicit hosted-runner allowlist and delegates all other checks unchanged. If a new workflow pattern is required, update the checker and add positive/negative regression coverage in the same change instead of weakening the policy implicitly.

## Re-audit triggers

Do not treat the current result as permanent. Re-audit this boundary before any of the following:

- changing Pull Request creation from `collaborators_only` to an external-contribution policy;
- adding a non-owner collaborator with write/maintain/admin authority;
- adding a GitHub App or bot with broad repository write authority;
- registering a self-hosted runner or making an organization runner group visible;
- moving the repository into an organization;
- introducing a trusted workflow that executes artifacts or source originating from a less-trusted workflow.

Public visibility itself is not permission to mutate trusted history. The security boundary is determined by who can create trusted changes and where attacker-controlled code can execute.
