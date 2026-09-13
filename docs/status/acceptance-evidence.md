# Acceptance evidence and limits

[日本語](acceptance-evidence.ja.md) | English

Status: recorded acceptance evidence. These tests ran on the identified historical commits; this documentation refactor does not rerun or claim real-host acceptance. See [implementation status](../IMPLEMENTATION_STATUS.md) for current availability.

Read each pass, failure and skip within its fixture and candidate. A narrower or later pass does not establish the cause of a different failure. Maintain evidence that changes support decisions and unresolved limits here, rather than appending daily run logs.

<a id="installation"></a>

## Installation and Host

| Candidate / gate | Result and limits |
|---|---|
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
| `84062060e0ef465e73ec45b43b6ed785ce879d81` / [Windows job](https://github.com/SLktEx/Hacocoon/actions/runs/34740317809/job/103678816517) | Post-termination ordinary Host entry reported `Host setup is busy`; the harness then waited for its deadline. The harness now fails immediately on that product error and records phase timing. The underlying busy condition remains unresolved; fail-fast reporting is not a product fix or successful Windows acceptance. |

At the uncommitted #615 working candidate based on `7b4e2356d73a163b31e784a0a5b7400fed1a05cf`,
full Go test/vet, race, shipped-command fixture E2E, documentation and workflow policy
passed on the local Linux validation environment. Real Incus and packaged Windows/WSL
were not run there. Commit-bound hosted results must be recorded separately.
