# Implementation Status

## Second-stage workflow

Status: **partial**. B1–B3 and the selected B5/B6 improvements are implemented
and locally accepted. B4 image distribution is implemented with repository
regressions, but real Docker/nerdctl distribution and independent container
execution are **not executed**. The requested second-stage acceptance is not
complete. See the [workflow](reference/managed-repository-workflow.md),
[OCI contract](design/oci-image-distribution.md) and
[follow-ups](status/development-follow-ups.md).

| Step | Implemented behavior and observed result |
|---|---|
| B1 | Explicit trusted-Host setup discovers existing DrvFs drives. PowerShell received a separate spaced argument, preserved stdout/stderr and exit 23; user-owned `/mnt/c` reads/writes passed. Rechecked on final installed `029ff08`. Only C is mounted; extra-drive parsing has regression coverage, additional-drive real-host acceptance is not executed. Environments have no Windows devices, `/init` or interop environment. |
| B2 | Packaged `087e7e2`: one `b-dev` Environment mounts independent Btrfs copies at `/workspace/b-first` and `/workspace/b-second`, with separate `.git`, no alternates/commondir. SSH fetch/pull, edit, commit and fixed-content approval push passed for each repository. GitHub returned `be34f60c2c3d1ab5761e821fbdaada5e4d5802dc` and `b834ee67dbc8f5e37e73656f13872d42ceda40f3` on the respective test branches. Two different remote URLs also pass local real-Git regression. |
| B3 | Packaged `3747bae`: `haco base list` and `env switch-base` moved the same managed Workspace from Ubuntu 26.04 to 24.04. All 54 repository files, including `.git`, had identical hashes after switching. Unpushed commits `bce47b9` / `6d5fc53`, modified tracked files and untracked notes survived. Reconnected Git/SSH, pinned the new host key, and edited over SSH on Ubuntu 24.04.4. |
| B4 | `haco plugin oci distribute --runtime <runtime> --image <image> <environment>` selects `docker` or `nerdctl`, stages a bounded private archive and loads into a separate guest runtime. Unit/component checks cover both command families, failed source export, invalid input, archive bounds and fixed instance-local sockets. Actual runtime installation/nesting and container execution are awaiting explicit authorization; neither Docker nor nerdctl has real-host acceptance. |
| B5 | Packaged `029ff08`: `haco env ssh-config b-dev` generated OpenSSH configuration and standard `ssh -F ... haco-b-dev` connected successfully, removing manual host/port/user transcription. Existing Incus 6.0.5's unsupported `config show --format` was replaced with its JSON query API and regression coverage. |
| B6 | Packaged `029ff08`: `haco env status` clearly displayed Environment, state, Workspace, access and Base. Stopped status explicitly said the Workspace is retained. `--json` preserves machine-readable output. |

**Test target:** all external writes used only
`https://github.com/SLktEx/Hacocoon-test.git`, branches
`codex/stage-b-b-first-20260906` and `codex/stage-b-b-second-20260906`.
The two real-host registrations use this same authorized URL with separate
branches; different remote URLs were exercised only in repository regression.

Btrfs independently reported source UUIDs
`411102dc-d913-264a-96a0-b09d079eb898` / `58ccd7df-d8df-3444-98b4-67b35d85018e`
as the parent UUIDs of the respective Workspace volumes
`49eff338-40d8-244b-9276-e35952b475b2` / `a23fadbc-77af-be4a-b7a9-f9829e96e613`.
This verifies the observed COW relationship, not performance.

**Final installed candidate:** `029ff08e34c98e075b7b0b3d3a7fc7f639e89323`,
checkpoint `v0.28`, snapshot `0.27.0-SNAPSHOT-029ff08`, built
`2026-09-06T10:25:40Z`. ZIP SHA-256:
`20f308cb5bcccfdaef1f0c76914bdae65834c957afd6c446fd0effdda26717fe`.
Each candidate was built from its branch commit and applied through its ordinary
Windows BAT to the existing Hacocoon WSL distribution. No installer-specific
product override or internal state repair was used. Fresh installation was not
repeated. The retained A configuration is Windows 26200.9278 / WSL 2.7.12 /
Incus 6.0.5 / Incus-owned Btrfs `haco-local-default`.

**Bases:** B3 moved from revision
`sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c`
to Ubuntu 24.04 revision
`sha256:f38ca805517f5b6e301f33b0f44523386c5a050847564c1233e586106b31dbc9`.
Later explicit Ubuntu 26.04 creation resolved
`sha256:297ce79fb308c09126222dd6e64c260003c5d1e1ea1ce46ea43e80a419941636`;
the earlier Environment's recorded revision was unchanged.

