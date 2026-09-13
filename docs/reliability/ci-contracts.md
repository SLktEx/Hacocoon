# PR verification contracts

[日本語](ci-contracts.ja.md) | English

PRs targeting main run all four maintained contract workflows. There is no
path/type filter: shared packages, modules, dependencies, packaging, clients and
new directories cannot accidentally omit installation acceptance. This deliberately
spends more runner time until a dependency-aware selector has equivalent negative
coverage. Do not replace this with an enumerated list of known source directories.

The executable job inventory is [ci_contracts.json](../../tools/ci_contracts.json).
[The contract checker](../../tools/check_ci_contracts.py) rejects missing triggers,
filtered PRs, missing/conditional/tolerated jobs or contract steps, and incomplete evidence dependencies.
It runs in the existing workflow-policy job and local CI. Security policy remains
owned by [the CI trust boundary](../../.github/security/CI_TRUST_BOUNDARY.md).

## Required checks and their scope

| Change / contract | PR workflow and required job | Actual execution boundary |
|---|---|---|
| Core, CLI, Policy, approvals, Git broker, shared packages | `test`: test matrix, race, e2e | Go unit/component/integration; shipped process black boxes with isolated fixtures. Local Git transport refusal does not claim authenticated GitHub push. |
| Package/release configuration | `test`: release-config, build | Both Linux architectures, GoReleaser packaging and installer archives before merge. |
| Incus lifecycle, network, egress | `incus-core-e2e`: incus-standalone, incus-core-e2e | Independent fresh runners; native substrate, provider contract and shipped controller/CLI lifecycle. Standalone's scoped Docker coexistence rules are not applied to Hacocoon Environment egress. |
| Incus-owned Btrfs, Base, snapshots, transfer, retained OCI | `incus-core-e2e`: incus-owned-btrfs | Native adapter tests plus CLI/controller scenarios; the fixture creates retained data and tests lazy pool creation. Adapter tests are not complete packaged acceptance. |
| Native Ubuntu installer and installed network isolation | `ubuntu-installer-e2e`: ubuntu-user-path | Unchanged packaged installer, ordinary user, installed product CLI run/create/status/stop/start/delete with retained Workspace, the migration CLI journey, and kernel network assertions. |
| Windows/WSL installer, restart, reinstall | `windows-installer-e2e`: windows-user-path | Packaged BAT, ConPTY ordinary WSL/Host entry, terminate before reinstall, retained Host data. Phase timing is recorded; the initial driver alone does not establish Environment data retention. |
| SSH, IDE, Windows network, transfer, reclamation, notification | `windows-installer-e2e`: windows-user-path | Follow-on installed egress and native Windows/OpenSSH/VS Code acceptance; require their actual steps, not just the initial BAT result. |

Configure branch protection to require `test-evidence`, `incus-core-e2e-evidence`,
`ubuntu-installer-e2e-evidence` and `windows-installer-e2e-evidence`, in addition to
the existing checks. Repository code cannot configure GitHub branch protection.
The evidence job uses `!cancelled()` and checks every required job and named contract step result explicitly;
skipped, cancelled, missing or failed jobs cannot satisfy it. Named matrix variants must also have successful receipts, so an aggregate needs result cannot hide a removed architecture or Go series. Artifact/history
retrieval failures are also failures. Native prerequisites are never substituted
with a successful focused probe.

Failed or skipped dependencies still reach evidence validation. Whole-workflow
cancellation stops the evidence job, so a superseded run cannot retain the
concurrency slot while waiting for an evidence runner. Keep the underlying
required checks: a cancelled workflow does not establish successful execution.
Do not use job-level `always()` here; [GitHub cancellation](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-cancellation)
can leave such a job running after cancellation.

## Failure and rerun evidence

Steps identify their boundary as `[product]`, `[fixture]` or `[infrastructure]`.
These identify the failed operation's owner, not a speculative root cause. A
product step can reveal an infrastructure incident; investigate its diagnostics
before assigning a cause. Unannotated historical steps remain `unclassified`.

[ci_history.py](../../tools/ci_history.py) reads Actions metadata with a read-only
token, including every attempt and paginated job collection. Each workflow retains
`ci-evidence.json` for 30 days with workflow, source SHA, checked SHA, run, attempt,
job, failed step, boundary and same-SHA red-to-green observations. It also examines
other runs of that workflow/event/source SHA, including reopened PR runs. Successful
partial reruns never erase the original failed job. Native Go commands also retain `ci-test-results.jsonl` with exact expected test names, PASS/FAIL/SKIP or missing results, command exit status and run identity.
Each attempt's workflow conclusion is also retained. A startup failure with zero
jobs cannot disappear when a later attempt starts successfully; attempt identity
must match the recorded run and source SHA.

