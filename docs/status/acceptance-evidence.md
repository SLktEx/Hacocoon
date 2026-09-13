# Acceptance evidence and limits

[日本語](acceptance-evidence.ja.md) | English

Status: recorded acceptance evidence. These tests ran on the identified historical commits; this documentation refactor does not rerun or claim real-host acceptance. See [implementation status](../IMPLEMENTATION_STATUS.md) for current availability.

Read each pass, failure and skip within its fixture and candidate. A narrower or later pass does not establish the cause of a different failure. Maintain evidence that changes support decisions and unresolved limits here, rather than appending daily run logs.

## Temporary-run ownership and streams

`9f4cf5105f01c5da7dfe40e080651979789c799b` (PR #590) passed test
[34729490922](https://github.com/SLktEx/Hacocoon/actions/runs/34729490922), Ubuntu
[34729490923](https://github.com/SLktEx/Hacocoon/actions/runs/34729490923), Incus
[34729490810](https://github.com/SLktEx/Hacocoon/actions/runs/34729490810) and Windows
[34729490822](https://github.com/SLktEx/Hacocoon/actions/runs/34729490822).
Incus Btrfs job 103649531126 used 7.0.1 and passed captured temporary execution,
exit 17, retained Workspace data, cancellation cleanup and detached Store cleanup.
Private registry was skipped. This establishes the ownership change on the
disposable native fixture, before the later stdin/TTY implementation.

The following stream candidate initially failed its early-exit integration race:
closing a Unix socket with unread input could reset the peer and discard the final
receipt. Explicit input-stop/EOF drainage fixes the failure; three race runs of
the control/control API/CLI regressions pass, including thirty early exits with
unread input. Native binary pipe and Linux PTY acceptance and ordinary Windows
ConPTY edit/resize/exit/restoration are exercised by the maintained gates; exact
native results follow and are not inferred from component success.

Full local test CI on the stream candidate initially failed the existing
`TestUDPIdleCountsBothDirections` (200 ms idle deadline). Its isolated twenty-run
repetition and a subsequent complete `ci-local.sh test` passed. The original
failure's cause remains unconfirmed; no relay behavior or timeout was relaxed.
The final six-package process/temporary race check, documentation check,
workflow-policy suite and native fixture syntax checks passed separately.

At `b31698148db03915504c476e52e617fe70ecb527` (PR #591), test 34732860619,
Ubuntu 34732860608 and Incus 34732860628 passed. Btrfs job 103658786664 records
Incus **7.0.1**, the 2 MiB binary pipe, real PTY edit/resize, exit 17, terminal
restoration and cleanup **PASS**. Existing captured cancellation and retained
Workspace checks also passed. Private registry was skipped.

Windows run 34732860626 failed its new TTY input assertion. The guest read an
empty line before the driver's intended text; resize, exit 17 and terminal/catalog
restoration were observed. The driver sent CRLF to start the command, leaving a
second newline for the guest. It now sends one CR, matching Enter. Driver protocol
regressions reject empty input and changed native ownership. The initial failed
run remains part of the evidence.

The corrected `9767fd93ad16f9ee20ea9b2eb1394c47ca68fbbb` passed test
34734033900, Ubuntu 34734033821, Incus 34734033816 and
[Windows 34734033825](https://github.com/SLktEx/Hacocoon/actions/runs/34734033825).
Windows job 103662066493 confirms ordinary Windows/WSL/Host ConPTY input editing
(`RUN-INPUT:abD`), resize to 43x132, exit 17, restored terminal/catalog and unchanged
provider resource inventory. This rerun resolves the initial input failure within
that fixture. Japanese Windows, fresh GUI/toast decisions and VPN/NRPT remain
outside this acceptance.

## Local GUI candidate

`e7ba798728dcbe48a5179845673a333f8ff8968f` (PR #588) passed test
34727370959, Ubuntu installer 34727370966, Incus 7 34727370817 and Windows
installer 34727370876. Windows job 103643786611 reports
`VS CODE LOCAL APPROVAL WEBVIEW / REAL RENDERER HANDSHAKE / INSTALLED CONTROLLER STALE REFUSAL: PASS`.
This proves the packaged local panel, its actual renderer readiness and stale
request refusal through the installed controller. Ordinary SSH, interop, installer,
restart and reclaim steps also passed. Human toast clicks/fresh GUI decisions and
VPN/NRPT remain explicit **SKIP**. Japanese Windows and the existing local WSL
Interop failure remain unverified/unresolved respectively. This is development
branch evidence, not distribution or acceptance of later run-ownership changes.

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

## Daily entry and setup diagnostics

Status: **implemented; dedicated WSL/Linux daily acceptance passed**.

Host setup streams bounded fixed stage/state/reason and request IDs, records
structured journal diagnostics, rejects missing completion and preserves exclusion
after disconnect. Daily Env progress stays on stderr; stdout JSON remains valid.
Help and paired instructions cover create/open/work/stop/resume and distinguish
Env removal from retained data. Noninteractive confirmations never wait for input.
First SSH failure guidance also routes to read-only approval and Policy inspection
without claiming that packages or approval caused the failure.

Installed `6cf9295` in the dedicated `hacocoon-v2` Ubuntu 26.04 / Incus 6.0.5
checkout on 2026-09-12, using locally built binaries through the common installer.
The local bundle is development acceptance, not signed-release provenance.

| Actual check | Result |
|---|---|
| Common installer, setup, Host doctor | Exit 0; all six doctor checks passed, including Incus-owned Btrfs identity/mount policy and trusted-host DNS/HTTPS. |
| Ordinary user create/open/work | Default Base, external Workspace, `--no-oci`; SSH edited a source file, built an uppercase output and compared expected content. |
| Stop/start/open | Stopped status observed; Workspace output and a rootfs marker survived; pinned Linux SSH reconnected. |
| Duplicate create | Refused with `already_exists`; existing Env remained available. |
| Terminal blank selection | Canceled before desktop/connection changes. |
| Env delete | Target/data effects preceded canonical deletion; Env absence and retained external Workspace file confirmed. |
| Setup failure | Synthetic customization exit 29 reported the failed stage/reason/request ID; no false success or synthetic private output in CLI/journal. Cleared the fixture recipe through the normal API. |
| Setup interruption | Observer exited without completion; another setup was refused busy until the original operation completed in the journal. |

First SSH preparation failed with default-deny Policy and absent sshd. An explicit
Policy update limited to the current Env generation and Ubuntu package endpoints
allowed ordinary SSH preparation to complete. This establishes that tested path;
a generic SSH error still does not prove the same cause for another installation.
After package installation, only the four fixture rules were removed through the
configuration API, restoring the original default deny. An actual Linux terminal
SSH session still verified the retained Workspace in that state.

The dedicated development network namespace, veth and narrow outer NAT isolate
this Incus/controller from other WSL bridges. Incus and controller share their
namespace's sysfs/Btrfs mount view; this is local acceptance configuration, not a
product default. The WSL kernel has AppArmor disabled; no kernel or isolation
checks were changed. This does not establish AppArmor confinement acceptance.
Windows IDE/default-entry installation, Windows SSH, private Git/registry,
OCI retention and a cold distribution restart were not exercised by this run.

Repository validation: standard local test/vet/notification and full race targets
passed, as did fixture CLI E2E and 22 Python interop tests. Windows installer
component fixtures passed on Windows with read-only transport pinned to the
selected WSL; installer mutation paths were mocked. Linux PowerShell cannot run
that Windows-only fixture because SystemDirectory is empty. These checks do not
substitute for the actual provider/client results above. The test workflow checks
PRs targeting `dev/v2`.

See [daily workflow](../reference/daily-workflow.md). That dev/v2 acceptance predates the separate Workspace and TCP/UDP implementation on dev/2.x.

## M0/M1 integration candidate, PR #583

At `37c3e679`, test run 34713814211 and Ubuntu installer run 34713814185
failed because their assertions still required the former horizontal env help.
The actual Ubuntu installation completed. `195172f4` updated those assertions
to the hierarchical help contract, preserving exit/output checks; full test run
34714239387 and packaged Ubuntu run 34714239415 succeeded. These precede the
shared Incus LTS change and do not accept that later code.

Windows run 34713814252 (job 103607232075) failed at the added pipe-based
keypress test, before installed Windows acceptance. Local pipe success did not
establish headless-console behavior. The replacement uses the shipped BAT in
ConPTY, waits for its prompt, sends a key and checks exit 37. Local native
ConPTY and 0/1/37/3010/prerequisite components passed; the replacement's CI and
Explorer double-click remain separate gates. A local dependency-import attempt
could not read the isolated package under sandbox permissions; the same test
with that package's installation permissions ran successfully. This was not an
installed-product failure or a permission change in the product.

The LTS command-boundary regressions reject additional/missing/duplicate signing
keys, wrong package sources/series, failed dependencies and a newer installed
series. A malformed multiline version initially passed shell validation; the
new regression detected it and the corrected validator rejects it. Six helper
tests, seven host preparation tests, installer packaging and Go HostDiagnostics
tests passed. No installed WSL packages or retained user resources were changed.
Fresh 7.0 product installation is still pending.

At `cc18a60b`, full test CI 34715459033 passed. Incus run 34715459013
passed standalone and Core/egress/lifecycle jobs, but failed owned-Btrfs Store
maintenance: the real CLI correctly rejected piped deletion confirmation with
exit 2, whereas the old test expected a terminal decline. The replacement uses
a private Linux PTY for both decline and approval; no product confirmation
requirement or cleanup assertion is weakened. Native rerun remains required.

Ubuntu run 34715458982 installed and verified Incus 7.0.1, then failed boot-guard
adoption because the daemon path differed from Ubuntu's package. `2c9faa07`
recognizes the exact Zabbly daemon path while retaining root, namespace and
systemd MainPID checks; 20 regressions passed. Unknown active daemons still fail
closed. This fix also needs installed acceptance. Windows run 34715459045 passed
the ConPTY component, then failed at the same boot-guard path after verifying
7.0.1. The driver ignored BAT's explicit failure and waited another 28 minutes
for its timeout. It now fails immediately on that final result and closes its
owned terminal; a regression prevents a second BAT from repairing acceptance.
Later Windows SSH, reclamation and notification stages were skipped.

At `96bbbdf8`, full test run 34717575075 passed and Ubuntu run 34717575034
successfully installed the packaged product with the corrected boot guard.
Its next assertion failed: it invoked privileged legacy `hacoq doctor` as the
ordinary user. Incus 7 returns failure when that user lacks daemon authority;
root's diagnostic succeeded. The gate now uses product `haco doctor`, which
exercises the installed controller and its intended user group. No Incus-admin
membership or permission relaxation is added. The remaining journey/security
steps were skipped and require another run.

Windows `96bbbdf8` run 34717575063 passed cached package installation, distro
terminate/restart/reinstall, controller HTTPS/direct-egress refusal, native
Windows OpenSSH with key pinning and stopped resume, and VS Code 1.136.1 Remote-SSH
file read/write plus terminal execution. Project setup save/replay/failure/update
also passed. The desktop job still failed independent approval-review, preview
setup and Environment export probes. Approval review supplied piped answers to
the terminal-only CLI; its fixture now uses a private PTY with separate JSON and
diagnostics (six regressions passed). Preview failed after that unresolved review;
the relationship remains unconfirmed until rerun. Export failed at its export
phase with insufficient categorized evidence; fixed allowlisted diagnostics are
added without raw output. No transfer success or cleanup is inferred.

The same run passed Linux Btrfs/ext4 trim stages, while public Windows reclamation
failed because the earlier transfer had not produced its retention manifest.
That is a missing prerequisite, not a VHDX compaction attempt or pass. The final
native notification route was skipped. Existing successful narrower results do
not erase these remaining failures.

Incus run 34717575098 for `96bbbdf8` subsequently passed every enabled job:
standalone runtime, owned Btrfs pool (Base/snapshot/CoW/import and reviewed Store
maintenance through a real PTY), and Core egress/lifecycle. Private-registry
acceptance was skipped for its existing prerequisite. This Linux success does
not resolve the separate Windows transfer failure above.

Candidate `5fe184a6` passed test CI 34719977795 and packaged Ubuntu 34719977797,
including ordinary-user doctor, installed journey and network/spoofing guards.
Windows 34719977824 passed its earlier installation/SSH/VS Code stages and now
also passed pending-review saved-ask/current-deny/one-shot-allow/reask/cleanup and
Edge preview/reuse/refusal. This resolves those two preceding failed probes; it
does not establish GUI-only approval. Export still failed (`phase=export`, fixed
evidence `volume-export,unavailable`). Linux trim passed; Windows reclamation
again lacked the transfer manifest and native notification was skipped. The new
classification localizes the remaining failure without exposing native output.

Upstream Incus 7.0.1's `cmdStorageVolumeExport.run` rejects an existing target
without `--force`; the controller-owned `/proc/<pid>/fd/<fd>` output intentionally
exists. The adapter now supplies that flag only for its live anonymous descriptor,
with ownership/unlinked/private-file regressions. Public destination overwrite
refusal remains unchanged. Native Windows transfer must rerun to establish the
fix; it is not inferred from the source diagnosis or component test alone.

Candidate `3cac2e95` passed test CI 34721760620 and packaged Ubuntu 34721760552.
Incus run 34721760571 failed the owned-Btrfs aggregate export and its cleanup
step. Its package log identifies **6.0.5-8**, not 7.x: `ci-incus-core.sh` still
installed the distro package and accepted `>= 6.0.5`, unlike the standalone
helper. The 7.x export flag exposed this remaining setup drift. Core and
standalone jobs passed; private registry and subsequent Btrfs probes were skipped.
Earlier all-enabled-job successes at `96bbbdf8` and `5fe184a6` therefore establish
Core/Btrfs acceptance on 6.0.5 only. Packaged Ubuntu/Windows 7.0.1 evidence remains
separate. Both maintained CI setup entry points now use the shared signed LTS
installer and bounded server-version gate, with a routing regression. Native
Core/Btrfs 7.x acceptance requires a new run; the failed cleanup remains recorded.

Windows run 34721760573 then passed every enabled step at `3cac2e95` (tested
merge `d3fb94a6e872bb44fd1d67d08ee842a1882f4843`, Incus 7.0.1). The real packaged
path now passes bundle hash/immutability, export/source deletion/import, native
Windows SSH and recreation from retained work. This resolves the earlier export
failure and missing-manifest prerequisite. Public reclaim completed Linux trim,
WSL stop, VHDX compaction from 7,931,428,864 to 4,033,871,872 allocated bytes,
resume, Host sentinel and detached Workspace/OCI/snapshot restore checks.
Native notification registration, stale/malformed/foreign ownership refusal,
controller subscription and owned listener cleanup passed. A human toast click
and fresh GUI decision remain explicitly **SKIP**, as does VPN/NRPT. Existing
installed SSH, VS Code, review and preview scopes also passed. This is one
disposable Windows/WSL configuration, not giant-repository measurement, full
Japanese UI acceptance or distribution.

At `655f03ce`, test CI 34723210857 passed. Incus run 34723210668 now records
**7.0.1** for Core/Btrfs. Standalone and Core jobs passed. Btrfs aggregate
export/import, OCI writable data, snapshot/restore/copy, retention and native
child refusal all passed, resolving the earlier export failure on this substrate.
`TestRealIncusSourceDeletionE2E` then failed its snapshot `show` fixture: Incus 7
requires separate volume and snapshot arguments. The fixture now uses that form,
as its existing create/delete calls already do. Product deletion checks are
unchanged. The storage cleanup step failed because that test stopped before its
owned cleanup; the overall Incus cleanup step passed. Later Btrfs probes and
private registry were skipped. The complete Btrfs job requires another run.

At `28ca8ebf`, test 34723923596 and Ubuntu 34723923612 passed. Incus run
34723923619 passed Core/standalone and Btrfs aggregate/source deletion, native
volume import and definition-driven Base build. The later persistent-copy
fixture had the same combined snapshot argument and failed; it is now corrected
to separate volume/snapshot arguments. Both workflow cleanup steps passed; the
failed test's explicitly retained recovery fixture remains recorded. Store
maintenance and private registry were skipped. No complete Btrfs job pass is claimed.

The automatic normal Windows/WSL entry language change passes product,
control API, selector and architecture tests. The user-path assertion's 12 tests
also pass, including refusal of absent, echoed, duplicate or mismatched language
markers. Direct local Windows PowerShell returned `en`. The separate opt-in
native query from existing `hacocoon-second` failed with `exec format error`;
read-only inspection found no `/proc/sys/fs/binfmt_misc/WSLInterop` registration.
That existing installation was not repaired or reconfigured. This failure stays
distinct from the passing fallback tests. Fresh packaged Windows CI now checks
the actual Host session value against the Windows user UI setting after ordinary
entry/restart/reinstall, without injecting a Hacocoon override. Its result is pending.

### Combined candidate on Incus 7 and packaged Windows

At `0c79f8209eec42b597cc811a9114e0351d8226d7` (PR #583), test
[34724986411](https://github.com/SLktEx/Hacocoon/actions/runs/34724986411), Ubuntu
[34724986358](https://github.com/SLktEx/Hacocoon/actions/runs/34724986358), Incus
[34724986357](https://github.com/SLktEx/Hacocoon/actions/runs/34724986357), and Windows
[34724986361](https://github.com/SLktEx/Hacocoon/actions/runs/34724986361) passed.
All enabled standalone/Core/Btrfs jobs passed on verified Incus **7.0.1**, including
Base build, native import, source deletion, snapshot/copy, persistent CoW and Store
maintenance/cleanup. Private registry was skipped. This resolves the two snapshot
fixture failures above on this candidate; it does not erase their retained-failure history.

Windows install/restart/reinstall matched the actual Windows UI setting to Host
`HACO_UI_LANGUAGE=en` without an injected override. Native interop, ordinary pinned
SSH/reuse/resume, actual VS Code editor/terminal, pending CLI decisions, preview,
export/delete/import and retained-work recreation passed. Public reclaim measured
VHDX allocation **7,864,320,000 → 3,974,103,040 bytes**, then resumed and verified Host
sentinel, retained Workspace/OCI and snapshot restore. Native notification ownership,
stale/malformed refusal and listener cleanup passed. Human toast clicks/fresh GUI
decisions and VPN/NRPT were explicitly skipped. Japanese Windows native acceptance
and the local existing WSLInterop failure remain unresolved. These results predate
the new GUI session and must not be relabeled as its acceptance or as distribution.
