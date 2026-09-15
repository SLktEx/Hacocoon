# Acceptance evidence and limits

[日本語](acceptance-evidence.ja.md) | English

Status: recorded acceptance evidence. These tests ran on the identified historical commits; this documentation refactor does not rerun or claim real-host acceptance. See [implementation status](../IMPLEMENTATION_STATUS.md) for current availability.

Read each pass, failure and skip within its fixture and candidate. A narrower or later pass does not establish the cause of a different failure. Maintain evidence that changes support decisions and unresolved limits here, rather than appending daily run logs.

<a id="portless-ssh"></a>

## Portless SSH and cold editor reconnect

At `e35a152e3d58fc917192d56e442517bd7805b8ff` in
[PR #631](https://github.com/SLktEx/Hacocoon/pull/631), the
[Windows native run](https://github.com/SLktEx/Hacocoon/actions/runs/34756266534)
passed real Windows OpenSSH command execution, managed alias/configuration,
strict host-key checks, four simultaneous cold reconnects and deleted-target
refusal. After Environment stop, controller stop and WSL termination, the first
Hacocoon contact was `ssh.exe` through ProxyCommand. The same provider generation
and Workspace marker survived. `ss -H -ltn` showed no new Host TCP listener,
Incus had no SSH proxy device, and SSH metadata had an empty Host and port zero.

A second cold cycle began with standard VS Code's saved remote-folder URI.
VS Code 1.136.1 and Microsoft's Remote-SSH 0.128.0 passed actual editor file
read/write, remote terminal execution, local approval stale refusal and probe
cleanup. No Hacocoon extension established the connection; the disposable UI
observer only checked the resulting session. Windows export/import SSH with a
fresh host-key pin, retained-data recreation, preview and public reclamation
also passed. [Repository CI](https://github.com/SLktEx/Hacocoon/actions/runs/34756266527),
[Ubuntu installation](https://github.com/SLktEx/Hacocoon/actions/runs/34756266617)
and [real Incus Core/Btrfs](https://github.com/SLktEx/Hacocoon/actions/runs/34756266512)
passed on that candidate.

The Windows aggregate still failed when its driver wrote `exit` to the old Host
terminal destroyed by the intentional WSL shutdown. The driver now closes the
terminal before cold checks and verifies ordinary Host entry in a new terminal
afterward; the earlier failed aggregate is not a whole-job pass. Earlier cold
fixtures used a lost `/tmp` Workspace; `/var/tmp` fixed that prerequisite.
At `d6059131`, cold SSH passed but the fixture omitted the standard Remote-SSH
extension and transfer still expected a Host port; these failures remain recorded
in [run 34755298769](https://github.com/SLktEx/Hacocoon/actions/runs/34755298769).

Repository regressions cover raw binary stdio/UDS, EOF and half-close, cancellation,
controller delay/disconnect, stale identity/lease/grant refusal, concurrent resume
and owned-fragment cleanup. A real PC power-cycle, manual Remote Explorer mouse
selection, VPN/NRPT and broad IDE compatibility were not tested. The private-registry
job is manual-dispatch-only and skipped in PR runs. Network-independent initial SSH
setup on official Bases remains [Issue #603](https://github.com/SLktEx/Hacocoon/issues/603).

<a id="incus-lts"></a>

## Incus 7.0 LTS baseline

The supported contract is `>= 7.0.1`, `< 7.1`; previous 6.0.5 results are
historical compatibility evidence. At development candidate `0c79f8209eec42b597cc811a9114e0351d8226d7`
in [PR #583](https://github.com/SLktEx/Hacocoon/pull/583),
[Ubuntu installation](https://github.com/SLktEx/Hacocoon/actions/runs/34724986358),
[fresh Windows/WSL installation, restart and reinstall](https://github.com/SLktEx/Hacocoon/actions/runs/34724986361),
and [standalone/Core/Btrfs Incus gates](https://github.com/SLktEx/Hacocoon/actions/runs/34724986357)
passed on server 7.0.1. Native gates include lifecycle, egress, Base build,
snapshot/copy/import, retained Store operations and owned cleanup.
[Repository CI](https://github.com/SLktEx/Hacocoon/actions/runs/34724986411) also passed.
Private registry, VPN/NRPT and human notification decisions remain unverified.

The main-targeted #479 change extracts the shared installer, doctor and required
vendor-daemon/export/fixture fixes. The preceding integrated-candidate passes
are not a native rerun of that extraction or evidence of publication. The
independent extraction's acceptance is recorded below.

At extraction `9a4dc42`, [repository tests](https://github.com/SLktEx/Hacocoon/actions/runs/34739589129),
[Ubuntu](https://github.com/SLktEx/Hacocoon/actions/runs/34739589125) and
[real Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34739589134) passed.
[Windows](https://github.com/SLktEx/Hacocoon/actions/runs/34739589114) passed fresh
installation/restart/reinstall, egress, transfer, reclaim and retained-data restore,
but the desktop aggregate failed approval review and subsequent preview setup.
The approval fixture piped answers into a terminal-only command. It now uses a
private PTY, preserving JSON receipts and bounded child cleanup; acceptance readers
also request `--json` explicitly after main's output change.

At corrected extraction `34ff371cedb7558959201b316a2aebe7f3542eee` in
[PR #600](https://github.com/SLktEx/Hacocoon/pull/600),
[Windows/WSL](https://github.com/SLktEx/Hacocoon/actions/runs/34741178336),
[Ubuntu](https://github.com/SLktEx/Hacocoon/actions/runs/34741178335) and
[Incus Core/Btrfs](https://github.com/SLktEx/Hacocoon/actions/runs/34741178370)
passed, including the Windows desktop aggregate. The earlier failed aggregate
remains a failure. [Repository CI](https://github.com/SLktEx/Hacocoon/actions/runs/34741178334)
passed after retrying only the Go 1.27 job: the first attempt timed out in the
existing interactive PTY resize test; 30 local repetitions with the same shuffle
seed passed without a code change. The intermittent timeout's cause is unconfirmed.
These results establish the extraction's tested scope, not release publication
or acceptance of later main integrations.

After main integration at `d6f078e`, [Windows run 34742409841](https://github.com/SLktEx/Hacocoon/actions/runs/34742409841)
passed installation and the first Host diagnostics on Incus 7.0.1, then failed
ordinary entry immediately after WSL termination/restart with `Host setup is busy`.
The fixture subsequently timed out; later Environment/desktop gates were skipped.
Shell preparation now waits for controller setup exclusion within its existing
deadline, retaining explicit-setup conflict refusal and failed-recipe recovery.
Component/race coverage checks waiting, cancellation and exclusion release;
the repaired integrated candidate still requires Windows acceptance.

<a id="installation"></a>

## Installation and Host

| Candidate / gate | Result and limits |
|---|---|
| `fced264` / standard Host tooling | Dedicated Incus/Btrfs fixture on WSL amd64 passed fresh Git/gh/containerd/nerdctl/BuildKit provisioning, public BusyBox pull/run, Dockerfile build/run, service restart, Host stop/start, repeat setup preserving image identity and BuildKit cache IDs, and offline image execution/deletion in an independent Store. Fixture `haco-area-55b79c9d56f597f1` completed owned cleanup. Earlier attempts exposed service readiness, system D-Bus startup and systemd argument expansion failures; regression fixes are included. Full released Windows installer, arm64 runtime, authenticated registry and custom existing-runtime migration remain unverified. |
| `1817e7c` / managed-user preparation | On a dedicated recovery WSL, the old function failed because the `hacocoon` group already existed. The corrected installer function created the managed account and verified its default-user setting after an exact-distribution restart; PowerShell component regressions passed. This is account-preparation acceptance, not a complete packaged installation or whole-installation restore. Fresh Japanese-Windows locale/entry acceptance remains pending. |
| `c749ff9`, `81c0d16` / `9049df3`, `4df465a` | Packaged Windows/Ubuntu setup, controller round trip, proxy-allowed/direct-denied traffic and WSL registration continuation passed within M0–M1. Actual Windows OS reboot was outside that scope. |
| `029ff08`, `42e2fb3`, `1b2d6ae` / run 34051931616 | Earlier Incus SIGKILL failures remain recorded. Stale cross-namespace PID replay is strongly supported by PID/worker traces; no userspace stack proved every kill source or OOM. The boot guard passed its dedicated gate; same-boot PID reuse is outside its protection. |
| `63fdf24`; current `73f63f2` | A WSL account lookup failed while the account still existed; cause unconfirmed. Current code retries only bounded read-only readiness/account probes. The current P/PF binfmt registration fix has repository coverage; the earlier manual workaround is not packaged-fix acceptance. |
| `c86c43e`; `61a26e3` | Recorded native Windows CLI/drive projection, retained data and standard OpenSSH passed on their candidates. Legacy Base switching/distribution at `029ff08` is historical. A real WSL restart at `61a26e3` retained owned instances/data; hotplug, broader Windows tools and interrupted upgrade recovery remain unverified. |

Full original receipts, fixture identities and log/artifact links remain in [the original implementation record](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md). These immutable records are historical evidence, not current operating instructions.

<a id="development"></a>

## Daily development and approvals

| Candidate / gate | Result and limits |
|---|---|
| `1817e7c` / installed source-guard observer | A dedicated Incus/WSL observation passed after canonical start of a stopped recovered Env, pinning its generation and actual native guard rules. The first startup failed before controller socket readiness; an earlier standalone observation failed too. These failures remain. The full packaged Windows SSH gate, spoofed-packet delivery and complete reboot/recreate sequence are not established by the observer. |
| `7a4d122`; `f8517ba`; `8752431` | Managed clone/Workspace/SSH/Git approved push/stop passed on installed candidates. Resume and SSH-key addition passed at `f8517ba`. Manual VS Code Remote-SSH passed on the older `8752431` installation after an initial resume failure; temporary rules/connection were removed. |
| `347ca50`, `d4aef8d` / run 34139245378; `bcc1baf` | Installed recipe save/replay/update/clear, basic Workspace setup, HTTP/Edge preview and doctor prerequisites passed. Recreation/cancellation, default-browser launch and VPN/NRPT are separate gaps. |
| `2584ec6` / run 34152700897; `71dbb4f` | Configuration round trip passed while preview doctor failed. A local empty-saved-array configuration failure was fixed; later preview success does not establish the earlier failure's cause. |
| `eb16300b6700` | Real GitHub approved push, saved ask reuse and denial passed. Other saved choices have repository coverage only. Remote test branch `codex/stage-b-b-first-20260906` reached `3ca59c…`; denied `26a7b…` and fixture `git-save-eb16300` were retained. Ambiguous push completion still requires remote inspection. |
| `470a2b8` / run 34188963290 | Windows notification service, VS Code/terminal review and stale-refusal paths passed. Fresh human toast decisions and Linux activation remain unverified. Earlier `711005a` hit service start limits; the reset/new-unit follow-up is separate evidence. |
| `c05528a`; `226991b` / run 34479510230; `684e411` | DNS equal-policy/default-deny checks passed; idempotent DNS service startup and installed Git-over-SSH passed in later gates. SSH now receives proxy variables automatically. Real reboot/VPN/NRPT combinations remain incomplete. |
| `4adfe19` / run 34115004878; `093ed159b80e` | Temporary execution and cancellation cleanup passed on real Incus; interactive input/TTY and populated OCI acceptance are absent. AWS guest refusal and bounded synthetic download passed; authenticated real AWS listing/download paths were skipped for missing prerequisites. |

Full original receipts, fixture identities and log/artifact links remain in [the original implementation record](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md). These immutable records are historical evidence, not current operating instructions.


Earlier development failures remain material: DNS setup failed at `72096d8` before lookup; `7eecbdf` identified manager readiness; `c05528a` then failed in CRLF setup input; `5f824b4` exposed lost stdin delegation; `39b5ce4` exposed DNS start-limit-hit. Preview at `bffc3fd` failed its extensionless-marker byte/text assertion. Later passes are scoped corrections, not proof of every cancellation/restart case.

The local `71dbb4f` configuration/preview fixture kept default deny and eight administrator rules, applied then removed four narrowly scoped archive rules, read the exact Workspace marker on port 36059 and cleaned up its own resources. It did not prepare SSH; the earlier failure's cause remained unknown. Full file-specific receipts: [configuration](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/reference/configuration.md), [name resolution](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/name-resolution.md), [project setup](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/project-setup.md).

<a id="storage"></a>

## Snapshots, cleanup and reclamation

| Candidate / gate | Result and limits |
|---|---|
| PR #493, #501; `a2fcb72` / runs 34297739368, 34297739417 | Snapshot restore is independent of Base filesystem retention and source deletion; fresh authority is created. Base build/create/SSH passed after earlier machine-ID/stdio and 600-second timeout failures. Exact ownership was cleaned only after absence checks; diagnostics remain historical receipts. |
| PR #504–507; `c4842c2` | Workspace, built Base, whole Store and source-repository cleanup passed their native gates. Base Windows alias SSH initially failed; source-repository tests initially stopped at instance initialization. Downstream skips in those failures are not passes. |
| `4d9038b7`; `bd1c9a5` / run 34417051340; `9484d06` / run 34493016558 | Attached/Host image operations passed runtime-adapter fixtures. Detached nerdctl delivery and bare controller/CLI passed in 588.51s; reviewed unused-image deletion also passed. Full installed-controller/Standard composition and detached Docker are still incomplete. |
| `f3f5557`, `ae0c245` | Host OCI area isolation, offline copy and bounded completed-copy recovery passed with Docker 28.5.2/vfs and nerdctl 2.3.5/containerd 2.3.3 fixtures. An initial root mismatch failed before correction. Unknown provider completion still blocks release; broad runtime/version/installed acceptance is not implied. |
| `5100d86` / run 34623036552, job 103341362151 | Public reclaim dispatch/status passed: Windows allocation 7,964,983,296 → 4,224,712,704 bytes (3,740,270,592 recovered), 1 TiB virtual and 128 GiB Incus capacity unchanged. Linux discard, exact-distribution stop/compact/resume and retained Workspace/OCI/snapshot restore passed. |
| `4369fdb`, `d675c5a`, `de72119`; earlier Windows trials | Job-related launch errors have unconfirmed causes. `d675c5a` retained pending with access-denied 5 and no Linux start; `de72119` saved a worker failure without reporting it to the launcher. Earlier OpenVirtualDisk error 32 and partial disk-only successes do not prove combined reclaim. Junction refusal passed; some symlink checks skipped for privilege. Existing-installation, power-loss, cross-session and real interrupted-worker review remain unverified. |

Full original receipts, fixture identities and log/artifact links remain in [the original implementation record](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md) and [the original feature evidence](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/storage-reclamation.md). These immutable records are historical evidence, not current operating instructions.

<a id="transfer"></a>

## Environment transfer and evacuation

| Candidate / gate | Result and limits |
|---|---|
| `653dc985` / [Incus/Btrfs job](https://github.com/SLktEx/Hacocoon/actions/runs/34636086219/job/103384200857) | Stopped containerd transfer passed again in 115.96s: canonical and shipped-controller import retained image identity and writable data after source deletion, then explicitly started the saved container. The source container and daemon were stopped before export. No running task, Docker, arbitrary application or whole-installation restoration is claimed. |
| `1817e7c` / native image evacuation | Two split container images crossed dedicated WSL installations through Windows-retained files. All parts passed SHA-256 checks, imported fingerprints/types matched in a new isolated Incus image project, and retained files stayed unchanged. Initial `incus project show --format` observation failed because that option was unsupported; its receipt remained and `project list` observation preceded import. The project and images remain retained. Unified images, new-Env boot and whole-installation replacement were not tested. |
| `3d0dd9a` / run 34430493864; `b7297a3` / run 34455660292 | Dedicated Linux export and aggregate import passed. Local export at 480/720 seconds and local aggregate import at 720.07 seconds failed. Catalogs `2545909325`, `462967548`, `1920048809` identify retained evidence; successful narrower gates do not erase those timeouts. |
| `a58d553`, `6d5e027`, `e598270`, `0cc27a5`, `7517c27`; `684e411` / run 34471376143 | Bare controller import passed in 20.35s while an 89.56s aggregate failed. Later SSH, missing Git (127) and apt (100) failures preceded installed export/import/source-delete/fresh pinned SSH success. Separate approval failures in the successful Windows transfer run remain failures. |
| `c4449e1` / run 34482712957; `6974272` / run 34501951826 | Windows projected-file size/hash and import passed. Real stopped containerd data transfer passed (aggregate 103.36s, controller 22s); earlier `ba4dbcd`/`8103e3f` failed before export. Native Windows CLI/direct DrvFS delivery, authenticated imported Git, Docker/BuildKit and arbitrary live database consistency remain incomplete. |
| G2 inventory and direct-file fixtures | Read-only schema 10–13 inventory (schema 9 unsupported), catalog/native-image references and explicit unresolved projections are implemented. One file walk saw 47,848 entries: 39,061 files, 5,050 directories, 17 mounts, 3,718 symlinks, 2 special files. Its report `/var/lib/haco-file-inventory-4kvt5eyb/wsl-root.json` records gaps; enumeration is not capture or deletion authority. |
| G2 synthetic recovery fixtures | Direct tar capture/restore passed in 20.59s, saved-rootfs capture in 24.52s and an EPERM snapshot-delete fixture's manual capture/restore/cleanup in 11.74s (root files 9.37s). These are isolated fixtures, not whole-installation or actual corruption recovery. |
| Historical encrypted fixtures | A 10,440-byte encrypted acceptance fixture remained after its original identity under `/tmp` became unavailable across PrivateTmp. Its ciphertext cannot be called recovered; later retention fixes (2.31s) and a new Windows-owned identity do not recover the missing key. Current capture uses ordinary archives with no mandatory encryption/key setup; maintained native CI checks contents, independent hashes, explicit xattr restore and incomplete-capture refusal. The encrypted fixtures are historical optional evidence and no longer run in maintained CI. |
| `61a26e3` managed cross-WSL fixture | A separate fresh WSL imported a 550,415,872-byte bundle in 171.27s; all 94 fixture-volume entries, guest identities, fresh pinned SSH/local Git and retained Workspace/OCI after recreation passed. Initial raw Host UID comparison failed because of ID mapping; apt exit 100 was resolved with narrow policy. Authenticated Git, Windows VS Code, whole-installation coverage and G4 replacement were skipped/incomplete; original WSL was retained. |

Full original receipts, fixture identities and log/artifact links remain in [the original implementation record](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/IMPLEMENTATION_STATUS.md) and [the original feature evidence](https://github.com/SLktEx/Hacocoon/blob/73f63f23b4a57d2fefa5764c523798b1fa8e1962/docs/design/environment-transfer.md). These immutable records are historical evidence, not current operating instructions.

[Additional scoped receipts at 1817e7c](https://github.com/SLktEx/Hacocoon/blob/1817e7cf9e8910bf31ae23b714053e2580d03fa0/docs/IMPLEMENTATION_STATUS.md).

<a id="development-branch-integration"></a>

## Integrated development branch evidence

These are imported, commit-bound results from the development branches, not tests
of the merged main candidate. Integration does not expand their acceptance scope.

| Candidate | Results, failures and remaining limits |
|---|---|
| `58c4a56` / `dev/1.x` | Maintained test/vet, race, fixture E2E, systemd, isolated forwarding and 15 pinned AWS SDK contract tests passed. All-stage CI stopped at the Ubuntu >=26.04 installer requirement on Ubuntu 24.04; that guard was preserved. Native Incus 6.0.0 observation/deletion passed with exact cleanup. Rootfs import first failed on native `amd64` metadata; archive regressions reproduced it, then the corrected Btrfs aggregate passed in 64.20s with public export/import, snapshots/copy, fresh generations and retained Git/Workspace/OCI bytes. Both failed and successful fixtures were cleaned by exact ownership. Shipped-controller import, actual SSH, live OCI and installed Windows were not tested in this run. |
| `6cf9295` / `dev/v2` | A locally built installation in dedicated Ubuntu 26.04/Incus 6.0.5 passed setup, all six Host doctor checks, external Workspace create/open, Linux SSH edit/build, stop/start, duplicate refusal, blank-selection cancellation and Env deletion retaining files. Synthetic customization exit 29 reported its fixed stage/reason/request ID without leaking private output. Setup interruption retained exclusion until actual completion. Initial SSH failed with default deny/absent sshd; four scoped package rules enabled setup and were removed afterwards. The dedicated network namespace and disabled kernel AppArmor do not establish default installation networking or AppArmor confinement. Windows IDE/default-entry, private Git/registry, OCI retention and cold restart were not exercised. |
| `ae19db6` / `dev/v2` | Maintained test/vet/JS, race, fixture E2E and 22 interop tests passed. Windows installer component fixtures passed with mutation paths mocked and read-only transport pinned to a selected WSL. Linux PowerShell could not run that Windows-only fixture because SystemDirectory was empty; component results do not establish native installation. |
| `72058fc` / `dev/2.x` | Dedicated Incus/Btrfs installed CLI passed synthetic external IPv4/IPv6, Physical Host and peer Env TCP/UDP and Host-to-Env forwarding (0.099–0.169s per fixture journey), expiry, revocation, Policy expiry, replaced generation refusal and DNS pinning. An initial DNS fixture ran before controller readiness; bounded observation corrected its order. These are synthetic local fixtures, not public Internet, corporate VPN or production-service acceptance. |
| `ac67fad` / `dev/2.x` | Installed CLI/Incus passed two-repository preparation/reopen, SSH edits, retained files/Store data after recreation, independent stopped forks, no-OCI operation, explicit Store reuse and Base replacement. A destination OCI collision retained incomplete ownership/source reservations and refused reopen without changing the pre-existing Store. Windows SSH and an SSH-forwarded browser fixture passed with dedicated files and an explicit distribution/namespace route; automatic Windows open, VS Code UI and default installer networking were not established. Registered Windows TCP/UDP services answered local Windows probes but WSL/controller probes timed out; approved guest TCP recorded connect/failed/timeout, and UDP had no response. Outbound Windows-service access and its failure cause remain unverified. |

For the Workspace fixture, preparation took 0.556s / 126,976 Btrfs pool bytes;
open 7.064s / 25,333,760 bytes; reopen 0.793s / 147,456 bytes; fork 1.128s /
458,752 bytes; fork open 6.384s / 23,162,880 bytes; recreation 3.396s /
23,650,304 bytes. Base replacement took 17.986s with allocation unmeasured.
Each small source reported 12,075,008 extent bytes; prepared copies reported zero
exclusive extents. Pool deltas include metadata/runtime activity and do not prove
Linux-kernel-sized repository performance or controlled-load benchmarks.

At integration candidate `215019a`, maintained docs/workflow-policy, full Go test/vet,
27 JavaScript tests, full race, fixture E2E and systemd checks passed. Native Windows
installer component fixtures passed with mutations mocked. All-stage local CI stopped
at the Ubuntu 26.04 installer precondition on the Ubuntu 24.04 validation Host.
The forwarding entry first stopped because noninteractive sudo was unavailable;
the same kernel regression passed in a separate root-owned network namespace (3.25s).
These integration checks do not establish installed Incus, Windows/WSL product journeys,
private registry or live OCI acceptance of the merged candidate.

<a id="ci-reliability"></a>

## PR CI reliability incidents (#615)

These historical observations belong to [#615](https://github.com/SLktEx/Hacocoon/issues/615).
A successful rerun is evidence of an incident, not its resolution. Current routing
and gate semantics are owned by [PR verification contracts](../reliability/ci-contracts.md).

| Candidate / evidence | Finding and resolution status |
|---|---|
| `f3ef57b3ea028e10e942418a8408edd89a94b605` / [attempt 1](https://github.com/SLktEx/Hacocoon/actions/runs/34740688741/attempts/1), [attempt 2](https://github.com/SLktEx/Hacocoon/actions/runs/34740688741/attempts/2) | `test (1.26.x)` failed `TestSizedInteractivePTYReadlineResizeAndExit` waiting for the resized terminal marker; the same SHA passed on attempt 2. Fixture synchronization now waits for a foreground command before resize, avoiding readline's terminal-size restoration window. The three Linux sized-PTY regressions passed 100 repetitions locally with Go 1.27.0; this does not establish hosted native acceptance. |
| `69c85fb5214ba1a4a81c2c50cdec9d789c924315` / [storage attempt 2](https://github.com/SLktEx/Hacocoon/actions/runs/34738362521/job/103675967861) | `TestRealIncusResourceMaintenancePreparationE2E` expected interactive decline but supplied a pipe; shipped CLI correctly refused nonterminal confirmation with exit 2. The fixture now supplies a real Linux PTY, with a child-process terminal/read regression. Native maintenance revalidation is required. |
| `84062060e0ef465e73ec45b43b6ed785ce879d81` / [Windows job](https://github.com/SLktEx/Hacocoon/actions/runs/34740317809/job/103678816517) | Post-termination ordinary Host entry reported `Host setup is busy`; the harness then waited for its deadline. Fail-fast reporting alone did not fix that product defect. The subsequent login-bootstrap fix and native restart evidence are recorded below. |

At #615 candidate `4abadc16399dfdb1997351c7803fd76131cfdeed`, based on `7b4e2356d73a163b31e784a0a5b7400fed1a05cf`,
full Go test/vet, race, shipped-command fixture E2E, documentation and workflow policy
passed on the local Linux validation environment. Real Incus and packaged Windows/WSL
were not run there. Commit-bound hosted results must be recorded separately.

After integrating main at `8c645317101e007d57c752f35ae0a95f637d81b5`, related composition/Incus/product-CLI tests and vet passed with Python 3.13.15. The first local run failed because Python 3.10 lacked `tomllib`; installing the verified separate runtime satisfied the new Host-tooling test prerequisite without weakening the test. Sized-PTY and maintenance-terminal regressions also passed 100 repetitions on pinned Go 1.26.7. These are repository/component results, not installed native acceptance.

Candidate `8c645317101e007d57c752f35ae0a95f637d81b5` / [Windows job 103689222832](https://github.com/SLktEx/Hacocoon/actions/runs/34744299884/job/103689222832) reproduced the restart busy failure. Initial install and ordinary entry passed; restart entry failed in 11.218 seconds. Reinstall and downstream SSH/IDE/network/reclamation/notification steps were not executed. WSL source inspection identified the competing PTY-backed PAM login bootstrap; [ADR 0066](../adr/0066-wsl-login-bootstrap-routing.md) records the routing fix and rejected retries. The native restart results below verify that fix separately from later acceptance failures.

Candidate `75007eccd3b6d4290e456b1e346031203dcef227` / [test run 34745868490](https://github.com/SLktEx/Hacocoon/actions/runs/34745868490) waited behind superseded run 34744299866. Its required jobs were cancelled, but job-level `always()` kept the old evidence job queued and retained the concurrency slot. Evidence jobs now use `!cancelled()`: failed/skipped dependencies still require evidence, while whole-workflow cancellation can finish. This was a CI implementation defect, not proof of a runner outage. Static regressions reject restoring the uninterruptible condition.

At `7c73399bc36f2a6055c3f95d3c1f3671666481d5`, [repository checks](https://github.com/SLktEx/Hacocoon/actions/runs/34746556831) passed both Go series, race, CLI E2E, both build architectures, release packaging and the evidence gate. [Native Ubuntu installation](https://github.com/SLktEx/Hacocoon/actions/runs/34746556876) passed the unchanged installer, added ordinary-user product CLI lifecycle/Workspace retention, legacy journey, network isolation and evidence gate.

Windows [75007ec job](https://github.com/SLktEx/Hacocoon/actions/runs/34745868528/job/103693588946) and [7c73399 job](https://github.com/SLktEx/Hacocoon/actions/runs/34746556856/job/103695440904) both passed install, terminate/restart, reinstall and installed egress; restart entry took 33.547 and 35.844 seconds. Native interop, Windows SSH and VS Code Remote also passed, but both jobs **failed** configuration and pending-approval fixtures. Those fixtures parsed human-default output without `--json`; the fix requests JSON for configuration read/apply and pending lists, with executable fixture regressions. Later reclamation and notification steps were not executed. These runs establish restart recovery, not complete Windows acceptance or a same-SHA rerun pass.

At the same `7c73399` candidate, [native Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34746556850) passed standalone and Core lifecycle/egress, but Btrfs failed aggregate export and the source-deletion fixture. Incus 7 requires `--force` for the adapter-owned existing anonymous output. The merged #600 implementation supplies that flag for the supported 7.0 LTS baseline; this branch reuses it without a separate compatibility shim. The source snapshot observation also needed separate volume/snapshot arguments. Cleanup refused retained failed fixtures but previously continued toward pool/project deletion; it now stops before those operations on uncertain ownership or absence. Native revalidation is required.

At integrated candidate `fb5da79768c3fac5bf69db3c0496f936e9e1646f` (main `f590023`), local workflow policy, Actionlint, docs, product CLI/composition/Incus tests and vet passed. Eight JSON/terminal fixtures and five cleanup tests passed. The new LTS installer fixture had one failure because the validation Host is Ubuntu 22.04; the supported >=26.04 guard was not bypassed.

The four first-attempt hosted runs ([test](https://github.com/SLktEx/Hacocoon/actions/runs/34748814241), [Incus](https://github.com/SLktEx/Hacocoon/actions/runs/34748814235), [Ubuntu](https://github.com/SLktEx/Hacocoon/actions/runs/34748814274), [Windows](https://github.com/SLktEx/Hacocoon/actions/runs/34748814262)) ended in `startup_failure` with no jobs. The test run's annotation reports an unexpected GitHub error, request ID `CFDF:38DCEF:B99783:11CB2DC:6AA66658`. The public status page showed no reported incident at inspection; no platform-wide outage or recovery is inferred. This is no native product acceptance. The incident exposed a history-reader gap for jobless startup failures; workflow-attempt conclusions are now recorded independently of jobs, with a regression for startup failure followed by a successful attempt. No rerun was requested.

The active [Protect main ruleset](https://github.com/SLktEx/Hacocoon/rules/21838612), read through the existing GitHub connector, requires docs, workflow-policy, release-config, both Go checks, race and e2e. The four additional evidence contexts were absent at inspection. Their addition remains a separate required configuration action; no ruleset settings were changed.

At `def11e9ff31131e02be0eb3270bb3ebd62e1c452`, [repository checks](https://github.com/SLktEx/Hacocoon/actions/runs/34749383437) passed including `test-evidence`. [Ubuntu product acceptance](https://github.com/SLktEx/Hacocoon/actions/runs/34749383422/job/103703362684) passed, but its evidence gate failed: artifact 10315158077 contains successful required steps and `needs_success=true` while the API still reported the completed job's conclusion as null. A bounded read-only metadata observation now covers that propagation window without polling away terminal failures.

[Core](https://github.com/SLktEx/Hacocoon/actions/runs/34749383438/job/103703364764) and [Btrfs](https://github.com/SLktEx/Hacocoon/actions/runs/34749383438/job/103703364605) passed every product step, including aggregate transfer, Base build, Store COW and maintenance. Both jobs failed cleanup because Incus decorates the current project's CSV name and the strict identity check rejected it. Cleanup now consumes validated JSON names and still requires positive absence. These are failed jobs with scoped product evidence, not complete native acceptance.

The same `def11e9` candidate's [Windows user-path job](https://github.com/SLktEx/Hacocoon/actions/runs/34749383429/job/103703363209) passed the complete maintained native journey: packaged install, ordinary entry, terminate/restart, reinstall, installed egress, strict Windows SSH/VS Code interop, configuration and approvals, transfer, public reclamation, notification and cleanup. This is the first full Windows product-job pass in this investigation, not a rerun of a failed SHA. The workflow evidence gate also passed; later CI-helper changes remain separate from this product receipt.

At `d1c7480bd69157fb65974e9e2f2673e2ffffe4b6`, repository, Ubuntu and all required Incus jobs including their evidence gates passed. The [Windows job](https://github.com/SLktEx/Hacocoon/actions/runs/34750642440/job/103706445008) failed after public reclamation and Host resumption succeeded: detached Workspace/OCI reattachment through `haco env create` returned nonzero. Snapshot restoration and its retained content had already passed. The fixture discarded stderr, so the root cause remains unresolved; the preceding candidate's pass does not resolve this failure. Notification was not reached. Retention diagnostics now preserve numeric exit status, an allowlisted CLI reason and bounded read-only controller observations without raw output or mutation replay. These observations identify investigation boundaries, not proven root causes.

<a id="main-cli-language"></a>
## Main daily language integration candidate

`codex/main-daily-ux` reuses #580/#583 language/help work on main `ed3ad1a5`.
The first local focused attempt failed two stale test expectations: approval
listing was parsed as JSON without `--json`, and Open help was read from stderr
instead of its new local stdout route. Tests now use the actual public contract;
product output/authorization behavior was not reverted. The second immutable local
source passed focused product/Host/catalog/approval tests (2.60s), the maintained
`bash tools/ci-local.sh test` (all Go tests/vet plus Python/client checks, 13.28s),
related race tests (9.93s) and docs/checker regressions (4.73s). All current Go
sources match the tested archive. Hosted/installed checks for this candidate remain
pending; this is implementation and local verification, not distribution.

The shipped CLI E2E initially failed its historical one-line Environment usage
expectation. Reuse #583's existing `195172f4` fixture correction: assert the vertical
heading, command and create entry, compare usage with explicit help, and preserve
exit-code/stdout/stderr checks. The corrected black-box CLI E2E passed in 5.16s.
The matching installed-Incus assertion is updated, but no new native run is claimed.

The candidate's quality run 34883913571 passed coverage but failed lint on nine
unacknowledged output-write results. These are now explicitly acknowledged:
already-failed diagnostics remain failures, and help path recognition cannot fall
through into command execution when output closes. Existing return semantics are
preserved. This is separate from the successful repository test workflow.

Previous #583 head `0c79f820` passed repository, Ubuntu, Incus and Windows workflows
([Windows run](https://github.com/SLktEx/Hacocoon/actions/runs/34724986361)).
That development result does not prove this main integration or fresh Japanese
Windows, human GUI decisions, long-input/resize or the original SSH-failure route.
Performance measurement and M2–M5 acceptance remain separate.

At `2eb2e2f5`, repository checks passed, but quality run 34885076668 found
additional unchecked localized diagnostic writes; Ubuntu run 34885076467 completed
installation and failed the old horizontal-help assertion before later acceptance
steps ran. Those failures remain distinct from skipped downstream checks. The
remaining changed writes now explicitly preserve their existing outcomes, and the
Ubuntu assertion uses the same vertical heading/command/create contract as the
shipped CLI E2E. Local focused tests (2.58s), the CI-pinned golangci-lint 2.13.2 with
all changed-code findings shown (6.62s), workflow policy (1.05s) and CLI E2E (3.06s)
pass. Native acceptance for this correction remains pending.

At final PR #660 head `24cd508369ca5937495b378f69cec4414b9e8cfd`, repository,
quality, Ubuntu, Incus and Windows workflows all passed (runs 34886106686,
34886106919, 34886106764, 34886106700 and 34886106868). It was merged into main
as `7e876bc1e5e92432971427d3c778a4f5a07cb72e`. Earlier failed heads remain recorded.
This does not establish the later Windows-language handoff or human GUI answers.

<a id="main-notification-installer"></a>
## Main notification and installer integration candidate

The candidate reuses #583's failure grouping and BAT final-result implementation.
Local Go 1.27.1 notification/catalog/event tests passed (1.22s), notification/event
race tests passed (3.93s), maintained local test CI passed (13.11s), and docs/checker
regressions passed (4.79s). Existing tests exercise 100 distinct failed requests,
restart cursor preservation, independent targets and ungrouped approval/recovery.

The PowerShell 5.1 fixture script could not start because that shell's policy is
Restricted. This was an execution failure before tests, not a product failure or
pass. No execution-policy setting was changed. A direct invocation reused the
same native fixture source and the shipped BAT: 0/1/37/3010, missing PowerShell and
missing adjacent script all passed. The existing pywinpty 3.0.2 ConPTY fixture
passed final wait, explicit keypress and retained exit 37. It used an isolated
local Python environment and generated native stand-in, not a WSL installation.
The PowerShell wrapper's extra cleanup-sharing tests were not rerun by this route.
Fresh Explorer interaction, full packaged installation and original SSH failure
acceptance remain pending; previous native results do not establish this candidate.

<a id="main-host-language"></a>
## Windows presentation handoff on main

The implementation at `45b53f98bb3b24942011ddd7b0ff73474a8b65f5` reuses
#583's `3b8eefce`, `0c79f820` and `8c07e126` on the current M1 candidate,
with explicit `LC_ALL`/`LC_MESSAGES` kept ahead of Windows autodetection.
Local focused product/Host/catalog/control/Incus tests passed (15.05s), the full
maintained local test entry passed (14.38s), and catalog/control race tests passed
(7.57s). Initial changed-code lint failed on an unchecked test connection close;
the corrected control test passed (2.79s), and pinned golangci-lint 2.13.2 passed
with new files included (15.27s). Documentation checks/regressions passed (6.21s).

On this same PC, `TestNativeWindowsLanguageReadOnly` in `hacocoon-second` returned
`ja` through the actual system PowerShell query (0.26s test, 0.95s command).
Windows installer component tests passed under PowerShell 7.6.6. Neither test
changed OS/WSL locale or installed candidate binaries. This is a native read-only
query and component result, not fresh packaged login, complete Japanese text,
notification response or human GUI acceptance. Those remain open with #577.

After rebasing onto main `7e876bc1`, the immutable candidate passed the full local test entry (23.78s), shipped CLI E2E (4.54s) and documentation checks/regressions (5.17s). The Windows ordinary-entry observer now compares exactly one normalized Host language marker against an independent Windows UI-language query; all 13 observer tests passed locally. The first WSL launch failed before tests with `HCS_E_CONNECTION_TIMEOUT`; a later ordinary launch succeeded without restarting WSL. No product test was executed by the failed launch.

<a id="main-seed-retirement"></a>
## Seed retirement without version compatibility

`85c4c621cd478e300f481d16eb7142a358ea34e3` reuses the #655–#657 Seed
runtime/harvest/builder retirement on main `9e5f6f68`, then removes the remaining
sampling/recommendation and old image deletion/re-enable state. The user excluded
old-version compatibility/migration on 2026-09-15; no reader or conversion shim
is retained. Current Docker and managed Base/OCI operations remain.

The immutable local candidate passed focused CLI/composition/OCI/Incus tests
(4.31s), all maintained local tests/vet/Python/client checks (12.49s), pinned
changed-code golangci-lint 2.13.2 including new files (2.53s), workflow policy
(1.04s), shipped CLI E2E (2.95s), and documentation/regressions (4.64s). Final
status-only documentation changes passed the checker again. No native installation
or performance run was added for this candidate. Prior #655's scoped real-Incus
placement result is historical evidence for its SHA, not acceptance of this head.
The removed private-registry job tested only retired Seed acquisition; other
native jobs and historical failures remain. No user data or installation was deleted.

The candidate was rebased onto main `7e876bc1` without changing the Seed-retirement Go implementation. The first verification launch failed before tests with `HCS_E_CONNECTION_TIMEOUT`; the later ordinary WSL launch succeeded without restarting WSL. This startup failure is distinct from product validation. The main-based full local test entry passed (18.42s), as did the shipped CLI E2E (4.70s); documentation checks/regressions also passed (5.64s).

After PR #661 merged as main `44211fd2`, this Seed candidate was rebased while preserving both evidence sets. The verified combined source passed the full local test entry (13.96s), CLI E2E (3.41s) and documentation/regressions (4.96s). PR #661 final head passed all five workflows, including Windows run 34890523779. Seed head `8eda58ee` likewise passed all five workflows, including Windows run 34890531022; these remain results for their exact heads, not the rebased candidate.

<a id="main-git-branches"></a>
## Main Git branch workflow candidate

This candidate reuses #585 (`d93f61fb`) and #587 (`7bdd3db6`) on main
`7e876bc1`. An immutable source with 1,359 verified files passed Git/common
review/capability/product tests (2.80s), the CI-pinned golangci-lint 2.13.2
including new files with uncapped findings (5.00s), maintained local test CI
(10.98s), related race tests (23.59s), shipped CLI E2E (3.03s) and documentation
checks/regressions (4.49s), using Go 1.27.1.

Real local Git fixtures cover multiple heads and ref denial, moved/deleted heads,
new-branch denial, fixed approved commits, separate create/update saved choices,
main remaining subject to approval, concurrent different/identical creation, and
refusal of force/deletion/multiple refs. These are component results, not installed
Incus, authenticated GitHub, human GUI or large-repository acceptance.

The first focused invocation named nonexistent `internal/approvalreview`; the
existing Git, capability and product packages passed but the invocation failed.
The corrected package is `internal/review`. The first lint patch incorrectly
disabled Windows Git newline conversion and included unchanged files; its broad
findings are not presented as new-code findings. With the correct diff, lint
found two capitalized error strings and one switch simplification in the reused
code. They were corrected, then all final checks above passed. The earlier
invocation/lint failures remain recorded separately.

After Seed retirement merged as main `119e3007bc55333841a076f53d774be22ea5b711`,
PR #663 was rebased without changing its Git implementation. Combined local tests
(15.80s), CLI E2E (3.59s) and documentation/regressions (5.16s) passed. The old
head `6b436e4d` passed all five CI workflows, including Windows 34892114103;
those results are not substituted for this updated head. Seed #662 final head
`50e692d6` passed all five workflows, including Windows 34894991920 and the
same-commit evidence job 104155046690. No earlier failure or human acceptance gap
is erased by either result.

## Detailed guidance and single Host tool preparation

Candidate `4d7435cc` reuses #592/#593 and #659 on main `44211fd2`. Current
JSON opt-in, portless SSH, HTTP preview and once-per-Host setup semantics remain.
Focused CLI/catalog/Host/Incus tests (4.39s), pinned changed-code lint (4.09s),
full local tests (44.27s), related race (9.69s), CLI E2E (3.64s), documentation
(5.03s) and workflow policy (1.08s) passed on a verified source archive.

The first focused attempt exposed old fixture language selection and an overly
broad SSH-port assertion: current `haco open --port` selects HTTP preview. The
fixtures now use the shared locale selector and distinguish preview from portless
SSH. The next lint found two unchecked test-file closes; both are checked now.
These failures remain distinct from the subsequent passes. Fresh installed Host
preparation and original SSH-failure reproduction were not run for this head;
#655's original Host apt failure remains historical unresolved evidence.

PR #665 head `e6ef0431` passed repository, quality, Ubuntu and Incus CI, but
Windows run 34896159890 failed before packaging/installation in the first native
reclamation protocol subtest (job 104150566559, start, 30.09s timeout; only CLIXML
on stderr). The other five modes passed; subsequent product steps were skipped.
No changed file touched that protocol implementation. The same verified source
was built and run on this Windows PC: all six modes passed in 5.84s (command
7.67s), with start taking 3.68s. No WSL restart, reclamation, registration change
or execution-policy relaxation was involved. The CI timeout remains unexplained
and is not erased by local success. The candidate now includes main `119e3007`.

After merging main `119e3007`, the combined guidance/setup candidate `3c2d4c5c` passed the full local test entry (19.51s), CLI E2E (10.60s), and docs/regressions (5.55s). Windows CI is rerun for the updated head; the earlier protocol timeout remains unresolved evidence.


PR #665 head `108dd40cca4ea7bad0d0c8d1ddcc977a282d98aa` passed all five CI workflows, including Windows 34900315650. After merging main `9da3ec8f` as `35d5ea81`, combined local tests (13.58s), CLI E2E (3.22s) and docs/regressions (4.65s) passed. The earlier native protocol startup timeout remains unexplained; this later pass does not erase it. No fresh human desktop acceptance is claimed.


<a id="main-interactive-run"></a>
## Interactive temporary execution on main

The M3 candidate reuses #590/#591 on main's split lifecycle implementation.
At `acd61022`, focused tests (5.37s), changed-code lint (4.28s), all maintained
local tests (11.55s), related race checks (10.71s), CLI E2E (6.47s), docs (5.58s)
and workflow policy (1.17s) passed. Real local PTY and binary pipe fixtures are
component evidence, not installed Windows/Incus acceptance.

The first WSL launch failed before testing with `0x800705b4`. Later ordinary
launches worked without restarting WSL. Initial focused tests found missing
creation IDs and a hard-coded old schema in test data; the current schema tests
were corrected, not given migration behavior. A draft reference to a nonexistent
Environment field failed compilation and was removed: the canonical creation ID
is owned by the Workspace lease. Initial changed-code lint found unchecked
writes/closes, corrected before the successful run. All failures remain distinct.

The candidate is now based on PR #663's Git work (`ddb03e85`) over main
`119e3007`, with checkpoint v0.61 for interactive temporary execution. Main's
existing foreground-readiness PTY resize regression supersedes #594's older
approach; that patch failed applicability checking and was not applied. New
combined verification and installed acceptance remain separate from the tests
above. No old-version migration or fallback cleanup was introduced.

On the combined `8b4d00a2` source plus canonical v0.61 metadata, focused tests
(4.37s), uncapped changed-code lint (2.53s), full local tests (10.68s), race checks
(8.51s), CLI E2E (2.95s), docs/regressions (4.95s) and workflow policy (1.02s)
all passed. Both the Git and run changes were included. Fresh real Incus and
Windows streamed-run acceptance still require their ordinary environment checks.


PR #666 head `7ed40fe53850747ebd06231c09f9df9fc4a87959` passed test,
quality, Ubuntu installer and real Incus workflows. Incus run 34900137450,
job 104163854329 passed binary stdin, real PTY editing/resize, exit 17,
terminal restoration, cancellation cleanup and retained Workspace checks through
the product run route. Existing snapshot/copy/transfer checks also passed within
that fixture's scope. This establishes real Incus streamed-run acceptance.

Windows run 34900137466 failed at public reclaim (job 104163854107, step 19):
Linux reclamation completed, Windows stop was requested, but `compact_attached`
refused an attached VHD. Compaction and resume were not attempted; no native error
was recorded. The notification step was skipped. This is an unresolved Windows
reclaim failure, not streamed-run acceptance or evidence of a repaired worker.
Do not relax the attached-disk guard. Fresh human GUI, Windows stream use,
authenticated Git and large-repository performance remain unverified.

After #663 merged as main `9da3ec8f`, #666 was rebased to `18073ee7` with
an identical file tree to `7ed40fe5`. Only this evidence and status summary were
then updated; the prior source-bound results are preserved, not relabeled as CI
success for a new head.


<a id="cache-generation-foundation"></a>
## Cache generation foundation

Implementation `2a0e94990710bd3db9143f97d8fa5c4934b6a664` (v0.69) adds atomic source selection and
detached `build-cache` provider volumes. Go 1.26.8 Core/state/resource/architecture/
Incus regressions and three targeted race repetitions passed. Source verification
found 1,474 byte-identical files between the final tested copy and this commit.
Documentation consistency and 18 checker regressions passed.

Production changes passed the other packages in standard local test CI (Go
1.27.1, shuffle 615), but `TestLoginBootstrapPTYDoesNotStartHostSetup` failed
waiting for the Bash input prompt (6.64 s). Its earlier failures remain unresolved.
The later CI stages were skipped in that execution; separately, `go vet`, client
syntax, 32 notification-client tests and two packaging tests passed. These split
results do not turn the full CI run into a pass. The new opt-in native cache
fixture was added after that full run and executed separately.

Real Incus 6.0.5/Btrfs provider acceptance used a dedicated 1 GiB pool and random
8 MiB fixture data. Two independent copies took 3.837846024 s. `btrfs filesystem
du --raw -s` reported total/shared 8,388,608 bytes and exclusive 0 for each of the
source and two copies, before mutation. Contents, independent mutation, current
source deletion refusal, copies surviving reset/source deletion, native snapshot
reference refusal and exact fixture cleanup passed (16.45 s total). This measures
small synthetic data extents, not total pool allocation or giant-repository speed.

The first native attempt failed before publication because the test process
could not see the daemon's separate storage mount namespace. Pool/project
`haco-cache-15c4cf3cbcded3c0` and catalog
`/var/lib/haco-cache-generation-2844418008/state.json` retain a creating owned
source at generation zero; no forced catalog edit or cleanup bypass was used.
The successful attempt ran the same compiled fixture in the existing daemon
mount namespace, without changing isolation/authorization settings, and removed
its own pool/project `haco-cache-969c95ea6a2e3bc9`. It does not clean or resolve the
first attempt's retained source. This privileged fixture seeds and observes only
its own volumes; it is not ordinary-Env collection or installed client acceptance.

Host-selected paths, compatibility enrollment, multiple Env attachments,
automatic stopped collection, history/clear operations and actual large-repository
measurement remain open in [the cache contract](../design/cache-generations.md).

Parent Packer PR #643 (`80a687d0`) subsequently passed test 34778540239, Ubuntu
34778540205 and Incus 34778540180. Windows 34778540191/job 103781180868 passed
steps 13–20, including tunnel exit 0, but notification step 21 failed at
`stage=activation, reason=unavailable`; native/child exit/duration were unrecorded.
Fresh notification answers and actual Packer completion remain unverified.



## Main cache foundation integration

The main candidate reuses `2a0e9499`, `c4b7af50`, `094cc930`, `3a6e2bbc` and `2b3a5b56` over interactive-run source `7ed40fe5`. It preserves split lifecycle ownership and exact temporary-run identities, without adding old-version migration. Initial focused tests caught a missing generation-validation call during normalization and an imported old-schema acceptance fixture. The validation was restored and the fixture now tests preservation of current owned resources. All corruption-refusal cases then passed.

Final focused tests (12.86s), uncapped changed-code lint (10.84s), maintained local tests (22.15s), related race (9.78s), CLI E2E (4.01s), docs/regressions (6.96s) and workflow policy (1.34s) passed with Go 1.27.1. Earlier lint found read-response closes, fixture writes and boolean simplifications; fixed before these results. Historical provider measurements above are not relabeled as new native acceptance. The old ambiguous fixture pool remains untouched. Public configuration, stopped-Env publication, history/clear and added-data snapshot/copy/transfer remain incomplete, so production enrollment is disabled.
Integrating main `ef443132` as `a0352044` initially failed the full local entry (52.96s): the automatic merge duplicated three `run` help catalog keys, preventing compilation and the milestone blackbox build. Later checks were not run in that attempt. Removing the identical duplicate entries fixed the build; the corrected combined source passed full local tests (57.89s), CLI E2E (8.06s) and docs/regressions (9.86s). The earlier Windows `compact_attached` failure remains unexplained.


After integrating main `5e89597a` as `3d8c2877`, the cache foundation passed full local tests (13.62s), CLI E2E (3.27s) and docs/regressions (4.86s). Earlier head `22b119d8` passed all five workflows, including Windows34917359766. Public collection is a separate follow-up; this foundation does not enable enrollment.


<a id="ordinary-cache-collection"></a>
## Ordinary cache collection candidate

The public follow-up to #668 adds `haco cache settings/configure/status/collect`,
Host configuration persistence and stopped, creation-bound generation publication.
Full local tests (10.17s), focused tests (4.53s), changed-code lint (3.11s), related
race (6.02s), CLI E2E (2.73s), docs/regressions (4.82s) and workflow policy (0.93s)
passed on the verified source. An earlier full attempt stopped at 16 unchecked
CLI writer results in lint, after focused tests passed; output errors now return
failure. Subsequent presentation-only changes are checked separately.

Real Incus on WSL `hacocoon-second` passed the ordinary data-placement/collection
fixture in 15.35s (command 18.12s), using existing Btrfs pool `haco-local-default`
and cached Ubuntu 26.04 fingerprint
`b36d486c9412aee50d36c8875437070014bebd94d2207e0f703cd1b235c63033`.
Two cache directories were written inside an ordinary Env, stopped and collected.
After producer deletion, another Base name using the same cached image received
independent copies with the expected bytes; child cleanup, selection reset,
exact source cleanup and retained Workspace bytes passed. Native drift/ref-only
resume refusal and client-triggered resume also passed. The fixture used no
permission relaxation, proxy override, Host management socket in the guest or
manual catalog repair. A first PowerShell launch failed in the harness parser
before WSL/test execution; correcting its command text was the only launch change.

This proves the named rootfs-area functional slice, not different image contents,
installed CLI delivery, human Windows operations or giant-repository performance.
Existing-Env enrollment, history/clear/recovery commands and added-data transfer
remain incomplete. The old ambiguous pool `haco-cache-15c4cf3cbcded3c0` was untouched.



The final named-area presentation (including compatibility/shared scope and cleanup-required) passed focused CLI/UI/controller tests (2.61s) and docs/regressions (4.74s). This presentation change does not alter the native collection implementation exercised above.

<a id="main-packer-builds"></a>
## Packer HCL2 builds on main

The candidate reuses `1103505b` on main `9da3ec8f`: actual guest-local Packer, HCL2 context and external scripts, optional composition, canonical Base lifecycle and private failed-stage output. Focused tests (31.91s), changed-code lint (38.45s), maintained local tests (45.78s), related race (12.70s), CLI E2E (11.78s), docs/regressions (8.50s) and workflow policy (1.88s) passed with Go 1.27.1. Initial lint found unchecked read-handle closes, one error string and a switch simplification; corrected before these passes. Initial patch application targeted help catalogs absent from main and was refused without changing files; current main metadata was adapted instead.

These results do not execute Packer or establish installed acceptance. The source candidate's earlier whole-suite PTY timeout and Ubuntu dependency downloads rejected by the installed proxy with HTTP 403 remain unresolved historical failures, not a successful build. Full guest download/fmt/init/validate/build, Base publication/reuse, arm64, custom plugin failures and Windows entry remain unverified. No test-only policy grant or Host-side HCL execution is introduced. The existing simple JSON shell definition remains a current feature, not an old-version migration requirement.


After integrating #666 at `629f33ed` as `068c8106`, the canonical checkpoint tool advanced this candidate to v0.62 (Packer HCL2 Base builds). Combined local tests (34.93s), CLI E2E (5.34s) and docs/regressions (6.95s) passed. Both Packer and interactive run are included; this is not a release or full M4 acceptance.
Integrating main `ef443132` as `a0352044` initially failed the full local entry (52.96s): the automatic merge duplicated three `run` help catalog keys, preventing compilation and the milestone blackbox build. Later checks were not run in that attempt. Removing the identical duplicate entries fixed the build; the corrected combined source passed full local tests (57.89s), CLI E2E (8.06s) and docs/regressions (9.86s). The earlier Windows `compact_attached` failure remains unexplained.


At #667 head `61aeff8f`, Windows 34916801756 / job104216088784 passed installation, HTTPS, Windows interop, Base creation and initial strict SSH/desktop alias, then failed parallel cold reconnect after fixture WSL termination: exit255, ssh_progress=stream_denied. Reclamation and notification steps were skipped. The root cause remains unresolved; no actual Packer build is established by this run. Other four workflows passed.


After integrating main `5e89597a` as `075fc746`, Packer and current detailed help passed full local tests (13.24s), CLI E2E (3.18s) and docs/regressions (4.79s). Final formatting changes only whitespace in the two help files. The parallel cold SSH refusal at `61aeff8f` remains unresolved; new CI cannot retroactively establish its cause.

After integrating current main `5e89597a`, foundation `929346bf` and Packer `6ad776ad` as `25923518`, the v0.63 candidate passed maintained local tests (13.84s), CLI E2E (3.22s) and docs/regressions (4.93s). This combined check covers the public cache help and checkpoint; the native collection scope and remaining gaps above are unchanged.

## Named cache history and clearing

The follow-up to #669 passed focused tests (15.58s), changed-code lint (4.08s), full local tests (14.83s), related race (6.28s), CLI E2E (3.66s), docs/regressions (5.57s) and workflow policy (4.23s). The first focused run failed because a synthetic string reader was mistaken for a real nonterminal input in the test; using an actual pipe fixed the fixture without changing product confirmation.

The new real Incus maintenance attempt FAILED: `TestRealIncusEnvironmentDataPlacementE2E`, command251.40s/test243.38s, Env `data-e2e-c78cf57de85ce050`, catalog `/var/lib/haco-data-placement-529644104/state.json`. The four-minute context expired during existing resume/access steps before collection or the new history/clear assertions, reporting `signal: killed`. This is not a maintenance pass or a SKIP. Earlier ordinary collection success remains scoped to its own source; native clearing still needs acceptance. Cleanup outcome is being checked from the owned catalog.

Read-only follow-up confirmed no Environments, leases or persistent resources remain in that exact failed-fixture catalog, and the native name query returned no instance. Only two empty generation entries remain. The original timeout is unresolved.

## Named cache completion recovery

The follow-up to #670 passed focused lifecycle/cache/OCI/CLI/controller tests19.10s, changed-code lint15.71s, full local39.78s, race14.61s, CLI7.25s, docs/regressions10.41s and workflow2.00s. A real catalog with staged provider failure proves completed copies recover without recopy, original source pins are released only after verification, reset candidates remain retained, and unknown/provider-refused/wrong-owner cases remain blocked. Common OCI recovery now passes the exact owned reference. This is component evidence, not new native recovery or Windows acceptance. The earlier native maintenance timeout remains unresolved.

Final recovery usage text and bilingual help passed CLI/UI/controller tests9.40s and docs/regressions12.72s; ownership/recovery code is unchanged from the full validation above.

At recovery head3ac51f91, quality34924782288 failed QF1003 in cache_maintenance.go:41; test34924782199, Ubuntu34924782166 and Incus34924782231 passed. The dispatch is now a tagged switch with unchanged behavior. This failure remains distinct from local changed-patch lint success.

The dispatch correction passed focused control/cache/workspace tests11.23s, uncapped lint against current main18.59s and documentation/regressions8.48s. Reparenting onto main4cd0c7dc (the exact tree of tested parent1ee2962b) changed no files.

## Main client forwarding integration

The candidate reuses640c66ff/4b0b5baa/70037223/44a30b1a/bc8b915b/297b3095 over cache recovery3ac51f91. The initial focused build failed because the current SSH stream already owned readiness/completion types; reusing44a30b1a shared byte-stream handling removed the duplication. The next focused run passed6.54s but lint failed73 unchecked results and8 static findings in imported code. These were fixed without weakening validation. The corrected immutable source passed focused3.76s, lint4.64s, full local11.26s, race7.74s, CLI2.84s, docs5.24s and workflow0.98s (Go1.27.1). Windows amd64 compiled components then passed streamio0.27s, clientforward1.09s, wsllaunch0.34s and native CLI0.69s, covering real pipes/TCP/half-close, parent loss, exact target refusal and local help. These are component results, not new installed Windows/Incus acceptance.

The PowerShell5.1 installer component command was refused by the machine script-execution policy before tests ran. No execution-policy or isolation setting was relaxed. Existing source297b3095 has local foreground-interruption regression evidence, while its installed Ctrl+C rerun remained pending; preserve that boundary. The first checkpoint invocation could not interpret Windows worktree administration paths from Linux and made no change. The existing Windows lock helper around the same WSL updater succeeded, without bypassing the lock.

Native PowerShell7 installer components passed with both installed binary types: install/reinstall, missing/mismatched ownership, checksum, pinned worker and junction refusal. PowerShell5.1 remains not run under its script policy; neither policy was changed. The existing #634 evidence records parent #632 Ubuntu34763684021/job103740815424 passing eight concurrent2MiB installed Linux tunnel exchanges after the readiness fix. Reused #642 source preserves the later Windows Ctrl+C gap; neither result is relabeled as acceptance of this new main candidate.

After the v0.64 checkpoint and installation guidance update, presentation/build-identity tests2.81s, architecture/checksum/corrupt-archive packaging0.54s, docs5.72s and workflow1.11s passed. Windows CI now expects the actual ten build entries and requires the tunnel artifact; published amd64/arm64 targets stay intact.

The candidate rebased over corrected recovery c35f6610 as c682faaa passed focused4.22s, main-diff lint16.99s, full local104.53s, race36.42s, CLI18.56s, docs19.60s and workflow2.66s. Recovery #671 then passed all five workflows and merged as main df22a1a5. Its file tree exactly matches c35f6610, so reparenting the forwarding commit as11823e02 changed no files.

DNS selection candidate: focused72.65s passed initially. With snapshot creation and import mode-preservation regressions, focused30.07s, changed-code lint23.44s, full local45.48s, race14.76s, CLI35.53s, docs/regressions13.46s and workflow5.91s passed. The final status presentation and additional native-adapter regression are checked separately. No three-mode real Incus acceptance is established by these component results.

Final-candidate focused validation initially failed27.69s because the exact English status fixture did not include the newly visible resolver row. Updating that expected display retains the existing localized data and escaping checks; subsequent checks in that attempt were not run.

Real Incus on hacocoon-second passed TestRealIncusDNSModesE2E at ab98e0cc (command84.63s/test75.10s): host24.12s, backend13.09s, disabled37.89s. Each used a freshly owned Env, current product companion, canonical create/stop/start/delete, and loopback resolver/service inspection; backend also resolved example.com through the exact-owned tooling adapter. Catalogs: /var/lib/haco-dns-modes-905851288/state.json, -4084255079/state.json and -70566286/state.json. All owned Env cleanup passed. No Policy grants, guest management handles or alternate DNS fallback were added. This establishes native provisioning/resume and backend resolution, not end-to-end guest Policy queries, VPN/NRPT changes, Windows restart or GUI answers.

Correcting the expected status row passed the final current-main source: focused34.86s, full-PR lint52.03s, full local33.57s, race20.05s, CLI8.60s, docs13.53s and workflow2.47s. No product behavior was changed to satisfy the old output fixture.


## Named data snapshots and copy

Implementation26939f1b passed focused13.10s, main-diff lint11.93s, full local24.72s, race10.61s, CLI4.84s, docs7.51s and workflow1.36s. Catalog regressions cover changed source generations, complete child reservations, copied-data receipts before verification, source deletion exclusion and unknown-copy/cleanup retention. Initial focused attempts failed an obsolete unsupported expectation and the remaining saved-source lease refusal; focused-3 then passed17.32s. These failures were corrected, not skipped.

Real Incus on hacocoon-second, Btrfs poolhaco-local-default and cached Ubuntu imageb36d486c9412aee50d36c8875437070014bebd94d2207e0f703cd1b235c63033: data-native-rootfs-1 passed52.97s (test46.52s, build7.33s). Fixture saved-data-9fa8a00cd2833843, catalog/var/lib/haco-saved-data-2533260138/state.json, proved two rootfs data areas plus a two-repository Workspace: running capture and resource-aware resume, stopped independent copy, uncollected bytes, fresh ownership, independent edits, source deletion, copied resume and canonical owned cleanup. No source-generation publication or Policy grant was added. Performance and repository-subdirectory placement are not established.

Previous data-native-1 failed test compilation after a successful build8.99s; data-native-2 failed14.54s (test7.17s) before capture because this Incus lacks file_storage_volume, required for repository-subdirectory placement. Fixture saved-data-9cb00c708dc5ace0 and durable catalog/var/lib/haco-saved-data-1321951232/state.json remain recorded; created Workspace fixture volumes are retained for exact cleanup. The rootfs test is a separate supported placement, not a bypass or a pass for repository placement.

DNS candidate #674 head a44c0cc5 passed four Linux workflows; Windows34930836374/job104258506823 passed installation, HTTPS, SSH and Linux reclaim, but public reclaim failedcompact_attached before Windows compaction (open attempts1, compaction not attempted, not resumed). Notifications were skipped. Git/GUI #672 head7454efab also passed four Linux workflows; Windows34931359363/job104260056333 failed parallel cold SSH reconnect withstream_denied/exit255. Reclaim and notification were skipped. Neither head was merged; these failures remain distinct from earlier successes and notification failures. DNS is integrated into this candidate as98c0ddd2; only the data-vs-DNS validation overlap required conflict resolution.

After DNS integration and v0.66 generation, the combined candidate passed focused22.16s, complete main-diff lint17.84s, full local29.81s, race11.90s, CLI4.77s, docs8.15s and workflow1.50s. Native named-data evidence above covers unchanged lifecycle/provider code; no new Windows pass is claimed.

## Portable named Environment data

The v0.67 candidate on #675 head fa6c1312 implements named data in the existing export/import envelope and canonical Environment lifecycle. The immutable final source passed focused16.47s, complete main-diff lint15.45s, full local27.39s, race12.82s, CLI4.59s, docs/regressions8.38s and workflow1.48s. Tests cover full payloads, fresh local identities, import-only reservations, complete placement, unsupported-provider refusal before Workspace creation, and retained ownership after ambiguous cleanup. Initial full-1 failed because its temporary staging directory was not private; tightening the fixture to0700 fixed it. Full-2 passed focused12.92s but failed lint on two unchecked test-reader closes; both are now checked. Neither failure was skipped or fixed by weakening product permissions.

Native portability attempt portability-native-1 FAILED46.36s (build6.16s/test41.23s): owned snapshot volume export was unavailable before import. Fixture saved-data-f7c512b0ca4392e9, catalog /var/lib/haco-saved-data-2912432684/state.json, snapshot volume haco-snapshot-342b7e71e895c7a826c2d337efe90156. The Env and saved capture used canonical cleanup; retained Workspace fixture data requires exact cleanup and is not blanket-deleted.

Follow-up identified Incus client/server6.0.5 on hacocoon-second. It lacks the export --force option used by the existing exporter and file_storage_volume required by repository-subdirectory placement. The current product requires Incus7.0 LTS (>=7.0.1,<7.1). Previous DNS and rootfs snapshot/copy successes above are limited observations on6.0.5, not supported-baseline acceptance. No legacy-provider workaround was added. Native portability, repository placement and a fresh installation on the supported baseline remain pending; performance and large-repository measurements are deferred by the user. Existing Windows reclaim/parallel-SSH failures remain unresolved.


## Current data evacuation inventory

The schema16 inventory follow-up projects named data, selected generations and pending import/copy/cleanup receipts without modifying the catalog. Exact catalog links remain separate from native observations; absent historical producers do not become deletion candidates. The immutable candidate passed Linux evacuation regressions (78 tests,0.96s), maintained documentation/regressions34.39s and workflow policy4.39s. Windows discovery also passed but explicitly skipped29 Linux-only checks; the Linux run supplies those checks. A guard compares the supported current schema with the canonical Go store. This is read-only inventory coverage, not whole-installation backup, restore or supported-provider acceptance. Earlier native transfer and Windows failures remain unresolved.


## Repository data placement in saved Environments

Supported7.0.1 attempt saved-data-ec2813c23b2ada60 FAILED11.61s/test11.56s at snapshot planning with capability stale; catalog /var/lib/haco-saved-data-3393557504/state.json. The planner compared a data-only digest while ordinary repository placement also binds the leased Workspace storage. It now shares the existing runtime placement resolver with create/resume/import. A component regression accepts the complete repository binding and refuses data-only substitution.

The corrected immutable candidate based on f71ab282 passed focused16.88s, complete main-diff lint15.95s, full local27.33s, race11.29s, CLI4.21s, docs7.90s, workflow1.45s and native-test compilation1.61s. On newly installed Hacocoon-Roadmap-f68a8c6b (Ubuntu26.04.1, Incus7.0.1), placement-supported-native-1 PASSED55.61s/test55.58s. Fixture saved-data-054e30231401ddac, catalog /var/lib/haco-saved-data-2319641996/state.json, covers two Workspaces, rootfs compiler data and /workspace/two/node_modules, running snapshot/resume, stopped copy, portable export/import, uncollected content, fresh identities, independent edits, source deletion, imported restart and exact cleanup. The original failed fixture and its retained Workspace records remain separate.

The earlier rootfs-only supported transfer passed76.26s/test76.23s at f68a8c6b. Fresh normal Windows installation completed with doctor, DNS/HTTPS, Windows enrollment and notification registration; existing hacocoon-second was retained. Current-schema inventory also read the retained6.0.5 fixture catalog and native objects successfully in2.89s with authority=false.

Ordinary installed Packer sample builds FAILED at dependency installation:10.39s, then6.56s with private failure output. Ubuntu HTTP downloads returned403 from the normal proxy. The public config showed default=deny with zero rules; no test-specific allow was introduced. Actual Packer execution remains pending normal communication configuration. #676 head f68a8c6b Windows34935395589/job104272085316 passed installation, parallel cold SSH, actual VS Code editing, setup, preview and ordinary export/import, then FAILED the ordinary Windows tunnel with timeout/connection reset; reclamation and notification steps did not run. Earlier failures are not relabeled as resolved.