**A regression on final candidate:** ordinary single-repository `b-a-work` /
`b-a-dev` creation with explicit Base, generated SSH configuration, fetch and
fast-forward pull from `f4ff6e3` to `be34f60`, Python compilation/assertion, commit,
denial (remote unchanged), then approval push all passed. GitHub independently
returned `145fd7fce49a5a8771e39e7b142d47aa49c910c3` on the first test branch.
Disconnect and graceful stop succeeded; all 28 files including dirty/untracked
work and Git state had identical hashes before/after stop. The canonical lease
and Incus volume remain. Inner/outer clients matched the controller build;
all six doctor checks passed. The original A Workspace/Environment remains
preserved. `b-a-dev` is stopped; `b-dev` remains available for pending B4 checks.

**Repository validation:** maintained `ci-local.sh docs`, `workflow-policy`,
`test` (Go tests/vet and JS), `race` and `e2e` passed after B5/B6. Narrow lifecycle,
Git, collection-mount, OCI and SSH-configuration regressions passed while
iterating. GoReleaser check/build and installer archive checksums passed.
The complete release-config/forwarding jobs, hosted CI and broad real-host
runtime/network matrices are not claimed. Local validation used Go 1.27.1.

**Manual operations and blocking condition:** B1 requires the recorded Physical
Host script; repeat after interop socket changes. Authentication and narrow
Policy remain trusted-side setup. SSH private key and host-key pinning remain
client-owned; use the controller's Physical Host/Windows loopback, since a
different WSL distribution may have its own loopback namespace. SSH package
downloads still need credential-free proxy exports (existing #469). Base switch
discards root filesystem/packages and requires Git/SSH reconnection. B4's
automatic execution review rejected nesting/runtime installation as requiring
explicit approval for the two named instances. It also rejected an attempted
missing-runtime check because distribution could transfer an unverified image.
Those commands did not execute. The approval request is pending; B4 is current
required work, not a completed or deferred acceptance item.

## Managed repository WSL workflow — 2026-09-06

**Implemented; A1–A6 accepted on the local Windows/WSL configuration below.** The v0.27 candidate adds
product CLI repository registration, independent Incus Btrfs Workspace copies,
controller-backed Environment creation/SSH, a Git-only remote helper and
graceful stop retaining Workspace ownership. Authenticated Git executes inside
trusted `haco-host`; Policy, approval, state and Incus authority stay in the
Physical Host controller. Ordinary helper fetch, conflict-free pull, deny and
approval pinned to old/new OIDs pass a real-Git local integration regression,
including a local branch change during approval. Local repository tests and
the separately observed packaged journey are distinguished below. See the
[workflow](reference/managed-repository-workflow.md) and
[ownership decision](adr/0008-managed-repository-workspaces.md).

**Packaged acceptance:** `7a4d1227c95642f27cb118c3d20d2cd554e8be32`, version
`0.27.0-SNAPSHOT-7a4d122`, built `2026-09-06T07:57:54Z`. Windows ZIP SHA-256:
`0468c8f95c5b431c5d4160aead860deb152ed8d8e381b321c6b85b2f650d1a80`.
Windows build `26200.9278`, WSL `2.7.12.0`, kernel `6.18.33.2-2`, installed
Ubuntu 26.04, Incus `6.0.5`, Incus-owned Btrfs pool `haco-local-default`.
The ordinary packaged BAT applied to the existing Hacocoon distribution and
exited 0 after all six doctor checks. No CI product overrides or internal
resource repairs were used. Fresh absent-distro installation of this candidate
was **not** repeated; prior installer acceptance below remains separate.

| Step | Observed result |
|---|---|
| Entry and controller | Ordinary `wsl -d Hacocoon` entered trusted `haco-host`; inner and outer product clients returned the same build and `poc-dev` Environment |
| Repository and COW | Registered `poc` from `https://github.com/SLktEx/Hacocoon-test.git`, branch `codex/wsl-poc-20260906`; independent `poc-work2` copy mounted at `/workspace` in `poc-dev` |
| Default Base | `haco/ubuntu-26.04`, revision `sha256:d071290fb40659981198baf0161a8bcc9910ebae79a15f5ef5d9c06dbdb2ea4c` |
| Development | Standard OpenSSH with a client-owned key and pinned server key; Python byte-compilation, two unittest cases and a commit succeeded. Own `.git`, no `commondir`/alternates, no Host gh credential file or management socket; trusted source worktree remained unchanged |
| Fetch/pull | Ordinary helper fetch and `pull --ff-only` advanced the same Workspace from `f4ff6e3` to the remotely prepared `19caa79e123b981227d1c0b58783c7a6af80e930` |
| Denial | `haco git deny` rejected the pending push; remote stayed at `19caa79` |
| Approval | Proposal fixed `19caa79` → `c18cbb8e202cecc0d6c80b29a8cd700dc1c0558f`, the registered URL/ref and operation. `haco git approve` let ordinary `git push` finish 0; GitHub independently returned exactly `c18cbb8`. Audit recorded denial, approval and successful completion |
| Finish | SSH commands exited, `env disconnect poc-dev ssh-2222` revoked the endpoint, `env stop poc-dev` succeeded and inner/outer status reported `stopped`. Reconnection was refused |
| Retention | The exact canonical Workspace lease and custom volume remained. Unpushed HEAD `5650953d591fc6294a0db8db5f71a408e7917555`, modified `greeting.py`, untracked notes and the branch-ref file had identical SHA-256 before/after stop; remote remained `c18cbb8` |

Btrfs source UUID `760d7b7e-0e0e-7f4c-9f88-7303ad96f55c` matches the parent UUID
of Workspace `f3326bfe-8cb0-684e-a3a3-d437dd3b817e`. This establishes the observed
Incus COW relationship, not a performance measurement. The first candidate
`c116307` registered the source but failed Environment write access because it
dropped Incus ID-map bookkeeping while copying. A focused provider regression
and `7a4d122` fix preserve that bookkeeping. The failed `poc-work` copy was kept;
the accepted journey used a fresh ordinary `workspace create` from the same
registered source, with no chown or privileged-Environment workaround.

**Manual setup retained:** install `git gh` and authenticate gh inside trusted
`haco-host`; create exact Git/Ubuntu egress Policy rules on the Physical Host;
provide the SSH public key and pin its host key; export the credential-free
Standard proxy URL inside the SSH shell before apt. Existing authorized gh
credentials were transferred by stdin to the trusted Host only. Pre-application
Host HTTP/HTTPS attempts timed out; the normal BAT's setup then restored passing
readiness without separate repair. The initial failure's cause is unconfirmed.

**Repository validation:** maintained `ci-local.sh docs`, `workflow-policy`,
`test` (all Go tests, vet, JS syntax/tests), `race` and `e2e` passed. The E2E
expectation that product `env` was unavailable was updated, and the local E2E
runner used existing Go caches outside its temporary HOME. The Incus ID-map fix
passed focused race regressions; GoReleaser check/build and package checksums
passed. The complete release-config/forwarding jobs and a new hosted CI run are
not claimed by those local checks.

**Deferred / unverified:** SSH proxy setup is [#469](https://github.com/SLktEx/Hacocoon/issues/469);
interrupted approvals, unknown push results and retry are [#470](https://github.com/SLktEx/Hacocoon/issues/470).
The next requested B1 is trusted `haco-host` Windows exe execution and available
WSL drive mounts, under the existing [#275](https://github.com/SLktEx/Hacocoon/issues/275)
boundary; no automatic Environment exposure. B2/B3 multiple repositories/Base
switching, larger packs, other auth methods, force/multiple refs, LFS/submodules,
generic recovery/cleanup, resume UX and a wider host matrix remain outside this
acceptance. The retained test Environment is stopped; both Workspace copies and
the test branch are intentionally preserved. B/C implementation is not part of
this checkpoint's acceptance work.

The historical M1 observations below remain tied to their original builds.

## WSL delivery update — 2026-09-06

This candidate branch implements the following WSL slice; the requested WSL M0–M1 scope is **implemented and accepted**. Refreshed main `e8974ef` includes #441/#442/#453/#456 and #458/#459. Its two later release-bootstrap/revert commits have no net file changes; merge `b58f82c` has the same product tree as the accepted `c749ff9`. A merged PR or an older green run is not acceptance of later product changes.

- **Implemented storage:** Incus-owned Btrfs is the only distribution/runtime path. External `driver`/`source` attachments and uncertain inspections fail closed. Desired policy is `compress=zstd:3,noatime,nodiscard`. The [read-only mount diagnostic](design/btrfs-storage-layout.md#read-only-mount-diagnostics) separates configuration, verified live application and `pending` application. Backing device/inode, one full-image loop association and the Btrfs root mount must agree. Unknown, malformed or changing observations cannot pass. Hacocoon adds no separate image/loop/mount lifecycle or diagnostic repair.
- **Implemented installer and trusted Host:** the default account is non-root `hacocoon` with a locked password; `-InteractiveUserSetup` is optional. Current reruns preserve account identity/password state and write no sudo policy. Controller-owned `haco setup` provisions the owned trusted host and narrow endpoint; the common installer now requires all product doctor checks before completion. Fresh hosts have explicit devices without inherited profiles. The owned `haco-host0` bridge provides trusted infrastructure DNS/DHCP/NAT, and Docker forwarding is restricted to that bridge and established replies. See [bootstrap](WINDOWS_WSL_BOOTSTRAP.md), [trusted Host](design/trusted-host.md) and ADRs [0004](adr/0004-wsl-installer-authority.md), [0005](adr/0005-trusted-host-network-ownership.md), [0006](adr/0006-controller-owned-host-setup.md).
- **Partial product CLI:** new `haco` provides help/version, `setup`, `doctor` and the controller-backed WSL login alias without invoking `hacoq`. The Physical Host owns controller state, Policy, providers and Incus authority. There is no guest controller or Incus daemon. [Diagnostics](design/controller-client-transport.md#host-diagnostics) return six ordered checks with bounded failure/pending actions. The controller wait and read-only guest DNS/route startup wait do not retry failed external checks or repair resources. The broader lifecycle/Base/SSH CLI migration remains separate; the #456 controller adapter is reusable.
- **Implemented Standard proxy lifecycle:** the installed controller owns the fixed proxy listener and verifies shared guards before binding. Control/proxy shutdown is coupled, including hijacked CONNECT tunnels. The daemon has no ambient approval provider; exact allows stay audited and require-approval fails closed. Same-PID listener and unmanaged-source refusal are distinct from allowed Environment traffic. See [ADR 0007](adr/0007-controller-owned-standard-egress.md).
- **Repository validation — `c749ff9`:** focused race/vet tests, pending CLI/API regressions, nine Windows assertions, five installer shell tests, shell syntax and documentation checks passed. The maintained `ci-local.sh test` passed the full shuffled Go suite, vet, two JavaScript syntax checks and five notification tests. An earlier local vet attempt included downloaded research sources under `bin/`; after renaming those observations to `.txt`, the complete entry point passed. These tools supplied no product environment overrides or installed-resource repair.
- **Planned Seed retirement:** Seed code remains with [Base/optional OCI dependencies](design/oci-seed-and-cow.md). Base selection and optional Plugins remain supported architectural boundaries.
- **Implemented registration continuation, Windows package accepted:** failed WSL inventory cannot become assumed absence, and native creation success requires exact registration readback. A failed creation/readback saves an advisory stage/options record for a manual current-BAT rerun; it grants no authority and is never executed. Explicit exit 3010 propagates as restart-required, while exit 0 without registration remains incomplete with a conditional reboot action. PowerShell 5.1 component tests and actual BAT exit-propagation tests passed. Windows feature installation and OS reboot are not accepted by those tests. See [bootstrap continuation](WINDOWS_WSL_BOOTSTRAP.md#interrupted-registration-and-windows-restart).

Packaged acceptance is bound to **`c749ff9033b33c3526e108f60ce2009638075152`**:

| Environment | Observed acceptance |
|---|---|
| [Windows gate](https://github.com/SLktEx/Hacocoon/actions/runs/34008408570) | Exact cached BAT fresh creation, ordinary entry, stop/reentry, same-version rerun, cold doctor, build identity, retention, proxy listener ownership and unmanaged-source 403 passed |
| [Ubuntu installer](https://github.com/SLktEx/Hacocoon/actions/runs/34008411207) | Packaged ordinary-user installation and trusted-host checks passed |
| [Incus gates](https://github.com/SLktEx/Hacocoon/actions/runs/34008410296) | Standalone, owned Btrfs, authenticated private registry and Core jobs all passed |
| Current Windows host | Unchanged ZIP application and same-version BAT rerun exited 0 after all six readiness checks; normal entry, both clients' full build identities, retained UUID/file/account/sudo policy, Btrfs state and proxy checks passed. Doctor after confirmed distro stop exited 0 in 51.906 seconds |

The local ZIP is `0.26.1-SNAPSHOT-c749ff9`, built at `2026-09-06T03:12:38Z`, with SHA-256 `f638379fb293cf249f32ef46b5576b95906ff775bc2f00f96ae3ed602724d3f9`. Fresh Windows acceptance means an absent Hacocoon distribution on the runner's current-WSL substrate, not disabled Windows features or a Windows OS reboot. Local retained data is the trusted-host sentinel and baseline, not proof of uncommitted/untracked/unpushed Workspace retention.

**Unresolved startup failure:** an ordinary entry on `42e2fb3` saw Incus main PID 282 receive SIGKILL at 11:33:30 JST, leaving the standard 600-second start-post waiter and controller dependency blocked. The signal source is unconfirmed; available kernel records did not establish OOM. Incus's standard automatic restart began at 11:43:16 without manual service/mount repair, and later entry/retention passed. The guest DNS/DHCP startup race has a separate fix and acceptance. Neither that fix nor the later `c749ff9` successes establish the SIGKILL source or the cause of the earlier independent WSL exit-9 observation.

**Registration package acceptance — `4df465a71aedcdc70c28b543220b79b2465808ab`:** [Windows run 34010791925](https://github.com/SLktEx/Hacocoon/actions/runs/34010791925), job `101426135649`, passed the exact fresh cached BAT, ordinary entry, restart, same-version rerun, retained data, six doctor checks and both PowerShell/BAT regressions. Local PS5.1 native inventory/argument tests, package/provenance checks, `ci-local.sh docs` / `workflow-policy`, and native documentation consistency passed. The provenance test's first Ubuntu 22.04 attempt correctly failed the 26.04 baseline; it passed on the supported substrate without changing product requirements. Local ZIP SHA-256 is `439dfc8a0a4dab5ef4adf05f1b1ed9b3e02883a5009b66dca7513c528d0d3105`, version `0.26.1-SNAPSHOT-4df465a`, build `2026-09-06T04:10:02Z`. It was built and checksummed, but not installed locally again: the current local installation remains accepted `c749ff9`, while CI supplies the changed fresh-registration/rerun coverage.
**Current M1 scope:** the latest user direction excludes actual Windows OS reboot implementation/acceptance and further continuation work. Keep verification proportional to a concrete change or failure, and put additional maintained regressions in CI. Existing successful checks need no repeat without new evidence. The required installed Environment allowed-proxy/denied-direct acceptance through the existing controller/provider boundary has now passed. The unexplained startup incident remains documented; the later scoped signal observation did not identify its cause. Broader diagnostic features, firewall-order matrices, CLI/SSH development and Workspace retention remain follow-up work, not expanded completion conditions.

**Environment source correction:** the Windows gate on `f373cfc` passed the exact BAT journey but its allowed HTTPS probe returned proxy 403. The persisted source resolver compared provider-local refs with the routed refs produced by Environment creation. It now matches both provider and native ref using the canonical router decoder. A focused regression reproduces the failure with the actual Base router's creation result and refuses another provider's identical native ref.

**M1 acceptance — `81c0d160722b96864daa8d6f5f3b9ea86423ff48`:** [Windows run 34013409969](https://github.com/SLktEx/Hacocoon/actions/runs/34013409969), job `101432997324`, passed fresh cached BAT installation, ordinary entry, stop/restart, same-version rerun, retained trusted-host data and six doctor checks. The installed-controller Environment check also passed certificate-verified allowed HTTPS, unapproved-hostname 403, direct TCP refusal, management-socket absence and controller cleanup. CI checked PR merge commit `9049df39f8000e32103b6a2f3939ea3d14fc5ffe`; its entire tree was verified identical to candidate `81c0d16`. Local egress, Environment router, composition and Standard proxy tests passed after the routed-reference regression failed on the previous code. Documentation consistency passed.

The new local ZIP is `0.26.1-SNAPSHOT-81c0d16`, built at `2026-09-06T05:12:08Z`, SHA-256 `4938622b994a66b71d5647086819db63e7ee7a7a8ea1189e3b2ad964ccb69c6b`. GoReleaser packaging and all distribution checksums passed. This ZIP has not been reinstalled on the current Windows host, whose installed version remains `c749ff9`; candidate Windows acceptance above is from CI. Actual Windows OS reboot remains outside scope.

**Historical next item:** M2 Environment creation is implemented in the v0.27 candidate above; packaged journey acceptance is tracked separately.

The historical checkpoint tables below retain their original milestone context.

Status date: 2026-08-31, after cloud deferral, the Base/OCI CLI split, Docker compatibility lifecycle integration, the OCI Seed Builder repository slices including credential-free managed-Environment harvest, the client-neutral interaction-event contract, the reusable client-adapter contract, domain-aware Standard egress enforcement, Incus-owned Btrfs rootfs pool integration, default Incus-managed Btrfs `zstd:3` compression, browser/native/VS Code notification clients, phased real-Incus CI acceptance on Ubuntu 26.04, the shared structured logging foundation, ordinary-user real-Incus storage acceptance, and the persistent trusted `haco-host` / default WSL entry slice.

This file reports **current code reality**, not desired architecture. Hacocoon is pre-1.0; implementation does not imply API stability, production support, or real-host acceptance beyond explicitly named acceptance checks.

The current milestone position is **v0.28**. Milestones are lightweight development checkpoints: v0.17 still has acceptance work, but that partial status does not block later implemented checkpoints such as v0.18-v0.26.

| Area | Current repository reality | Milestone |
|---|---|---:|
| Secure Workspace Runtime / Workspace leases | Incus-backed Environment lifecycle, canonical Workspace identity, RO/RW leases and recovery are implemented | v0.1-v0.2 |
| Client access | status, loopback forwarding and SSH preparation/revocation are implemented | v0.3 |
| Policy / Capability | fail-closed policy, approval and audit are implemented | v0.4 |
| Git / GitHub push | privileged push is brokered on the trusted Host without exporting reusable Host credentials | v0.5 |
| Agent / orchestrator integration | `haco run`, machine output and external events are implemented; orchestration remains outside Core | v0.6 |
| Client-neutral interaction events | public `pkg/interaction` projects capability audit records into minimized stable event types with deterministic IDs, resumable cursors, bounded batches, recovery/attention flags, and public corruption errors; observation never authorizes or executes a capability | v0.6 / cross-cutting |
| Environment routing | the provider-neutral routing seam remains implemented; **cloud implementation is currently deferred** and concrete EC2/AWS/EBS code is absent from the active tree | v0.7 |
| Reusable client adapter contract | public `pkg/clientadapter` exposes client-owned DTOs for exact Environment ensure/reuse, status, loopback SSH/TCP connections, revoke/delete, `/workspace` discovery, and `pkg/interaction` batches; ordinary `haco ssh` is the non-VS-Code proof path | v0.8 / cross-cutting |
| VS Code / Agent Host | `haco-vscode`, per-agent binding and `haco-agent-host` foundations are implemented | v0.8-v0.10 |
| Base lifecycle | provider-neutral Base identity and `haco base list` / `haco base inspect` / `create --base` are implemented | v0.11 |
| Resource budgets | CPU, memory, PID and root-storage budgets are modeled and Incus finite limits are enforced or rejected | v0.12 |
| Managed sandbox network | managed `haco-sandbox0`, proxy-only ACL transport guard and `haco-sandbox` profile are created/verified; bridge DNS is disabled while DHCP remains; drift fails closed | v0.13 / cross-cutting |
| Git fetch plugin | `haco plugin git fetch <environment>` uses trusted Host Git/GitHub authority including `gh auth git-credential` for HTTPS private repositories | v0.14 |
| OCI usage telemetry / Seed recommendation | `haco plugin oci seed sample` records image identities and `recommend` ranks immutable identities; deterministic top 10% eligible recommendations become `auto_promote=true` | v0.15 implemented |
| OCI image deletion | `haco plugin oci image delete <reference[@digest]>` records deletion tombstones; exact immutable identities can be explicitly re-enabled without silently undoing broader deletion state | v0.16 implemented |
| OCI Seed Builder / Btrfs COW | `haco plugin oci seed build` / `current`, per-Base pinning, conservative GC/recovery, trusted Host acquisition, credential-free exact-image harvest from explicitly marked running managed Environments, offline no-NIC build, immutable publication/current pointer, exact-parent resolution, and pre-build interrupted-builder recovery are implemented; real-host/authenticated-registry/COW acceptance remains pending | v0.17 partial |
| Docker compatibility | `haco plugin oci docker status/prepare` validates a Base-provided genuine Docker profile, verifies pinned systemd units, refuses active vendor-daemon takeover, and enables Environment-local socket activation without making Docker a Core requirement | v0.18 implemented |
| Domain-aware egress authorization | Core `network.egress/connect` authority, Standard HTTP/HTTPS proxy, Host-side DNS pinning, private-address rejection, CONNECT/SNI validation, trusted Incus source-IP mapping and `haco egress serve` are implemented; real supported-Incus acceptance remains host-dependent | v0.19 implemented |
| Managed Btrfs rootfs storage | local composition lazily asks Incus to create the `haco-local-default` loop-backed Btrfs pool with `size=128GiB` and routes Hacocoon-owned Base/Tooling/Seed/Environment/trusted-host rootfs paths to that pool | v0.20 implemented |
| Managed Btrfs transparent compression | default Incus pool creation requests `compress=zstd:3`; `compress-force` and `autodefrag` are not desired defaults, while filesystem mount lifecycle remains Incus-owned | v0.21 implemented |
| Interaction notification clients | `haco-notify` provides loopback interaction delivery for browser and native OS notification flows, and the optional VS Code notification extension consumes the same client-neutral interaction stream; replay/dedup behavior is covered by tests | v0.22 implemented |
| Real Incus E2E acceptance | GitHub-hosted Ubuntu 26.04 first verifies standalone real Incus system-container behavior and then runs Hacocoon Core lifecycle E2E on a fresh runner; the phased gate covers systemd/exec, networking, hotplug, storage/snapshot behavior, diagnostics, and guarded cleanup | v0.23 implemented |
| Structured logging | shared `log/slog` foundation, INFO-default text/JSON output, Environment lifecycle operation fields, sanitized DEBUG Host-command tracing, egress authorization tracing, and defense-in-depth secret redaction are implemented across maintained executables | v0.24 implemented |
| Incus-owned Btrfs storage acceptance | the actual ordinary-user `haco` binary is exercised against real Incus; acceptance verifies lazy pool creation, sparse Incus backing image, loop attachment, Btrfs mount, zstd policy, writable Workspace flow, pool reuse, and guarded cleanup | v0.25 implemented |
| Trusted `haco-host` / default WSL entry | local Incus runtime can ensure and shell into a persistent trusted logical `haco-host`; exact ownership markers and reserved-name collision refusal protect the boundary, managed storage is used, WSL interactive entry targets `haco-host` by default while Physical Host root remains an explicit recovery path, and raw Incus control is not exposed into the trusted Host | v0.26 implemented |
| OCI plugin boundary | containerd/nerdctl/Docker-dependent behavior lives under optional `modules/plugin/oci`; `HACO_PLUGIN_OCI=nerdctl|docker` opts in, and Core remains valid when unset | cross-cutting |
| Optional Local OCI Registry | Registry/proxy is optional and not required for ordinary direct upstream pulls or Seed construction | unversioned optional / deferred |

## Domain-aware egress boundary

Ordinary HTTP/HTTPS egress is enforced through the Standard proxy rather than by DNS-to-IP ACL approximation. The Incus NIC remains default deny and allows only TCP to the managed bridge gateway on the Standard proxy port. The bridge keeps DHCP but disables its dnsmasq DNS listener with `raw.dnsmasq=port=0`; unmanaged DNS or ACL configuration fails closed.

The managed profile provides HTTP(S) proxy discovery to Hacocoon Environments. The proxy derives Environment identity from trusted Incus source-IP state, routes each hostname/port/protocol request through the existing Policy / Approval / Capability / audit path, resolves DNS only on the Host after authorization, pins the public answer set per connection, and validates HTTPS CONNECT against ClientHello SNI before forwarding TLS bytes. `haco egress serve` is the foreground trusted-Host launch path so the current stdio approval provider remains usable. See [`EGRESS_AUTHORIZATION.md`](EGRESS_AUTHORIZATION.md).

## Notification clients

v0.22 turns the client-neutral interaction stream into user-visible notification adapters without moving approval authority into the client. `haco-notify` exposes the loopback bridge used by the browser and native notification paths, while `clients/vscode-notify` provides the optional VS Code consumer. Cursor persistence, replay, deduplication, corruption handling, browser behavior, and VS Code behavior are covered by repository tests. Reading or displaying a notification remains observational only and never approves or executes a Capability. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Real Incus E2E acceptance

v0.23 records a support-confidence checkpoint rather than a new Core API. GitHub Actions on Ubuntu 26.04 proves the Incus substrate independently before exercising Hacocoon Core. The phased standalone stage verifies real system containers, systemd/exec behavior, networking, device hotplug, storage/snapshot behavior, diagnostics and exact cleanup; the dependent Core stage then runs the Hacocoon lifecycle against real Incus on a fresh runner. This distinguishes an Incus substrate failure from a Hacocoon regression and prevents fake-only E2E from being treated as sufficient acceptance.

## Structured logging

v0.24 makes structured logging a named milestone. Maintained executables configure one shared `log/slog` root from `HACO_LOG_LEVEL` and `HACO_LOG_FORMAT`; INFO/text is the default and JSON is available without changing stdout command results. Environment create/exec/shell/delete operations carry stable `operation`, `environment_id`, duration, result/error fields through context. The trusted Host runner adds sanitized DEBUG command metadata and classifies Incus/network/storage/Git/OCI commands without automatically logging subprocess stdout or stderr. Network egress authorization adds normalized target/protocol and request correlation at DEBUG.

The shared handler redacts known password/token/API-key, authorization/cookie, credential-bearing URL, and secret-assignment patterns as defense in depth, including at DEBUG. Call sites still must omit arbitrary headers, environments, configuration objects, private keys, request bodies, and untrusted output. ERROR ownership remains at the operation/reporting boundary; lower Host/provider layers use DEBUG diagnostics rather than duplicating ERROR. See [`reference/logging.md`](reference/logging.md).

## Trusted `haco-host` / WSL entry

v0.26 introduces a persistent trusted logical Host on the local Incus path. `haco host ensure` creates or reconciles `haco-host`, and `haco host shell` enters it after ensure. Hacocoon marks exact ownership, rejects name collisions with non-owned instances, uses the managed storage path, and keeps the raw Incus control socket outside `haco-host`. The WSL login shim makes normal interactive distro entry target `haco-host`; explicit Physical Host root entry remains the recovery escape hatch.

Real Incus acceptance covers trusted-host creation, ownership, idempotent ensure, stopped-state recovery, managed-storage behavior, and control-socket non-exposure. Real Windows/WSL interactive-login acceptance is still host-dependent. The current slice establishes lifecycle/default-entry behavior; broader Git/OCI/credential/control-channel migration remains follow-up work. See [`design/trusted-host.md`](design/trusted-host.md) and [`WINDOWS_WSL_BOOTSTRAP.md`](WINDOWS_WSL_BOOTSTRAP.md).

## Client adapter boundary

`pkg/clientadapter` is the reusable adapter-facing contract for VS Code-independent clients. Exported signatures use package-owned DTOs and public error sentinels rather than `internal/core` types. The adapter can ensure/reuse an Environment only when the canonical Host Workspace and requested access mode match exactly, exposes the in-guest Workspace as `/workspace`, reconciles connection metadata, and composes the public `pkg/interaction` event contract.

SSH preparation accepts public-key material only. Clients retain their private keys and IDE configuration. Returned/reconciled SSH and TCP connections are revalidated as loopback-only; incompatible provider output is rejected and newly-created invalid connections are revoked or surfaced as recovery-required when cleanup cannot be proven. The existing `haco create` + `haco ssh` + ordinary `ssh` flow is the non-VS-Code proof. See [`CLIENT_ADAPTER_CONTRACT.md`](CLIENT_ADAPTER_CONTRACT.md).

## Client interaction boundary

`pkg/interaction` is the reusable client-facing event contract. It reads the existing trusted capability audit stream and exposes only stable, presentation-safe fields: schema/event/request identity, UTC time, event kind, Environment/capability/action labels, attention/recovery flags, a closed failure code, and the next resume cursor.

Raw capability resources, authority attributes, opaque parameters, provider output, approval tokens, credentials, and free-form audit reasons are not part of the client schema. Browser, VS Code, code-server, JetBrains, and future adapters may independently observe/deduplicate these events; reading an event has no side effect and never substitutes for the trusted Policy/Capability approval or execution boundary. See [`INTERACTION_EVENTS.md`](INTERACTION_EVENTS.md).

## Core/plugin boundary

With `HACO_PLUGIN_OCI` unset, Hacocoon Core must not require or probe for containerd, nerdctl, Docker CLI, Docker Engine, or a local OCI Registry. Base identity remains a Core/provider-neutral concept under `haco base ...`; OCI workload tooling lives under `haco plugin oci ...`.

The project-maintained OCI plugin profile may use containerd + nerdctl, and the Docker driver may provide genuine Docker compatibility. Neither choice defines a mandatory Hacocoon Core runtime.

## OCI Seed / storage

v0.17 has repository slices for build/publish, operations hardening, and credential-free managed-Environment harvest. The implemented path is trusted Host acquisition/cache -> offline no-NIC Seed Builder -> immutable Seed revision/current pointer -> exact-parent resolution -> normal Incus/storage-driver clone. One writable `/var/lib/containerd` must never be shared across Environments.

Explicit per-Base pins are persisted as immutable OCI identities. Deletion tombstones override recommendations and existing pins until the exact immutable identity is explicitly re-enabled. `haco plugin oci seed recover` reconciles exact Hacocoon temporary builders and then performs conservative GC; `haco plugin oci seed build` invokes recovery before a new build while holding the Seed build lock. GC does not manipulate Incus-owned Btrfs internals and retains current, in-use, instance-base, or externally aliased images. Deletion state is re-checked after publication so an operator deletion racing a long build cannot silently become current.

For an exact immutable identity already present in an explicitly marked running Hacocoon-managed Environment, Seed acquisition can copy a temporary `nerdctl save` OCI archive into the trusted Host cache and then delete the guest archive. It does not copy registry credentials, credential-helper output, workspace data, arbitrary Environment files, or live containerd state. Legacy/unmarked Environments are not inspected; failed or unavailable harvest falls back to the existing trusted Host pull path.

v0.20 makes local rootfs storage explicit without giving Hacocoon its own block lifecycle. Before an Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, local composition lazily ensures `haco-local-default` through Incus. Incus creates and owns the sparse backing file, loop device, Btrfs filesystem, and mount; Base, Tooling, Seed, trusted-host, and Environment rootfs volumes, snapshots, and clones share that Incus-owned Btrfs pool while Host Workspaces remain bind-mounted outside it.

v0.21 makes transparent compression the default creation policy: Hacocoon asks Incus for `btrfs.mount_options=compress=zstd:3`, does not request `compress-force`, and keeps `autodefrag` out of the desired default. Incus remains the mount lifecycle owner, and Hacocoon does not rewrite existing extents merely to recompress them.

v0.25 is the real ordinary-user acceptance checkpoint for that storage path. Dedicated GitHub-hosted Ubuntu 26.04 CI runs the actual `haco` binary against real Incus, verifies lazy `haco-local-default` creation, confirms the Incus-owned sparse backing image and loop attachment, checks the live Btrfs mount and zstd policy, exercises writable `haco create` / `exec` / `delete` / `run`, proves pool reuse, and verifies guarded cleanup. See [`design/btrfs-storage-layout.md`](design/btrfs-storage-layout.md).

Local Registry is not a prerequisite and has no reserved milestone. Remaining storage acceptance includes authenticated/private-registry combinations using Host-owned credentials without leakage, physical Btrfs compression ratio and CPU-cost measurement, COW/compaction behavior, broader real-host failure injection, Windows/WSL behavior, and supported-host verification beyond the automated GitHub-hosted Ubuntu 26.04 CLI lifecycle.

## Docker compatibility

v0.18 is implemented at the repository gate. This code landed while the feature was temporarily numbered v0.17 and is reclassified without rollback. `HACO_PLUGIN_OCI=docker` exposes `haco plugin oci docker status <environment>` and `prepare <environment>`. `prepare` does not install packages or mount Host sockets: it requires the selected Base/Seed to provide Docker CLI, dockerd, containerd, systemd, the docker group, and the Hacocoon-pinned socket/service units. It fails closed on unit drift or an already-active vendor Docker daemon instead of silently taking it over.

Real Incus/systemd acceptance remains host-dependent and is tracked separately from repository implementation status.

## Cloud status

v0.7 retains the provider-neutral Environment routing seam because that architecture remains useful. The former concrete EC2/AWS/EBS implementation was intentionally removed while the local/provider contracts are still moving. **Cloud implementation is currently deferred** and must not be described as active or accepted.

## Acceptance gaps

Repository tests do not substitute for all real-host acceptance. v0.23 proves a phased real-Incus substrate plus Core lifecycle on GitHub-hosted Ubuntu 26.04, v0.25 additionally proves ordinary-user Incus-owned Btrfs CLI behavior, and v0.26 proves trusted-host lifecycle/control-socket isolation on real Incus. Real Incus networking/resource behavior beyond those paths—including proxy-only bridge ACL/dnsmasq behavior—Windows/WSL + VS Code and interactive `haco-host` entry, private-registry credentials, Docker compatibility, physical Btrfs compression/COW/compaction behavior, broader storage failure injection, desktop notification delivery, and future cloud adapters remain environment-dependent. Partial acceptance in an earlier milestone does not prevent later minor checkpoints from advancing.