A failed attempt continues to fail the evidence check for that source SHA. There
is no retry-to-green switch or automatic waiver. Investigate, fix the cause or
record a reviewed infrastructure resolution in a new commit. Changed runner
images/input must be considered when comparing runs; a source SHA alone is not
proof of equivalent environments. Cancellation is not called a flake, but a
cancelled required job does not pass the current gate.

For a recent-run report, with a read-only `GITHUB_TOKEN` and
`GITHUB_REPOSITORY=SLktEx/Hacocoon` in the environment:

```bash
python3 tools/ci_history.py --recent 100 --output /tmp/ci-evidence.json
```

This reads at most 100 recent runs, then all attempts/jobs for those runs. It does
not claim repository-wide historical completeness. Retain decisive findings and
issue links in [acceptance evidence](../status/acceptance-evidence.md); download
reports before Actions retention expires. Reports never copy credentials,
arbitrary subprocess logs, configuration dumps or remote artifact contents.

## Synchronization and isolation

Each Incus job bootstraps on its own disposable runner. The two bootstrap helpers
share one LTS source and the daemon's bounded `admin waitready` API. No job inherits
another runner's substrate. This implements the scheduling shape requested by
#607 without pooling state; #463 still owns Windows performance targets.

Required provider tests have explicit names and must emit exactly one PASS.
An empty selection, skipped prerequisite or failed command is not a pass. Normal
repository tests can still skip opt-in native cases; that is distinct from the
dedicated required native job.

The PTY regression synchronizes on a foreground command marker before resizing;
command output alone does not establish that readline has finished restoring
terminal state. Guest state/address/DNS polls use explicit predicates. Poll delays
are sampling intervals; deadlines remain upper failure bounds. Windows Host entry
errors terminate the driver without retrying the user action. Installer-owned WSL restarts observe successful stop-state listings instead of waiting a fixed 750 milliseconds. Project cleanup
requires successful inventory and positive project absence; a failed query cannot
authorize deletion or report successful cleanup.

## External inputs

| Input | PR constraint / remaining variance |
|---|---|
| Actions | Full commit SHA pins; read-only PR authority and no secret-bearing workflow trust bridge. |
| Runner | Explicit Ubuntu 26.04 / Windows Server 2025 OS families. GitHub still updates image builds, kernel and bundled packages; the Actions setup log owns the exact image version. This is not an immutable VM image guarantee. |
| Go | Exact 1.26.7 and 1.27.0 matrix; `GOTOOLCHAIN=local`, setup-go check-latest disabled. Unit shuffle seed 615 is reproducible; race remains enabled. |
| GoReleaser / OCI fixture / VS Code / ConPTY | Existing exact tool versions, OCI downloaded archive SHA256 checks, portable VS Code verification and pywinpty 3.0.2. |
| Incus CI substrate | One signed Zabbly 7.0 LTS source, fingerprint verification and bounded server version checks. LTS patch packages remain mutable. |
| Packaged installer apt | Unchanged shipped Ubuntu installer/repositories. Package mirrors and apt versions remain external inputs; preinstalling dependencies would hide installation defects. |
| Ubuntu container / WSL images | Current product Base acquisition and shipped WSL metadata/hash verification; trusted cache is an optimization with the verified download fallback. Aliases/metadata remain mutable and must be recorded when diagnosing a failure. |
| Internet / authenticated services | Some installed connectivity/Base tests still require upstream availability. Authenticated GitHub push requires a dedicated credential and remains separate from untrusted PR execution. |

## Acceptance layers and remaining limits

Main retains repository/native validation; manual `real-git-push-e2e` and the
authenticated private-registry scenario add credential/provider compatibility.
`windows-wsl-image-cache` prepares a verified download cache; it is not acceptance.
Release publication/attestation and externally published artifact installation
remain release concerns under #370. Broader compatibility/stress can extend these
layers without removing a directly changed PR contract.

The credentialed GitHub push path still has no equivalent credential-free shipped
PR success path. Hosted image builds, apt packages and product image aliases are
not immutable. Native failures require cause-specific fixes and new execution
evidence, not relabelling as passes or moving out of PR. Consequently static
policy and repository tests alone cannot establish Issue #615's full native
repeatability criteria. Track exact candidate/run results in acceptance evidence.
