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


<a id="main-gui-approval"></a>
## Main GUI approval integration candidate

The VS Code portion reuses #588 (`e7ba7987`) on main `7e876bc1`.
Focused desktop/common review/control/product tests (2.67s) and changed-code
golangci-lint 2.13.2 (4.49s) passed. The first full local run passed Go and all
32 renderer/client tests, then failed both VSIX packaging tests because our
verification archive assigned epoch-zero timestamps to uncommitted new files.
The Windows checkout packaging tests passed; product packaging rules were not
weakened. Correctly preserving source timestamps fixed the verification copy.
The subsequent full local test entry (13.99s), related race (6.18s), CLI E2E
(3.49s) and docs/regressions (4.51s) passed. This is repository/component evidence,
not fresh installed Webview or human answers. Historical #588 acceptance remains
scoped to its own source and cannot establish this main integration.

The combined candidate `da064d83` adds Windows notification-contained choices
from #611 (`667ae5bf`) plus duplicate-refusal and cancellation diagnostics
(`7de0ad51`, `8eeac2b8`). On the verified source, focused tests (2.84s), pinned
changed-code lint (4.38s), the full local test entry (10.77s), related race
(6.64s), CLI E2E (2.93s), docs (4.59s) and workflow policy (1.02s) passed.
The first lint attempt found three unchecked test-stream closes and one error
capitalization; those were corrected before this successful run.

Windows amd64 test/GUI-adapter builds and Windows-target vet passed. Actual
Windows review components (4.27s), shared desktop review tests (0.30s) and the
isolated registration test (2.27s) passed. This includes the ordinary initial
owned-history clear, English/Japanese ToastGeneric selection XML accepted by
Windows history, removal, COM activation/refusal, cancellation/process reaping,
and exact registration ownership. The registration fixture cleaned only its
fresh keys/files; no installation, execution policy or pending user request was
changed. A first test harness launch stopped before tests because its PowerShell
path variable was missing; selecting the current executable resolved that
harness error without altering execution policy.

These results do not establish human button answers, visible layout or the
installed notification-to-controller journey. Earlier development candidates'
8-second notification clear timeouts remain unexplained failures; a successful
isolated current-component run does not erase them. Fresh installed VS Code
answers and authenticated Git also remain unverified.

PR #664 head `8c1cc435` passed repository, quality, Ubuntu and Incus workflows.
Windows run 34894187686 failed at installed native notification review (job
104143946090): stage=clear, reason=timeout, child exit=1, duration=8023 ms,
no native HRESULT. Installation, strict SSH and both reclamation stages passed
before it. The root cause remains unknown; component success does not erase it.
The follow-up adds bounded fixed progress observations without extending deadlines
or bypassing notification history. Fresh validation is recorded below.

The follow-up on main `119e3007` (`ffb31f2b`) passed focused checks 2.48s, lint
3.49s, full local tests 10.53s, race 6.15s, CLI E2E 2.91s, docs 4.83s and workflow
policy 1.04s. Windows test/GUI builds and vet passed; actual Windows review tests
(11.41s), desktop tests (0.42s) and isolated registration (2.38s) also passed.
Fixed progress parsing covers partial reads, unrelated/oversized output and child
failure; the ordinary initial clear/show/history/remove path was exercised again.
The installed CI clear timeout still needs a result from this updated candidate.

A preceding validation archive was captured while the merge commit was completing
and incorrectly retained a removed Seed fixture. Its full test run failed on that
fixture's source-guard mismatch. This is retained as an invalid-source validation
failure, not evidence for the merged candidate. Archive creation now pins one
commit and rejects a moving source before validation; a fresh exact archive
produced the results above. No product guard or timeout was relaxed.


At #664 head `38dc1ffe`, Windows run 34914309433 / job 104208480202 passed installation, strict SSH and public reclaim, then failed native review step 20. The new fixed report was `stage=activation, reason=unavailable`; native HRESULT, child duration and renderer progress were unobserved. After about ten seconds, the missing-request probe did not receive the expected no-longer-pending refusal. This is a separate unresolved activation failure, not proof of repair of earlier clear timeouts. Fresh human answers remain unverified.


The activation diagnostic follow-up records fixed COM initialize/register/create/dispatch HRESULTs and preserves read-only timeout/cancellation across COM and private peer shutdown. Focused regressions (1.22s), docs/regressions (10.37s), native Windows review (4.75s), desktop (0.44s), isolated registration (2.81s), Windows build/vet and PowerShell probe parsing passed. Final formatting only changes whitespace. This does not establish that the installed activation failure is fixed; its failing run remains above.


After integrating main `ef443132` as `effc7801`, the combined GUI candidate passed the full local test entry (72.69s), CLI E2E (6.79s), and docs/regressions (8.14s). Native installed activation and earlier clear failures remain unresolved pending the updated Windows run.


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


Integrating main `ef443132` as `a0352044` initially failed the full local entry (52.96s): the automatic merge duplicated three `run` help catalog keys, preventing compilation and the milestone blackbox build. Later checks were not run in that attempt. Removing the identical duplicate entries fixed the build; the corrected combined source passed full local tests (57.89s), CLI E2E (8.06s) and docs/regressions (9.86s). The earlier Windows `compact_attached` failure remains unexplained.

At #664 head `aef58798`, Windows34918511743/job104221234323 passed installation, strict SSH and Linux reclamation. Public reclamation Host re-entry failed at 02:08:10 UTC with `stage=notification_setup reason=failed`; the observer then waited until 02:37:51 and timed out. The public reclaim operation was not reached and notification step20 was skipped. Other four workflows passed. This differs from the earlier clear/COM activation failures. The follow-up reuses `5a6fb54c` classification and failed-entry detection, without claiming a root-cause fix.

The main notification-setup integration reuses 5a6fb54c over GUI aef58798 and main 5e89597a. Focused tests4.37s, notification Python regressions0.69s, changed-code lint30.66s, full local117.79s, race20.22s, CLI11.78s, docs17.46s and workflow2.88s passed. Initial integration testing failed at import of a future stream acceptance script absent on main; its unrelated test import was removed while retaining the existing native entry regression. Windows execution of the native observer tests passed6 tests0.555s. No new installed notification success is claimed.
