# Acceptance evidence and limits

[日本語](acceptance-evidence.ja.md) | English

Status: recorded acceptance evidence. Except for explicitly marked local candidates, these tests ran on the identified historical commits; this documentation refactor does not rerun or claim real-host acceptance. See [implementation status](../IMPLEMENTATION_STATUS.md) for current availability.

Read each pass, failure and skip within its fixture and candidate. A narrower or later pass does not establish the cause of a different failure. Maintain evidence that changes support decisions and unresolved limits here, rather than appending daily run logs.

<a id="incus-vm-hosted"></a>
## Optional Incus VM on GitHub-hosted Ubuntu

On 2026-10-09, source `19ceda9ecd17a77f6512da276e5cfce2397bf632`
([#744](https://github.com/SLktEx/Hacocoon/pull/744), checked PR merge
`035d5c16d810c9c1ce0c38c772fd2cbe94d1015e`) passed the new
[VM discovery job](https://github.com/SLktEx/Hacocoon/actions/runs/37927938220/job/113811304884),
run `37927938220`, attempt 1. The result is **supported on this runner**:
Ubuntu 26.04.1, image `ubuntu26/20260927.149.1`, x86_64 kernel
`7.0.0-1012-azure`, Incus 7.0.1 package `1:7.0.1-ubuntu26.04-202609250211`
and QEMU 11.1.1. AMD `svm` was exposed. `/dev/kvm` was a character device
owned by root, mode 0660; the ordinary client lacked direct access (errno 13),
while the root Incus daemon successfully ran the VM. Client access was correctly
kept separate from daemon capability.

The ordinary repository probe launched an Ubuntu 26.04 VM with 2 CPUs,
1 GiB memory and only a root disk on the initialized `dir` pool. No inherited
profile, NIC, Host bind mount or management socket was supplied. Its immutable
image fingerprint was
`4d2727c7480d2f54d916a18bb092af876dd089df2fdc9de69beda9248b434f25`.
Agent exec confirmed systemd PID 1 and readiness, then repeated after verified
stop/start. Exact nonce-owned deletion passed, the always-run cleanup check
passed, and the final independent instance inventory was empty.

Artifact `incus-vm-probe-1` (`11614369907`) contains `status=supported`,
`guest_systemd=ready_after_restart` and `cleanup=verified_absent`, plus the
runner/SHA receipt. The downloaded archive SHA256 is
`788285fa59881ac978fdf848e54357e9037d2d835b0759718fc47b0f08900cd6`.
This is evidence that a future optional Incus VM backend is practical on the
sampled GitHub-hosted runner; it is not a Hacocoon VM implementation, guest
network/storage-policy acceptance, a universal runner guarantee or the complete
PR gate. Unsupported-KVM classification was covered by repository regressions,
not a native no-KVM runner. See the [discovery contract](../reliability/ci-contracts.md#optional-incus-vm-capability).

<a id="unified-creation-local"></a>

## Unified creation: local Issue #728 candidate

On 2026-09-26, the uncommitted Issue #728 working tree based on `9d63f193`
passed `TestRealIncusSnapshotAggregateE2E` on Windows/WSL2, Ubuntu 26.04.1,
Incus 7.0.1 and Btrfs. The shipped CLI used a private test controller/catalog.
The run verified running/stopped Snapshot capture with a new Image, export/import,
`open --new --snapshot`, independent rootfs/Git/Workspace/OCI bytes, fresh authority,
source-state preservation, deletion of generated data and exact owned cleanup.
Uncommitted/untracked files and unpushed commits survived. The original 20-minute
fixture budget expired after import; the bounded 45-minute rerun passed in 1,754
seconds. The first fixture was cleaned through canonical ownership-checked APIs.
Existing user Environments were not used as fixtures.

Local CI stages covered Go tests/vet/race with Go 1.26.7, command/orchestration E2E,
isolated kernel forwarding, documentation/workflow checks and Linux/Windows
snapshot packaging. Windows native installer components also passed locked-file,
ownership and junction refusal, PowerShell 5.1-to-WSL literal argument transport
and WSL stop-readiness checks. This is local working-tree evidence, not a published
release or hosted CI result. Fresh standard-Image installation, VS Code/SSH GUI
acceptance, authenticated Git network operations and live OCI consistency were
not rerun. The native aggregate deliberately skipped cached-source-image deletion,
live containerd transfer and the separately gated installed-controller import.

### Integration with updated main

On 2026-09-27, candidate `f2a2b78f` integrated main `4cbe853f`, including
Repository retry/progress and Windows restart changes. Documentation, workflow
policy, shipped-command/orchestrator E2E and isolated kernel forwarding passed.
The normal bootstrap Image acquisition path also passed against real Incus with a
private preferences catalog: the downloaded standard Image became the persisted
initial default and repeated setup succeeded. This used the existing image cache,
not a fresh Host installation. Native Windows component tests passed GUID/name
termination routing and preservation of primary/resume failures; they did not run
an actual VHD reclamation cycle.

All Go packages passed normal and race tests, followed by vet, 32 notification
client JavaScript tests and two VS Code packaging tests. The maintained `test`
and `race` invocations first hit the existing checkpoint black-box package's
10-minute limit on the Windows-mounted source. Only that package was rerun with
`go test [-race] -v -count=1 -timeout=30m ./tools/milestone`; it passed in 1,060
and 1,032 seconds respectively. Product deadlines were not changed.

The maintained `release-config` entry point passed in a local Ubuntu 26.04
container for the same production tree: Linux/Windows amd64/arm64 snapshot
artifacts, installer bundles and every generated checksum validated. The initial
minimal-container attempt lacked the fixture's `/usr/bin/python3`; installing
that test prerequisite fixed the preparation without changing installer code.

The first live-containerd aggregate saved writable data and exported successfully,
then its Host-side Git observer refused an Incus-shifted UID. The fixture now trusts
only each provider-verified path for that Git invocation, matching its existing
import/copy checks; product ownership checks and global Git configuration are
unchanged. Canonical cleanup removed the failed fixture's saved/copy resources,
and an independent inventory confirmed its original/restored volumes absent.
An earlier attempt with an auto-expired cached source fingerprint was refused
before creating any fixture resources.

The next native run verified containerd Image identity and stopped-container
writable data after shipped CLI export/import, then successfully created a new
Environment from a running-source Snapshot. Its JSON observer incorrectly mixed
stderr progress with stdout. Correction `5b5336cd` separates those streams and
adds a focused regression, which also passed with race detection. A temporary,
exact-fixture continuation repeated the original remaining assertions against the
retained catalog and passed in 619 seconds: Snapshot open, independent copy,
Git/rootfs/OCI contents, unchanged source, attached-data deletion refusal and
canonical cleanup. It was removed after compilation. An independent Incus
inventory retained all pre-existing user instances/volumes and found both fixture
names absent; the completed catalog retained only lifecycle lock identities.

This is segmented local native evidence, not an uninterrupted corrected aggregate
run. Cached-source-image deletion and the separately gated installed-controller
import remained skipped. Explicitly starting saved containerd work passed; running
task migration, arbitrary application consistency, restored SSH/VS Code GUI and
authenticated Git acceptance are not established by these tests.

### Hosted unified-creation acceptance and preview timeout

On 2026-09-27, `f43b9f7095cd1c6ad977326d5dd02876880acebc` passed
[repository tests](https://github.com/SLktEx/Hacocoon/actions/runs/36299877273),
[quality checks](https://github.com/SLktEx/Hacocoon/actions/runs/36299877282),
[real Incus](https://github.com/SLktEx/Hacocoon/actions/runs/36299877261) and
[packaged Ubuntu installation](https://github.com/SLktEx/Hacocoon/actions/runs/36299877263).
Incus included the uninterrupted aggregate Snapshot and installed-controller
transfer paths. SonarCloud reported 80.9% new-code coverage and a passing gate.

The [Windows run](https://github.com/SLktEx/Hacocoon/actions/runs/36299877270)
passed installation, lifecycle, egress, interop, customization, notification and
reclamation. Its first access job failed at the existing 30-second Edge headless
process deadline (`test_windows_environment_ssh.ps1:470`). Windows HTTP access
and the exact Workspace marker had already passed; independent SSH, VS Code,
transfer and tunnel probes also passed. The failed-job retry on the same code
passed the full access job, including Edge rendering and preview reuse/refusal.
No timeout, assertion or product behavior changed for that retry.

The evidence gate correctly retained the initial failure after the successful
retry. The Edge timeout's root cause remains unproven; a later pass does not fix
or erase it. This record preserves that limitation for the next candidate rather
than weakening the gate. These hosted substrates do not establish authenticated
Git, private-registry, live-workload migration or every physical Host configuration.

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

<a id="host-oci-sharing"></a>

### Real Host OCI sharing measurements

PR #743 head `6af9764d2ea4c7b0a01aa931c2cb1ec071ddb35e`, tested as merge
`0785baf61d9552fcd6f8de0972298a7427db5906`, passed
[`TestRealIncusHostToolingE2E` in 104.17 s](https://github.com/SLktEx/Hacocoon/actions/runs/37928748494/job/113813959793).
The dedicated pool/project was `haco-area-6e4bcd1e35bce84c`; exact fixture cleanup
passed. Substrate: Ubuntu 26.04, Linux `7.0.0-1012-azure` x86_64, Go 1.27.0,
Incus 7.0.1, Btrfs-progs 6.17.1, nerdctl 2.3.5, containerd 2.3.3 and BuildKit
0.31.2 with the native snapshotter. The actual BusyBox/BuildKit-built image was
`sha256:964017daedcd872403b9aef02dd51e47a727d2c42bfce6d57495e6f63f9980f7`.

The [reproducible measurement procedure](../design/btrfs-storage-layout.md#host-oci-copy-measurement)
uses `du --apparent-size --block-size=1`, `du --block-size=1`, and
`btrfs filesystem du --raw --summarize` while the exact owned consumers are paused
through Incus. Selected whole-Store results, in bytes:

| Area / operation | Logical | Referenced allocation | Extent total | Exclusive | Set-shared |
|---|---:|---:|---:|---:|---:|
| Host before copy | 23,136,518 | 23,183,360 | 22,994,944 | 5,120,000 | 8,937,472 |
| Host after native copy | 23,136,518 | 23,183,360 | 22,994,944 | 0 | 14,057,472 |
| Copy after native copy | 23,136,518 | 23,175,168 | 22,994,944 | 0 | 14,057,472 |
| Copy before container write | 23,136,518 | 23,179,264 | 22,999,040 | 307,200 | 13,754,368 |
| Copy after 8 MiB container write | 35,970,962 | 36,069,376 | 35,856,384 | 8,732,672 | 13,717,504 |
| Copy after container/tag deletion | 23,136,518 | 23,179,264 | 22,999,040 | 352,256 | 13,709,312 |

After native copy, actual containerd content blobs had 2,240,512 shared extent
bytes and zero exclusive bytes in each area; copied BuildKit data had 7,147,520
set-shared bytes and zero exclusive bytes. Native parent-UUID ancestry, same image
identity, offline execution, independent writes and source deletion passed.
The 8,388,608-byte random writable-layer payload increased referenced allocation
by 12,890,112 bytes and exclusive extents by 8,425,472 bytes; native snapshot/image
references and runtime metadata explain why those counters are not payload size.
The source image remained usable and did not contain the receiver's write.

The 4,294,967,296-byte sparse pool backing file's allocation changed from
2,462,105,600 to 2,462,134,272 bytes across native copy (+28,672). Receiver
provisioning occurred before the next baseline: 3,013,103,616 bytes before the
container write and 3,021,672,448 afterward (+8,568,832). After deleting the
container and one image tag, the source image and receiver's second tag remained;
backing allocation was 3,021,905,920 bytes, not a measured physical decrease.
These are whole-pool observations including rootfs/metadata activity, not isolated
device-write counters, compressed-extent sizes or export/import amplification.
The full log retains both areas and their content/BuildKit subarea samples.

The first head `ae95116bb04122f672baf9046cc7597ab374b007`
[failed before byte measurements](https://github.com/SLktEx/Hacocoon/actions/runs/37927891326/job/113811150446):
pull/build/reuse passed, but the observer rejected Btrfs' standard feature-bearing
version banner. The first-line version parser and its regression fixed that
observer defect; the earlier failure and skipped Packer stage remain distinct.
Local focused/race/vet/lint and documentation checks passed; local native cases
were skipped because Incus/Btrfs were unavailable. The hosted result above is the
native Host evidence. The next section adds bounded managed-Workspace
capture/restore/copy measurements. Base/multiple-Env and saved-rootfs restore byte
accounting, archive/publish amplification, Docker drivers, completed physical
reclamation and large-workload measurements remain open under
[#241](https://github.com/SLktEx/Hacocoon/issues/241).
The [rootfs/Image receipt below](#rootfs-image-sharing) covers a further
bounded slice; it does not extend this Host OCI receipt.

<a id="workspace-sharing"></a>

### Managed Workspace sharing and logical deletion

PR #751 head `729b3009495e76dc8db09df990eb83e55f63c9c8`, tested as merge
`a7060f189fadc98dc005bd7af75d8c3b41cc8d0e` into `f05d3d1554b405bfa2db5b83b7f1dd4f226992ca`,
passed [`TestRealIncusSnapshotAggregateE2E` in 229.01 s](https://github.com/SLktEx/Hacocoon/actions/runs/37964529331/job/113935434154).
The [tested fixture source](https://github.com/SLktEx/Hacocoon/blob/729b3009495e76dc8db09df990eb83e55f63c9c8/internal/adapters/incus/snapshot_aggregate_e2e_test.go#L448-L804)
retains its original Git/content/ownership assertions and confirmed exact-owned
cleanup. Fixture `haco-aggregate-11705984157cdfa3` used Ubuntu 26.04,
Linux `7.0.0-1012-azure` x86_64, Go 1.27.0, Incus 7.0.1 and
Btrfs-progs 6.17.1. The cached input image fingerprint was
`ffb1a536af31961dbc1976f5e0ca9a900ae43dbe50619fa179ad2963bd9d4173`.

The [measurement procedure](../design/btrfs-storage-layout.md#managed-workspace-lifecycle-measurement)
uses bounded guest writes in the selected managed repository, canonical capture,
public restore/copy and exact-owned deletion. Historical synthetic setup writes
occur before this interval. Each physical sample holds lifecycle locks and pauses
only the exact running consumers. Whole selected-Workspace results, in bytes;
combined labels below mean each listed area had the same values, not their sum:

| Area / operation | Logical | Referenced allocation | Extent total | Exclusive | Set-shared |
|---|---:|---:|---:|---:|---:|
| Source before Snapshot | 8,416,620 | 8,511,488 | 8,404,992 | 8,388,608 | 12,288 |
| Source and saved after Snapshot, each | 8,416,620 | 8,511,488 | 8,404,992 | 0 | 8,400,896 |
| Source after unlink, Snapshot retained | 28,012 | 122,880 | 16,384 | 0 | 12,288 |
| Saved after source unlink | 8,416,620 | 8,511,488 | 8,404,992 | 8,388,608 | 12,288 |
| Saved and restored after public restore, each | 8,416,620 | 8,511,488 | 8,404,992 | 0 | 8,400,896 |
| Copy after public copy | 8,416,620 | 8,511,488 | 8,404,992 | 0 | 8,400,896 |
| Copy after independent 8 MiB write | 16,805,228 | 16,900,096 | 16,793,600 | 8,388,608 | 8,400,896 |
| Copy after Snapshot deletion, restored source retained | 16,805,228 | 16,900,096 | 16,793,600 | 8,388,608 | 8,400,896 |
| Copy after source and Snapshot deletion | 16,805,228 | 16,900,096 | 16,793,600 | 16,777,216 | 12,288 |
| Copy after last live payload unlink | 28,012 | 122,880 | 16,384 | 0 | 12,288 |

The original random payload was 8,388,608 logical/referenced/extent bytes. It
changed from wholly exclusive before capture to zero exclusive and 8,388,608
set-shared bytes in each captured/restored/copied payload directory. Its SHA-256
remained `c2d6ac8279f1a8189b1a6513aa10baecc10b48ea167e4940bcca9fc98f9d09b7`.
The copy-only 8 MiB write increased both whole-Workspace referenced allocation
and exclusive extents by exactly 8,388,608 bytes; saved/restored payloads remained
unchanged. After logical source/Snapshot deletion, this run observed all
16,777,216 payload extent bytes exclusive in the surviving copy. Final guest
unlink reduced its payload's logical, referenced and extent counts to zero;
the separate Git/Workspace data remained.

The shared pool's sparse backing file stayed 137,438,953,472 bytes long. Its
observed allocation was:

| Phase | Whole-pool allocated bytes |
|---|---:|
| Before Snapshot | 745,246,720 |
| After Snapshot | 748,982,272 |
| Source unlink, Snapshot retained | 750,309,376 |
| After public restore | 1,455,939,584 |
| After public copy | 1,463,418,880 |
| After copy-only write | 1,472,823,296 |
| Snapshot deleted, restored source and copy retained | 1,474,723,840 |
| Source and Snapshot deleted, copy retained | 1,476,001,792 |
| Last live payload unlinked | 1,477,312,512 |

These counters include other rootfs, Image, metadata and Host activity. Aggregate
transfer/runtime steps also intervene between source-unlink and restore samples;
the large increase is not attributed to Workspace restore. The final unlink
removed 16 MiB of live payload references while backing allocation increased
1,310,720 bytes. No isolated physical-write cost, compressed-byte saving, discard
or Windows VHDX reclamation is established. FIEMAP counts remain separate from
referenced allocation and are not summed across copies.

Each phase performs one filesystem sync and one observation, without waiting or
retrying for capacity reduction. [Btrfs filesystem sync](https://btrfs.readthedocs.io/en/latest/btrfs-filesystem.html#subcommand)
starts deleted-subvolume cleaning but does not wait for it. Provider absence and
the observed exclusive transition therefore do not establish a general completed
physical-retirement fence. The full job receipt retains 17 area samples, nine
pool samples, exact ownership-identity hashes and successful cleanup. This
bounded Workspace-data result leaves saved-rootfs byte accounting, Base/multiple
Env, archive/publish amplification, completed reclamation and large workloads open.

<a id="rootfs-image-sharing"></a>

### Saved-rootfs sharing and ordinary Image reuse

PR #752 head `de3817208964ab0c0d0cbdb4123567b4a9e59bd2`, checked out and
built as merge `031d0a7b9996a661f7b9dda6e54a748320ccda5b` into
`932790322bcd54d8b8aa6fda5a13eaec030c0f67`, passed
[`TestRealIncusSnapshotAggregateE2E` in 272.24 s on its first attempt](https://github.com/SLktEx/Hacocoon/actions/runs/37992973385/job/114031637967).
The [tested fixture](https://github.com/SLktEx/Hacocoon/blob/de3817208964ab0c0d0cbdb4123567b4a9e59bd2/internal/adapters/incus/snapshot_aggregate_e2e_test.go#L448-L886)
used `haco-aggregate-b632059ba14ffa2d`, Ubuntu 26.04.1 (OS identity 26.04),
Linux `7.0.0-1012-azure` x86_64, Go 1.27.0, Incus 7.0.1 and
Btrfs-progs 6.17.1. The input cached image fingerprint was
`08360c2fb053a234134c457588b90b733344188af95bddce1b2eba3ebb68b8b3`;
the independently published ordinary Image had
`origin=snapshot-generated-image` and fingerprint
`d26206bc2cb3722868879881e882fbe8169178548a1883f3aab276aae35de4a4`.

The [measurement contract](../design/btrfs-storage-layout.md#rootfs-capture-and-ordinary-image-reuse-measurement)
keeps two paths separate: native saved-rootfs capture/restore/copy, and normal
positional `open --new IMAGE` initialization from the immutable optimized cache.
Exact ownership, canonical locks, verified pause/resume, read-only cache identity,
root/parent identity and Host mount-scope checks passed. Whole-rootfs samples
below are in bytes; they include runtime files, not only the controlled payload:

| Area / operation | Logical | Referenced allocation | Extent total | Exclusive | Set-shared |
|---|---:|---:|---:|---:|---:|
| Source before capture | 671,568,712 | 731,987,968 | 672,321,536 | 12,496,896 | 275,742,720 |
| Saved after capture (source identical) | 671,568,712 | 731,987,968 | 672,321,536 | 0 | 288,100,352 |
| Image cache after two ordinary Envs | 671,568,712 | 731,987,968 | 672,321,536 | 0 | 281,776,128 |
| Ordinary Image Env A | 680,050,419 | 740,478,976 | 680,804,352 | 8,486,912 | 281,776,128 |
| Ordinary Image Env B before write | 679,957,417 | 740,384,768 | 680,710,144 | 8,388,608 | 281,776,128 |
| Ordinary Image Env B after write | 688,441,496 | 748,871,680 | 689,197,056 | 16,879,616 | 281,776,128 |
| Cache after both Image Envs deleted | 671,568,712 | 731,987,968 | 672,321,536 | 672,321,536 | 0 |
| Restored from saved rootfs | 680,047,008 | 740,474,880 | 680,800,256 | 8,482,816 | 288,096,256 |
| Copy before independent write | 680,142,757 | 740,564,992 | 680,894,464 | 4,083,712 | 292,585,472 |
| Copy after independent write | 688,531,365 | 748,953,600 | 689,283,072 | 12,472,320 | 292,585,472 |
| Copy after source/Snapshot deletion | 688,531,355 | 748,953,600 | 689,283,072 | 29,458,432 | 275,742,720 |
| Copy after final payload unlink | 671,754,139 | 732,176,384 | 672,505,856 | 12,681,216 | 275,742,720 |

The original rootfs payload was exactly 8,388,608 logical, referenced and extent
bytes, wholly exclusive before capture. After capture, saved-rootfs restore and
copy, each corresponding payload had zero exclusive and 8,388,608 set-shared
bytes. The same shared-payload counts were independently observed in the generated
Image cache and both ordinary Envs. Its SHA-256 stayed
`b7c7e4e6f5421e7f8ecec0aded73211eeafc25e52e6db5a00d84512f4c103618`.
After unlinking the source file, the saved payload was intact and wholly exclusive,
even while the separately materialized Image cache/Envs shared their payload.
This does not establish sharing across Image publication.

Each independent 8 MiB write added exactly 8,388,608 payload referenced/exclusive
bytes: once in ordinary Env B and once in the restored copy. Peer, cache and
saved payloads remained unchanged. Env B's whole-rootfs allocation increased
8,486,912 bytes and exclusive extents 8,491,008 bytes; runtime activity makes
these different from the fixed payload change. The restored copy's whole-rootfs
allocation and exclusive extents each increased 8,388,608 bytes.
After deleting both ordinary Image Envs, this run observed the retained cache's
entire rootfs exclusive. After deleting the Snapshot and restored source, the
surviving copy's 16,777,216 payload extent bytes were exclusive. Final guest
unlink reduced all payload counters to zero while other rootfs data remained.
These exclusive transitions are observations, not a required retirement fence.

The shared sparse backing file remained 137,438,953,472 bytes long:

| Phase | Whole-pool allocated bytes |
|---|---:|
| Before capture | 748,027,904 |
| After capture | 760,381,440 |
| Source unlink, Snapshot retained | 761,937,920 |
| After two ordinary Image Envs | 1,108,312,064 |
| After Image Env B write | 1,118,994,432 |
| Both Image Envs deleted, cache retained | 1,119,551,488 |
| After public restore | 1,816,449,024 |
| After public copy | 1,824,456,704 |
| After copy-only write | 1,833,926,656 |
| Snapshot deleted, restored source/copy retained | 1,845,264,384 |
| Source/Snapshot deleted, copy retained | 1,847,066,624 |
| After final live payload unlink | 1,848,442,880 |

Other rootfs, Image, Workspace, metadata and Host activity share this pool.
Aggregate transfer/import also intervenes before the restore sample. No pool
delta is attributed to one operation. Final live unlink removed 16 MiB of payload
references while pool allocation increased 1,376,256 bytes; no completed physical
reclamation was observed. FIEMAP lengths are not compressed-byte or device-write
counters, and per-area values are never summed as unique storage.

The full receipt retains 27 rootfs/payload area samples, 12 whole-pool samples,
identity hashes and the existing 17 Workspace area/nine pool samples. Exact-owned
Env/automatic Workspace cleanup, saved aggregate cleanup and final fixture
absence passed; the job then deleted its pool and project. Persistent catalog
lock identities were intentionally retained. Original cached input-image deletion
was explicitly skipped because dedicated-image permission was not enabled.
No retry was used. Local sandbox failures remain distinct from this hosted pass.
General Base-build/Packer efficiency, archive/publish amplification, broader OCI
workloads, restored SSH/live application consistency, completed reclamation,
large workloads and Windows VHDX effects remain outside this bounded result for
[#241](https://github.com/SLktEx/Hacocoon/issues/241).

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
The corrected package is `internal/policy/review`. The first lint patch incorrectly
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

At #664 head `aef58798`, Windows34918511743/job104221234323 passed installation, strict SSH and Linux reclamation. Public reclamation Host re-entry failed at 02:08:10 UTC with `stage=notification_setup reason=failed`; the observer then waited until 02:37:51 and timed out. The public reclaim operation was not reached and notification step20 was skipped. Other four workflows passed. This differs from the earlier clear/COM activation failures. The follow-up reuses `5a6fb54c` classification and failed-entry detection, without claiming a root-cause fix.

The main notification-setup integration reuses 5a6fb54c over GUI aef58798 and main 5e89597a. Focused tests4.37s, notification Python regressions0.69s, changed-code lint30.66s, full local117.79s, race20.22s, CLI11.78s, docs17.46s and workflow2.88s passed. Initial integration testing failed at import of a future stream acceptance script absent on main; its unrelated test import was removed while retaining the existing native entry regression. Windows execution of the native observer tests passed6 tests0.555s. No new installed notification success is claimed.



After integrating main `5e89597a` as `3d8c2877`, the cache foundation passed full local tests (13.62s), CLI E2E (3.27s) and docs/regressions (4.86s). Earlier head `22b119d8` passed all five workflows, including Windows34917359766. Public collection is a separate follow-up; this foundation does not enable enrollment.

After merging current main5121b205 as ac145abc, the combined GUI/notification candidate passed full local tests50.00s, CLI12.86s and docs/regressions31.34s. The aef58798 installed notification-setup failure remains unresolved pending new fixed-operation evidence.

## Git push reconciliation integration

The main candidate reuses 42aa706f over GUI/notification source 8d509399. Focused tests (9.31s), changed-code lint (12.57s), maintained local tests (25.06s), race (28.69s), CLI E2E (4.44s), documentation/regressions (8.66s), and workflow policy (1.50s) passed with Go 1.27.1. The first lint attempt found an unchecked audit-fixture close; the fixture was fixed before the complete pass. Tests exercise real Git repositories and read-only remote observation, including ambiguous receipts and replaced Environment identity. Authenticated remote Git and human approval acceptance were not run. Reconciliation never repeats a push or infers its original success from current branch equality.

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

After merging main4cd0c7dc as611bedaf, the combined Git/GUI/Packer/cache source passed full local tests14.30s, CLI3.36s and documentation/regressions5.62s. At prior GUI head8d509399, Windows34923857407 passed SSH and public reclamation but notification clear failed after8024ms with progress=decode. The same fixed native script on this PC under normal Windows permissions passed isolated test IDs in0.53s/0.23s; the restricted execution frame refused before notification work. This does not resolve the CI failure or prove human answers.

At dca688f6, Windows34926634572/job104245971260 again failed initial clear with reason=timeout, child_exit=1, duration_ms=8019, native_progress=decode; previous SSH/public-reclaim steps passed. The product process ceiling is now30 seconds for cold PowerShell/WinRT initialization, while existing shorter caller deadlines and exact child cancellation remain. This is a bounded startup correction to verify, not proof of the underlying CI slowdown or successful GUI acceptance.

The corrected source passed the full native Windows review test suite on this PC in 8.22s, including actual English/Japanese ToastGeneric display, history and removal (6.90s), activation callbacks, redaction and exact child cancellation. This is native component evidence; human clicks, visual layout acceptance and the CI cold-start outcome remain unverified.

## Named cache completion recovery

The follow-up to #670 passed focused lifecycle/cache/OCI/CLI/controller tests19.10s, changed-code lint15.71s, full local39.78s, race14.61s, CLI7.25s, docs/regressions10.41s and workflow2.00s. A real catalog with staged provider failure proves completed copies recover without recopy, original source pins are released only after verification, reset candidates remain retained, and unknown/provider-refused/wrong-owner cases remain blocked. Common OCI recovery now passes the exact owned reference. This is component evidence, not new native recovery or Windows acceptance. The earlier native maintenance timeout remains unresolved.

Final recovery usage text and bilingual help passed CLI/UI/controller tests9.40s and docs/regressions12.72s; ownership/recovery code is unchanged from the full validation above.

At recovery head3ac51f91, quality34924782288 failed QF1003 in cache_maintenance.go:41; test34924782199, Ubuntu34924782166 and Incus34924782231 passed. The dispatch is now a tagged switch with unchanged behavior. This failure remains distinct from local changed-patch lint success.

The dispatch correction passed focused control/cache/workspace tests11.23s, uncapped lint against current main18.59s and documentation/regressions8.48s. Reparenting onto main4cd0c7dc (the exact tree of tested parent1ee2962b) changed no files.

After integrating main df22a1a5 as 2f07fa3d, full local tests passed95.04s, CLI8.56s and docs/regressions10.27s. Native notification evidence above applies to the unchanged notification implementation.

## Main client forwarding integration

The candidate reuses640c66ff/4b0b5baa/70037223/44a30b1a/bc8b915b/297b3095 over cache recovery3ac51f91. The initial focused build failed because the current SSH stream already owned readiness/completion types; reusing44a30b1a shared byte-stream handling removed the duplication. The next focused run passed6.54s but lint failed73 unchecked results and8 static findings in imported code. These were fixed without weakening validation. The corrected immutable source passed focused3.76s, lint4.64s, full local11.26s, race7.74s, CLI2.84s, docs5.24s and workflow0.98s (Go1.27.1). Windows amd64 compiled components then passed streamio0.27s, clientforward1.09s, wsllaunch0.34s and native CLI0.69s, covering real pipes/TCP/half-close, parent loss, exact target refusal and local help. These are component results, not new installed Windows/Incus acceptance.

The PowerShell5.1 installer component command was refused by the machine script-execution policy before tests ran. No execution-policy or isolation setting was relaxed. Existing source297b3095 has local foreground-interruption regression evidence, while its installed Ctrl+C rerun remained pending; preserve that boundary. The first checkpoint invocation could not interpret Windows worktree administration paths from Linux and made no change. The existing Windows lock helper around the same WSL updater succeeded, without bypassing the lock.

Native PowerShell7 installer components passed with both installed binary types: install/reinstall, missing/mismatched ownership, checksum, pinned worker and junction refusal. PowerShell5.1 remains not run under its script policy; neither policy was changed. The existing #634 evidence records parent #632 Ubuntu34763684021/job103740815424 passing eight concurrent2MiB installed Linux tunnel exchanges after the readiness fix. Reused #642 source preserves the later Windows Ctrl+C gap; neither result is relabeled as acceptance of this new main candidate.

After the v0.64 checkpoint and installation guidance update, presentation/build-identity tests2.81s, architecture/checksum/corrupt-archive packaging0.54s, docs5.72s and workflow1.11s passed. Windows CI now expects the actual ten build entries and requires the tunnel artifact; published amd64/arm64 targets stay intact.

The candidate rebased over corrected recovery c35f6610 as c682faaa passed focused4.22s, main-diff lint16.99s, full local104.53s, race36.42s, CLI18.56s, docs19.60s and workflow2.66s. Recovery #671 then passed all five workflows and merged as main df22a1a5. Its file tree exactly matches c35f6610, so reparenting the forwarding commit as11823e02 changed no files.

After integrating main forwarding 2f995027 as14b8fb9a, the Git/GUI/notification candidate passed full local53.41s, CLI8.88s, docs/regressions19.00s and workflow2.79s. Both private entry points and the union of native Windows client tests are retained; notification process behavior is unchanged from the native8.22s pass.

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


Integrating main9f5bc9e3 (DNS and named-data snapshot/copy) as ab77483f changed only evidence overlap during merge. Combined Git/GUI local tests105.78s, CLI7.58s, docs10.98s and workflow2.62s passed. The earlier Windows parallel cold SSH stream_denied remains unexplained; these local passes do not replace native acceptance.


## Calling-thread network identity

The native namespace regression FAILED11.20s with the former process-leader opener, reporting another thread's identity. After switching to the calling-thread namespace it PASSED23.70s using the same namespace authority. Focused13.84s, changed-code lint12.61s, full local33.40s, race18.79s, CLI4.29s, docs8.59s and workflow1.50s passed. Host-namespace refusal and dedicated-thread destruction remain unchanged. This establishes the defect and correction; earlier Windows stream_denied failures remain uncorrelated until installed reconnect runs.

A fresh ordinary Windows installer from f68a8c6b completed on Hacocoon-Roadmap-f68a8c6b (Ubuntu26.04.1/Incus7.0.1), including storage/trusted Host, doctor DNS/HTTPS, Windows enrollment and notification registration. No test permission override was used. Supported named-data rootfs snapshot/copy/export/import PASSED76.26s/test76.23s; fixture saved-data-c1685c13e42f7479, catalog /var/lib/haco-saved-data-90071584/state.json. Fresh identities, independent edits, imported restart and exact cleanup passed. No human notification, authenticated Git or giant-repository claim.

The separate repository-subdirectory attempt FAILED11.61s/test11.56s at capture: capability stale, Env left stopped. Fixture saved-data-ec2813c23b2ada60, catalog /var/lib/haco-saved-data-3393557504/state.json. This occurs on supported7.0.1 and is a product defect to investigate, separate from earlier6.0.5 missing APIs. Source cleanup ran; retained Workspace fixture records remain.

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

The combined candidate b42be03e includes current main a3d0f4fd, Git/GUI #672 and repository placement #679. Focused21.73s, complete main-diff lint23.92s, full local41.13s, race21.73s, CLI5.39s, docs/regressions11.17s and workflow1.75s passed. The existing native namespace and repository-placement evidence applies to unchanged implementations. #677 passed all five workflows at f71ab282 and merged as a3d0f4fd; #676 was closed by incorporation. Its separate Windows tunnel failure above remains recorded.

## Selected Workspace membership

The candidate based on6b376a62 passed focused10.58s, complete main-diff lint16.44s,
full local25.61s, race27.75s, CLI2.86s, docs/regressions8.59s, workflow1.36s and
native-test compilation1.55s. Earlier full-1 also passed; full-2 adds the actual
registered-copy failure/source pin regression and native acceptance test.

On Hacocoon-Roadmap-f68a8c6b, Ubuntu26.04.1/Incus7.0.1, membership-native-1
PASSED16.98s/test16.95s. Fixture selection-c393aee4e83d, catalog
/var/lib/haco-selection-154075238/state.json, used test-authored local Git data,
the real repository backend and canonical snapshot/Env lifecycle. It preserved
saved dirty files, HEAD and index, added a registered source through the normal
Git preparation, left an omitted member in the source, and proved independent
edits, source Env deletion, destination restart and exact owned cleanup. No Policy
changes or guest management authority were introduced. OCI selection is covered
at the existing associated-data component boundary; native OCI, authenticated Git,
public CLI and giant-repository performance were not exercised by this native test.

## Outstanding Windows candidate failures

PR #678 at `6b376a62` passed quality, test, Ubuntu and Incus CI. Windows run
34938847867/job104282639159 passed ordinary SSH/editor/tunnel, reclamation and
retained-data restoration, then failed native notification stale activation:
`activation/timeout`, dispatch HRESULT -2147220990 (the helper's read deadline).
Reclamation observed 7,864,320,000 to 5,041,553,408 allocated bytes. This does not
resolve earlier tunnel/reclamation failures or establish human approval answers.

PR #680 at `2e8d905c` also passed all four Linux workflows. Windows run
34940269831/job104287130520 passed installation, native SSH/editor and Linux
reclamation, then failed public reclaim with `compact_attached`: one native open,
no compaction attempted, WSL resumed. Notification was SKIPPED. Neither PR is
merged on these failed Windows results.

A local notification probe on Hacocoon-Roadmap-f68a8c6b failed with review timeout.
Only the Windows helper had been updated; installed Linux still reported f68a8c6b
and did not implement `_desktop-review`. This mixed candidate is not acceptance
of #678 and does not explain its separate CI activation failure. A matching
ordinary installation must precede the next local end-to-end probe.

## Independent worktree input

The immutable input candidate based on2e8d905c passed full-3: focused36.42s,
complete main-diff lint22.84s, full local39.45s, race31.22s, CLI4.92s,
docs/regressions9.04s, workflow1.71s and native-test build1.96s. Final additional
archive traversal/alias/xattr/privilege/trailing-data regressions passed with
race3.01s and full main-diff lint19.81s. The CLI regression preserves a receipt
on an unknown result, refuses replay/replacement and never opens an unconfirmed
import. Real local Git regressions cover checkout, linked worktree, split index,
packed refs, staged/dirty content and exclusion of Host config/admin state.
Full-1 stopped at15 lint findings after focused26.38s; those were corrected.
Full-2 passed all checks before the extra CLI/refusal regressions.

On Ubuntu26.04.1/Incus7.0.1 in Hacocoon-Roadmap-f68a8c6b, input-native-1 passed
26.23s/test26.19s. Fixture selection-384863c4ef38, catalog
/var/lib/haco-selection-1381113910/state.json, copied an actual local linked
worktree through the real provider-neutral capture and Incus import into a new
managed volume. Selected HEAD/files, independent guest editing, normal Env
stop/start and exact cleanup passed alongside the existing membership test.
The product implementation was the candidate overlay, not unchanged2e8d905c;
the dedicated test binary was used, not the installed CLI. No Policy relaxation
or guest management authority was added. Authenticated Git, human GUI answers,
installed input and giant-repository measurements were not exercised.

### Installed Linux input gate

The candidate based on main `7a3a6b8b` adds
[`workspace_input.py`](../../test/e2e/installed/workspace_input.py) to the existing
mandatory Ubuntu installed lifecycle journey. It uses ordinary `repo add`,
`workspace import`, path `open`, `exec`, stop/reopen, explicit Environment/Workspace
deletion and source unregistration against the installed controller. The upstream
is the public Hacocoon repository; input
is a small local checkout and an actual linked worktree. No credential, private
registry, replacement controller, catalog edit or product setup repair is needed.
Git is an optional selected-Image prerequisite under [getting started](../guides/getting-started.md#develop-and-return-later).
For its two exact-owned Environments only, the fixture uses public `config` and
`exec` commands to install missing Git through ordinary Ubuntu package access.
Four temporary rules per Environment permit only `archive.ubuntu.com` and
`security.ubuntu.com`, HTTP/80 and HTTPS/443, with a 15-minute expiry. No wildcard,
global or DNS grant is added. These administrator grants are name-scoped: safety
is bounded to the fresh single-runner fixture's unique nonce names, with recorded
receipts checked before configuration and immediately before `exec`. This does
not claim instance-bound permission or atomic name-based execution. Each package
operation removes its exact rules in `finally`, preserving unrelated Policy;
changed/ambiguous rules fail closed and prevent Env deletion/name reuse before
runner teardown.
Configuration uses revision-bound public snapshots, never direct protected-file
writes or save replay. An absent snapshot cannot complete an unconfirmed add:
a delayed save may still commit. Such ambiguity retains pending grants and Env
names. A pending removal is reconciled only by observing absence after the add
was confirmed; an uncertain save is never automatically resent. Package failures
remain failures; Git/data assertions are unchanged. The fixture does not require Git in every Base or change the default Image.

Freshness is an execution precondition: this fixture runs only in that disposable
GitHub-hosted installer journey. It refuses execution without both `GITHUB_ACTIONS=true`
and `HACO_CI_RUNNER_ENVIRONMENT=github-hosted`; those markers do not prove freshness.
The empty `repo list --json` check is additional protection only. The public list
hides excluded sources, and registration can reuse equivalent HTTPS/SSH/scp
remotes. Do not run it against an existing installation. Unexpected registration
receipts are not adopted or unregistered; the requested identity and local receipts
remain for inspection. No test claims to detect records hidden by the public API.
Import/open observations are saved separately before validation; only validated
responses enter cleanup ownership. Nonzero import JSON can carry a recovery
reference, while the current open command emits JSON only on success. Records
retain bounded public identity/status fields, never raw command output or secrets.
On handled failures, an `INSTALLED_INPUT_FAILURE` JSON line preserves that bounded
report in the existing Actions logs after runner teardown. Local source trees
are not uploaded; abrupt runner termination may prevent the failure report.

Assertions distinguish selected HEAD, staged/dirty/untracked files, independent
guest commits, another managed Workspace and unchanged client/common Git state.
Repeated open and stopped resume must retain the same Environment/Workspace
identity; a mismatched path reference must be refused. Environment/Workspace
cleanup checks exact recorded owners and positive absence, preserving unconfirmed
creation receipts. `repo delete` only unregisters the exact owned source from
active selection. Its absence from `repo list` does not prove physical deletion:
native source data is intentionally retained until the disposable runner is torn
down. No purge or older repository-deletion behavior is used.
Fixture/cleanup regressions are repository evidence only. The installed success
below covers this added slice; it does not complete issue #344's Linux SSH,
connection-enumeration/forwarding or editor requirements. Existing Windows
acceptance remains separate and is not rerun or replaced by this fixture.

At `be032d9f`, [Ubuntu run 37934157996](https://github.com/SLktEx/Hacocoon/actions/runs/37934157996)
passed installation and the existing lifecycle, then failed the first Workspace
import with a connection reset and a retained recovery-required identity. The
bounded report survived in the job log; no imported Env was opened. A component
regression reproduces the missing `$HACO_ROOT/transfers` directory through the
actual receiver, upload framing over `net.Pipe` and native archive staging.
The receiver now initializes that private directory like the other import handlers.
Missing/existing roots, unsafe existing paths and retained failure identities pass
locally with race; installed acceptance at this stage remained pending. The initial
native failure is not relabeled a pass or bypassed by fixture-side preparation.

At `4e8f9464`, [Ubuntu run 37951046940](https://github.com/SLktEx/Hacocoon/actions/runs/37951046940)
passed both independent imports and their first opens. The next guest Git assertion
failed with `git: not found`; exact-owned cleanup completed and bounded receipts
were retained. This is the documented optional Git prerequisite, not an import
failure or a requirement to modify all Bases. The corrected fixture follows the
package-permission path above. Local regressions reproduce the earlier missing
prerequisite, preserve all Git assertions, and check bounded grants, cleanup,
uncertain saves and concurrent Policy edits. Installed execution of this follow-up
is recorded below. A separate default/custom-root registration regression verifies lazy
staging initialization without touching the default root or invoking the service.


At `23241a4`, [Ubuntu run 37954236509](https://github.com/SLktEx/Hacocoon/actions/runs/37954236509)
and its evidence gate passed. The installed linked-worktree fixture completed
selected HEAD/index/working-file checks, ordinary guest commits, independent
Workspace/client/common-Git isolation, mismatched-owner refusal, repeated open,
stop/resume, exact-owned Environment/Workspace deletion and source unregistration.
The job recorded both owned Env deletions, both Workspace deletions and final
fixture PASS at 2026-10-09 15:52:58 UTC. That PASS is reachable only after both
package setup `finally` blocks and fresh configuration reads confirm exact
removal, followed by positive resource-absence checks. This is code-bound
assertion evidence; no separate raw Policy snapshot is retained. The same job's
installed network checks also passed. External SonarCloud passed at 100% new-code
coverage. Other historical Windows/network failures are not resolved by these
results, and issue #344's remaining client/connection requirements stay open.

The final candidate normally merges main `23499701` after this scoped success;
its combined exact-head CI is separate and pending.

## Retained cache catalog maintenance

The candidate based on809bfb33 passed focused12.91s, complete main-diff lint15.08s,
full local26.56s, race11.53s, CLI4.17s, docs/regressions7.95s, workflow1.32s and
native-test compilation1.45s. Regressions cover missing producers, exact reviewed
owners, changed selection/display membership, incomplete/busy cleanup, recovery
without deletion, caller-path/owner refusal and common CLI confirmation/display.

On Ubuntu26.04.1/Incus7.0.1 in the dedicated WSL, cache-native-1 passed17.70s
(test17.67s). Fixture data-e2e-25cadfe2239e648b,
catalog /var/lib/haco-data-placement-24466710/state.json, exercised ordinary Env
creation, two cache areas, stopped collection, independent data-bearing reuse,
clear while a consumer remained live, then --all workflow inspection/cleanup of
retained collected data after both Envs were deleted. The external Workspace
marker remained. All operations used the fixture's private catalog and canonical
ownership transitions; no global/user data or Policy relaxation. This uses the
real backend and Standard workflow, not installed public CLI, OCI or large-repo
performance acceptance.

#681 at809bfb33 passed the four Linux CI workflows. Windows34943936799/job104298802478
passed installation, SSH/editor, tunnel and public reclamation, then FAILED stale
notification activation with dispatch HRESULT -2147220990 at its10-second deadline.
Notification follow-up and human decisions were not established by that run.
Readiness is addressed in #682; this historical failure remains a failure.


## Native review readiness

Matching installation of809bfb33 on Hacocoon-Roadmap-f68a8c6b completed normally.
The first invocation of the test used PowerShell5.1 and did not run because the
fixture requires7. With PowerShell7, native-review-matching-2 FAILED review timeout.
A read-only probe using the same WSL command/environment/private pipes received
an empty pending list in13.56s; the original first-review limit was10s. This
establishes a local startup timing failure, not the cause of every past CI failure.

The readiness candidate based on809bfb33 passed focused5.32s and complete main-diff
lint20.03s. Windows component tests passed0.83s, with native toast display initially
SKIPPED. Separate enabled native display/history/removal passed2.64s/test2.63s with
English/Japanese XML; no human click or visual-layout claim. An earlier unquoted
PowerShell -test.v invocation failed argument parsing before tests; structured
arguments corrected that harness invocation.

The ordinary Windows helper installer then applied the readiness candidate to
that dedicated distribution; its Linux809bfb33 protocol implementation was unchanged.
installed-review-1 PASSED actual COM registration, owned resume/idempotence, foreign
and mismatched activator refusal, stale/malformed input refusal, Host notification
controller subscription and listener cleanup. Human toast click/fresh UI decision
remains explicitly SKIPPED. No Policy/auth changes or answer retries were used.
The older #678 CI activation timeout and #680 compact_attached results remain
separate unresolved observations.


The final readiness candidate also passed full local26.41s, focused5.11s,
complete main-diff lint19.06s, race2.30s, CLI4.14s, docs/regressions8.03s and
workflow1.54s. This does not turn the skipped human response into acceptance.


The combined cache candidate2f49460f also includes notification readiness #682.
Focused15.09s, complete main-diff lint14.42s, full local28.63s, race11.70s,
CLI4.18s, docs/regressions8.27s and workflow1.48s passed. Both native implementations
are unchanged from the separately recorded supported-Incus and Windows checks.

## Snapshot deletion inspection candidate

At implementation `9c2736db`, local `snapshot-full-3` passed focused regressions
(17.96 s), lint against the complete main diff (10.05 s), maintained full tests
(25.99 s), lifecycle/API race checks (16.47 s), CLI E2E (4.12 s), docs (8.06 s),
workflow policy (1.51 s) and native test compilation (1.51 s). The first focused
run failed human message formatting and a stale request field in the test fixture;
both were corrected. The second run passed focused tests but failed errcheck on
the new diagnostic write; that was corrected before the full third run.

On dedicated WSL `Hacocoon-Roadmap-f68a8c6b`, Incus 7.0.1,
`snapshot-native-2` passed in 25.75 s (test 25.71 s), fixture
`saved-data-61784250e8f288f3`, catalog
`/var/lib/haco-saved-data-3210844539/state.json`. It observed rootfs, two Workspace
members and two managed data volumes, then passed ordinary deletion, independent
copy, source deletion, copied resume and exact-owned cleanup. This is provider and
service acceptance, not installed CLI, underlying Btrfs health, OCI runtime,
human approval or giant-repository performance acceptance. `snapshot-native-1`
was **SKIP** because the fixture opt-in was omitted; its exit zero is not a pass.

## Notification startup CI remains incomplete

[PR #682](https://github.com/SLktEx/Hacocoon/pull/682), `bd83251b`, passed the four
Linux workflows, but [Windows run 34946460934](https://github.com/SLktEx/Hacocoon/actions/runs/34946460934)
failed native review in job `104306922882`: activation creation HRESULT
`-2146959355`, no observed native progress. Prior installation, strict SSH/editor,
transfer, public reclamation and detached Workspace/OCI/snapshot restore passed.
The local installed review pass does not erase this COM activation failure or the
earlier dispatch timeout. #683 includes the change and needs its own acceptance.

## Base archive import candidate

At implementation `35a0c496`, `base-full-3` passed focused tests (23.08 s), full
main-diff lint (18.20 s), maintained full tests (33.10 s), staging/build/transfer/
lifecycle/API race tests (18.88 s), CLI E2E (4.51 s), docs (8.78 s), workflow policy
(1.57 s), CLI build (0.71 s) and native compilation (1.93 s). The added temporary
Workspace boundary regressions then passed with race detection (24.29 s), complete
diff lint (39.84 s) and docs (9.49 s). `base-full-1` failed the existing maximum
staging-budget regression; the extraction was corrected to preserve that valid
boundary. `base-full-2` passed focused tests but failed three new errcheck findings;
those were corrected. The formatter/final checks on the build WSL reported a
systemd root user-session startup warning while commands and tests succeeded;
that warning is not a new Windows acceptance pass.

On dedicated WSL `Hacocoon-Roadmap-f68a8c6b` / Incus 7.0.1, `base-native-1`
passed the actual `haco base import` CLI/controller stream in 165.94 s (test
165.89 s), private catalog `/var/lib/haco-base-import-1372522241/state.json`.
It exported its owned source rootfs, deleted the source, imported the archive
through an isolated temporary builder, published immutable Base
`sha256:6fbaf82f1e891f16d32da5186119908cd1499614018eeb2646f7828fd74986b8`, and used its
tool in a fresh Env. Normal exact-owned Env/image cleanup passed and the input
archive remained. This is native CLI/provider acceptance, not a packaged Windows
entry, authenticated Git, Packer dependencies or giant-repository measurement.

[PR #683](https://github.com/SLktEx/Hacocoon/pull/683), `bda75b67`, passed four Linux
workflows but [Windows run 34947135337](https://github.com/SLktEx/Hacocoon/actions/runs/34947135337)
failed job `104309115278` with the same COM activation creation HRESULT
`-2146959355` as #682, after public reclamation passed. These heads are not
qualified for main merge; local notification acceptance does not erase the failure.

## Notification ownership, readiness and cold controller

At `e043b740`, native Windows kernel-object regressions passed unpublished-owner,
closing-owner handoff and failed-start replacement. `lifecycle-local-2` passed the
Windows component suite (1.84 s). The display attempt initially failed at temporary
registry creation with access denied under workspace permissions; the same native
check with normal user registry access passed in 11.28 s (test 11.18 s), including
English/Japanese XML, history and removal. It did not observe human answers/layout.

`lifecycle-full-2`, covering follow-up `92ce27a5`, passed focused tests (11.02 s),
complete main-diff lint (21.31 s), maintained tests (35.57 s), private-client/review
race tests (15.74 s), CLI E2E (4.75 s), docs (10.72 s), workflow policy (1.81 s) and
native compilation (4.24 s). It checks delayed controller availability before
consuming a request, refusal without replay, and preserved failed pending reads.

Helper-only installed attempts `installed-review-1` and `-2` failed at private
readiness while the Linux installation remained `809bfb33`. Read-only diagnostics
then observed both successful pending reads and `pending_unavailable`, and a missing
controller socket during WSL startup. No permission, service or Policy override
was used to turn that failed read into success.

The ordinary Windows package installer updated dedicated WSL
`Hacocoon-Roadmap-f68a8c6b` and all companions to `92ce27a5` (local package build
37.42 s, version `v0.0.0-e2e`, unpublished). After terminating only that WSL,
`installed-review-3` passed actual registration/owned resume/idempotence, foreign
owner/mismatched activator refusal and stale/malformed request refusal. Its immediate
Host subscription check failed because the notification service was not yet active;
a later read observed enabled/active/running, zero restarts and success. This initial
startup observation remains an unresolved timing result, not an erased failure.
`installed-review-4` subsequently passed the same route, Host controller subscription,
absence of audit projection and owned listener cleanup. Human fresh answers remain
explicitly SKIP. These results do not establish a full CI pass or whole M0–M5 completion.

[PR #684](https://github.com/SLktEx/Hacocoon/pull/684), `a45e936f`, passed four Linux
workflows but [Windows run 34949250114](https://github.com/SLktEx/Hacocoon/actions/runs/34949250114)
failed job `104316004120` at `clear/timeout`, 20,013 ms, native progress `decode`,
after reclamation passed. [PR #685](https://github.com/SLktEx/Hacocoon/pull/685),
`d6c9fa13`, passed four Linux workflows but
[Windows run 34951609643](https://github.com/SLktEx/Hacocoon/actions/runs/34951609643)
failed job `104323633089` at `compact_attached`; Linux reclamation completed,
Windows compaction was not attempted, resume succeeded and notification acceptance
was SKIP. No failed head was merged. Earlier COM creation/dispatch failures remain.

## Env cache emptying

Candidate `2ca6af59c16b49d499c8c557f589934c1fcb33ad` adds reviewed cache emptying.
`empty-full-1` passed focused tests (19.93 s), full main-diff lint (40.76 s),
maintained local tests (102.05 s), race checks (50.65 s), CLI E2E (6.73 s), docs
(14.90 s), workflow policy (2.46 s) and native build (4.97 s). After adding direct
catalog snapshot/collection fences, `empty-final-guards` passed focused tests
(22.16 s), main-diff lint (13.25 s), race (27.00 s), docs (9.49 s) and native build
(2.15 s). Tests retain saved snapshots across a catalog reload, refuse another
snapshot while clearing, and require explicit same-owner recovery.

Earlier `empty-local-2` failed in newly added test fixtures: missing workflow
dependencies and using an in-memory confirmation reader as a nonterminal.
`empty-local-3` exposed the existing nonterminal refusal exit code 2 rather than
the expected 1. Corrected fixtures passed in `empty-local-4` (29.89 s; native build
2.31 s). No product permission or confirmation was weakened. Formatting also
reported a WSL root user-session startup warning while completing successfully.

On dedicated Incus 7.0.1, `empty-native-1` passed in 34.00 s (test 33.96 s),
fixture `data-e2e-ee7f06d38f566742`, catalog
`/var/lib/haco-data-placement-3790057143/state.json`. It exercised ordinary Env
creation/collection/reuse, reviewed single-area and all-Env emptying, nested files,
an outside Workspace symlink without traversal, sibling contents, common-source
retention, normal restart and canonical cleanup. The fixture's Workspace remained.
This is provider/lifecycle acceptance, not installed CLI, native OCI/snapshot
retention, human UI or giant-repository performance evidence.

The final catalog guards were included in `empty-native-2`, also PASS (26.36 s,
test 26.31 s), fixture `data-e2e-37c63c45b570dc31`, catalog
`/var/lib/haco-data-placement-605572226/state.json`, with the same bounded scope.

Parent [PR #686](https://github.com/SLktEx/Hacocoon/pull/686), `935c0752`, passed four
Linux workflows. [Windows run 34955347257](https://github.com/SLktEx/Hacocoon/actions/runs/34955347257),
job `104335936830`, failed at public reclaim with `compact_attached`: Linux stages
complete, stop requested, one open attempt, compaction not attempted, resume
succeeded. Notifications were SKIP. This failure remains distinct from local
notification acceptance; that head was not merged.

## Main integration and reclamation diagnostics

[PR #687](https://github.com/SLktEx/Hacocoon/pull/687) head
`21b2452b63cb58d86e9adc3cda546fc1c3214149` passed all five workflows and was
squash-merged to main as `2f421006d1ce86edbb5a1deb3da9c17c46a5ef5c`.
[Windows job 104346984027](https://github.com/SLktEx/Hacocoon/actions/runs/34958740591/job/104346984027)
passed ordinary entry, SSH/editor/tunnel, reclamation, detached Workspace/OCI/snapshot
restore and installed notification refusal/subscription/cleanup. Allocation fell
from 7,864,320,000 to 5,020,581,888 bytes (2,843,738,112 recovered), with 1 TiB
virtual capacity and 128 GiB pool capacity retained. Fresh human toast/UI answers
were explicitly SKIP. This later pass does not establish the cause of earlier
`compact_attached` or COM failures; their recorded results remain.

On the existing dedicated installation (`92ce27a5`), `ordinary-reclaim-1` failed
before a saved operation appeared (36.78 s from terminal start); the Windows saved
status remained `none`. No worker was retried or record cleared. A separate exact-GUID
systemd shutdown observation saw sharing error 32 after stop, then an unattached
disk at 81.30 s and successful same-GUID resumption at 108.41 s. This is not a
compaction attempt or a reproduction of CI's attached-disk observation.

The diagnostic candidate's `detach-local-2` focused tests passed (3.31 s); native
Windows suites passed for wslreclaim (0.77 s), reclaimclient (3.83 s) and haco-wsl
(0.45 s). Dedicated installed identity observation passed (5.54 s), followed by
existing enrollment/owner/file correspondence (5.78 s); no intent, trim or stop
was issued. Initial helper tests failed because two old assertions required empty
stdout on failure; they were updated for the bounded diagnostic receipt.
`detach-full-1` failed errcheck on the new display write; the return handling was
corrected. These checks do not establish a successful installed public retry.

`detach-full-2` passed focused tests (3.25 s), main-diff lint (5.27 s), maintained local tests (12.31 s), race (10.68 s), CLI E2E (2.88 s), docs (6.16 s), workflow policy (0.97 s) and native build (1.11 s).

Candidate `97ffa2d66c223ebced04b195bc1d409d59b43829` was built through normal packaging (21.54 s) and installed with matching Linux/Windows companions into the dedicated existing WSL (44.86 s), with installer doctor passing. `ordinary-reclaim-2` failed again (21.88 s from terminal start), now reporting preparation/enrollment, Windows code 2. The same target passes direct Windows enrolled-target observation. This route-dependent discrepancy is unresolved; no missing binding was recreated and no worker was replayed.

## Restored-tree comparison

Implementation `9f946abc561393141df5d0ef9a081ab1949f6b6f` adds portable tree
comparison using the existing read-only inventory. `compare-full-1` passed focused
file/real-tar regressions (0.48 s), maintained local tests (82.69 s), CLI E2E
(12.45 s), docs (18.00 s) and workflow policy (2.71 s). The six focused tests cover
real restored data, internal links, symlinks without traversal, xattrs, changed
content/mode/owner, incomplete reads, replacements and malformed manifests.

`compare-native-1` passed in 27.59 s. The preserved
`/home/codex-second/fixtures/workflow` tree (26 entries, 2,343 logical file bytes)
was captured with the existing GNU tar workflow into
`/var/tmp/haco-reviewed-capture-md4e11_9` on `hacocoon-second`. Its 51,200-byte archive
was retained outside WSL and SHA-256 checked, then restored into a new private tree
`/var/tmp/haco-reviewed-restore-8f9kdnr5/tree` on `Hacocoon-Roadmap-f68a8c6b`.
Linux scans and Windows-side portable comparison matched; original data, capture,
retained archive and restored tree remain. Numeric owners were compared in the raw
Linux Host namespace. This does not establish guest idmap correspondence, full
current-data migration, authenticated development or large-repository performance.
The initially inspected network-final tree was empty and was not used as evidence
of restored file contents. Old-version reconstruction/replacement remains out of scope.

The retained installation inventory queried all native resources successfully:
12 instances, 48 custom-volume entries and two images in `hacocoon`, plus one
separately owned cache volume. Ownership review, manual data classification and
whole-backup completion remain distinct from this read-only inventory.

For the independent installed-reclamation investigation, direct and trusted-Host
Windows reads used the same owner hash/64-bit process but saw different enrollment
visibility. The first terminal probe timed out on a cursor-position query; the
corrected read-only probe observed the discrepancy. A proposed maintenance WSL
restart was NOT performed because its guard found an unrelated running
`Ubuntu-24.04`. No global shutdown, binding overwrite or data deletion occurred.
The cause of this execution-context difference is still unestablished.


<a id="reclamation-language"></a>
## Reclamation language

Implementation `99522ebd893e7fbdc0752e3db3d6f84595f10fb6` translates ordinary
reclamation status, capacities, failure guidance and reviewed interruption through
the shared catalog. Focused tests9.45s, changed-code lint19.86s, maintained local
tests43.65s, race14.50s, CLI E2E4.05s, docs13.30s, workflow1.78s and native-build1.85s
passed. Japanese tests preserve exact operation/target/state consent and raw
protocol values. Public-journey presentation recognizes both languages; completion
still requires the original machine receipt. Native installed language acceptance
is recorded separately. The first formatting attempt refused a concurrently edited
source by hash check; a fresh archive was formatted before these tests.

Parent #688 passed all five exact-head workflows at `4e0a483e` and was squash-merged
to main `ee8bf7fb`. This does not resolve the dedicated local enrollment observation
failure or prove fresh human GUI answers.

The exact implementation was packaged through the normal installer tooling51.68s
and installed60.72s with matching Linux/Windows binaries; all doctor checks passed.
Ordinary Windows→WSL→Host Japanese help and read-only saved-result display passed
45.28s with status exit0 (no saved operation). An initial private observer expected
the internal fallback help instead of the shared vertical help and failed its
assertion despite successful product output; corrected before the second run.
WSL reported a managed-user systemd-session warning while Host entry completed;
its cause remains uninvestigated. This read-only success does not establish a new
reclamation start/compaction or fresh human GUI answers.


At restored-comparison head `379f0b156e853d81b00a2933dbd6983f00718cc9`, Windows
[run34964494309/job104365643144](https://github.com/SLktEx/Hacocoon/actions/runs/34964494309/job/104365643144)
passed installation, HTTPS, interop, SSH/editor/forwarding and Linux reclamation.
Public reclamation failed: both Linux stages complete; Windows stop requested,
open attempts1, compact_attached, compaction not attempted and resume unsuccessful.
Notification review was skipped. Four Linux workflows passed. This head was not
merged; #690 includes its code and merges the identical parent main tree without
changing candidate file contents. The Windows failure remains unexplained.

On installed `99522ebd`, a third dedicated ordinary Japanese reclaim attempt failed
after91.11s at prepare/enrollment with Windows error2. Read-only status was successful
and still returned none afterward; no new operation record, stop, compaction or
automatic replay was observed. The new diagnostic presentation worked, while the
previous enrolled-record visibility discrepancy remains unresolved.


<a id="supported-dns-modes"></a>
## Supported Incus DNS modes

With installed product `99522ebd893e7fbdc0752e3db3d6f84595f10fb6` on dedicated
Hacocoon-Roadmap-f68a8c6b / Incus7.0.1, the existing three-mode native regression
passed24.68s (test24.61s): host10.31s, backend8.09s, disabled6.20s. It created fresh
owned Envs, checked resolver service/configuration across stop/resume and backend
resolution, then completed canonical owned cleanup. Catalogs remain at
`/var/lib/haco-dns-modes-2079785240/state.json`, `-3117962770/state.json` and
`-811325797/state.json`. No Policy/configuration or dependency overrides were added.
This supplements the earlier6.0.5 evidence, not guest Policy-query/Windows DNS
tunneling/VPN/NRPT or giant-repository acceptance. The first wrapper used an invalid
Incus info flag and stopped before the test; the maintained query /1.0 interface
was used for the successful supported-server observation.

The M0–M5/status consolidation passed the maintained local docs check39.09s and
subsequent link checks. It removes stale candidate diaries, preserves unique failure
evidence and updates paired owning contracts; no product code or checkpoint changes.


At #690 head `b6dec8807e026bf9c765db0af16eea238186da06`, four Linux workflows passed,
but Windows [job104373589771](https://github.com/SLktEx/Hacocoon/actions/runs/34966961367/job/104373589771)
failed the changed-host-key assertion in test_windows_environment_ssh.ps1:554.
Earlier SSH/editor, approval Webview stale refusal, saved-choice review, setup,
preview, imported/recreated work and Windows tunnel checks passed. Linux/public
reclamation and native notification review were skipped. The assertion combines
exit status, unexpected stdout and missing host-key diagnostics; the log does not
establish which predicate failed. Subsequent Policy cleanup/disconnect reported
WSL Catastrophic failure. This is neither a proven host-key bypass nor a successful
refusal test. Keep this head unmerged pending diagnosis; do not relabel it using
the earlier #687/#688 successes.


## Windows host-key refusal diagnosis

The follow-up to #690 preserves the three existing host-key refusal requirements
but records which one failed, with existing allowlisted SSH progress. It recognizes
the NUL-interleaved WSL E_UNEXPECTED seen in job104373589771 without emitting raw
child output. Local PowerShell regression and real child timeout/nonzero checks
passed. This improves evidence for the next ordinary run; it does not establish
a cause or resolve that prior failure. No extra permission, restart or retry is added.

## Snapshot restore by environment name

Implementation `e1ec0894` adds `snapshot restore --latest <source-env> [new-env]`.
Local focus 11.21s, changed-line lint 13.35s, maintained test entry 30.53s,
race 14.92s, CLI 3.78s, docs 8.23s and workflow policy 1.41s passed.
Capture time survives real catalog reload; CLI/controller tests cover unordered
saves, incomplete/other-Env exclusion, date ties/unknowns, failed listing and no
fallback after a selected restore fails. The initial unchecked diagnostic-write
lint failure was corrected before the successful run. Checkpoint identity and
maintained docs checks also passed after the v0.68 update.

The ordinary ten-binary package built in 34.67s. Installed/new native acceptance
is recorded separately; these local tests alone do not prove Windows SSH or OCI
restoration. Prior Windows failures and the Packer permission question remain open.

On dedicated Ubuntu26.04.1/Incus7.0.1, the normal package installed in 39.73s and
`doctor` passed. The installed CLI cloned this repository, created a managed
Workspace and an Env without OCI, captured two different marker values, deleted
the source Env, and restored by the deleted source's name. Saves took 4.19/4.27s;
restore took 5.53s and returned a running Env with the second marker. Both saved
records remained unchanged. Only marker writes/reads used Physical-Host Incus
exec; all lifecycle/capture/restore operations used the normal installed CLI.
This does not establish a desktop SSH/OCI journey or giant-repository performance.
The exact fresh Env, two saves, both Workspaces and source registration were then
cleaned through ordinary product commands; all six cleanup operations passed.
Native receipt: `latest-b61bbc62`, restored Workspace `restore-5dbfe05afd5db0c4`,
source commit `e1ec08947e8ae2c7db5d0251f246cdc9403446a5`.

## Windows integration retry and interop observation

PR #692 head `4e7a45a75047c8d372da1ab889eb7d2796f4370b` passed four Linux
workflows. Windows34970515521/job104385385746 failed VS Code extension installation
with HTTP503. Ordinary SSH, cold parallel sessions, changed-host-key refusal,
DNS/Policy, setup, preview, transfer and restored work passed. Linux/public
reclamation and native notifications were skipped. The failed Windows jobs were
retried once for that external dependency failure; there is no successful merge
claim and the earlier unexplained #690 failure remains unresolved.

The local enrollment visibility difference survived restarting only the dedicated
WSL after confirming no ordinary Envs or reclamation operation. Doctor passed.
Windows direct and the physical WSL's normal invocation see enrollment; both
Physical Host and trusted Host using `/run/WSL/1_interop` do not. All observed
processes reported the same Windows owner hash, 64-bit execution and no package
identity. This isolates the observed difference to the init interop route, not to
an absent enrollment or only the Incus boundary. Its underlying Windows cause is
unproven. No registration/history mutation, socket substitution in the product,
global WSL shutdown or extra reclamation attempt was made.

## Native WSL interop registration recovery

On installed candidate `e1ec0894`, the subsequent native interop observation failed before querying the registry: the
WSLInterop binfmt handler was absent. Ordinary installed `haco setup` restored the
WSL-owned registration, and a Windows executable printed the expected marker.
This recovery does not establish a successful reclamation start.

## Incremental ordinary Git history

Development implementation `6088e6c542ffb9b13e0a2b3650c4d992c8a57435`
([#695](https://github.com/SLktEx/Hacocoon/issues/695)) passed local focused tests
11.52s, lint 24.76s, maintained full tests 29.90s, Git race tests 36.27s, CLI 9.34s,
docs 9.28s, workflow policy 1.38s and native test compilation 1.75s.
The real-Git component regression uses 34,603,008 bytes of existing random data:
fetch transferred 293 bytes and push preparation 319 bytes for its small changes.
Preparation left the remote unchanged. Existing broker approval tests passed.

The first run found that embedded `bytes.Buffer.ReadFrom` let subprocess output
bypass the capped writer. Removing that embedding made the full-history limit
regression and the actual pipe-copy regression pass. This is a local 33 MiB
functional result, not installed Incus/Windows/authenticated Git acceptance or
representative giant-repository performance. New-target and large-new-pack limits
remain; this fix does not advance the v0.68 checkpoint or publish a release.

Windows retry job `104394453906` for #692 head `4e7a45a7` passed SSH/editor and
Linux reclamation, then failed public reclamation with `compact_attached`:
stop requested, one open, no compaction attempted, same-target resume successful.
Native notification was skipped. The earlier HTTP503 and unexplained failures
remain independent; this head is not approved for main integration.

## Virtual-disk observation handle lifetime

Development implementation `f50c0d93445f3f6f427b0e301294e5101f94a65e` closes an
attached observation handle before its bounded wait, while retaining file/parent
pins. [ADR 0103](../adr/0103-virtual-disk-observation-lifetime.md) records the native
API contract and distinguishes observation lifetime from ownership.

Local focused tests 9.91s, lint 17.21s, maintained full tests 39.95s, reclamation
race tests 1.61s, CLI 4.96s, docs 13.49s, workflow policy 1.99s and native test
compilation 2.18s passed. Windows packages passed (reclaim 0.80s, client 6.09s,
helper 0.47s). Windows-specific lint first found an unchecked test handle close;
after fixing it, lint passed in 1.82s and focused native regression in 1.22s.
The native empty-disk attachment fixture was **SKIP**, with
`ERROR_PRIVILEGE_NOT_HELD`; no elevation or permission workaround was applied.
Other dedicated-disk native checks were not enabled. These results do not prove
installed public reclamation on the new candidate.

A separate read-only observation on installed `e1ec0894` requested poweroff of
`Hacocoon-Roadmap-f68a8c6b` after empty-Env/absent-operation checks. Native open
kept returning sharing violation for 90 seconds; same-target resume succeeded
at 90.94s. It never obtained the held handle needed for the intended comparison.
The systemd journal recorded intervening startup, and a separate Windows process
was running bash in the target. Its ownership/use is awaiting clarification;
no process was killed. This attempt is inconclusive about handle lifetime and
separate from CI #692's `compact_attached` result. Existing failures remain.

The normal ten-binary installer package for #697 head `b0b2fcbc` built in 34.90s.
It has not been installed over the dedicated WSL while concurrent use is unresolved;
installed `e1ec0894` and its existing data remain. This is packaging, not public
reclamation acceptance.

## New-branch Git history reuse

Development implementation `e17e5132e0ce9769a7c6446ac496797baa7df209` extends the
existing-history fix to ordinary new-branch preparation. The real-Git component
regression used 34,603,008 bytes of existing random data and sent a 322-byte pack
for its small new-branch change. It kept the expected-absent target and left the
remote unchanged. Ref read denial, movement and mismatched confirmation stopped
before push; existing ordinary broker approval regressions passed.

Focused checks passed in 13.70s, then final focused 13.85s, lint 24.54s, maintained
full tests 40.22s, Git race 33.37s, CLI 4.34s, docs 11.65s, workflow policy 1.72s
and native test compilation 1.69s passed. This is component/real-Git evidence,
not installed authenticated Git, a main merge, a release or giant-repository
performance. New pack data above 32 MiB remains unsupported. The v0.68 checkpoint
is unchanged. See [ADR 0104](../adr/0104-new-branch-git-history.md).

## Integrated candidate Windows tunnel failure

#697 head `b0b2fcbc` passed quality `34979869527`, test `34979869379`, Ubuntu
`34979869467` and Incus `34979869532`. Windows `34979869494`, job `104417065184`,
failed after installed SSH/cold reconnect, actual VS Code editing, approval
webview refusal, saved request decisions, preview and Windows-projected transfer
with restored work/recreation had passed. The ordinary tunnel's native listener
was confirmed, but its eight-client exchange received Windows reset `10054`;
the application fixture also timed out in `accept`. The log does not establish
whether its 40-second application readiness budget, stream preparation or another
condition caused the reset. Do not call this a proven product or fixture cause.

Linux/public reclamation and native notifications were **SKIP** in this run. The
readiness fix has not reached installed reclamation acceptance. Main remains
`ee8bf7fb`; no retry or merge was made after this failure. Earlier failures remain.

The #698 package at `023ca03e` built all ten binaries and normal installers in
40.22s; installation is pending concurrent-use clarification. Read-only Host
`gh auth status` in the running dedicated WSL confirmed no GitHub login. The user
was given ordinary Host login instructions; no credential was printed/exported,
and authenticated Git acceptance remains unperformed.

## Installed lifecycle lock collision and read-only Windows readiness

With installed product `e1ec0894`, ordinary public clone and managed Workspace
creation passed in the dedicated Incus 7.0.1 WSL. Env `tunnel-b60c7032` creation
failed before provider creation: the Physical Host controller (effective UID 0)
rejected `/tmp/hacocoon-environment-locks`, owned by UID/GID 1000 at mode 0700.
The Workspace lock directory had the same owner/mode. The actual protected
`/var/lib/hacocoon/state` directory is UID/GID 0, mode 0700. The first observation
inside trusted `haco-host` found neither temporary directory; the controller runs
on the Physical Host, where the subsequent read-only observation found both.
No ownership or permission change, deletion, retry, reinstall or restart was used.
The exact new repo `tunnel-b60c7032-repo` and Workspace `tunnel-b60c7032-work`
remain retained; Env status was not-found, which alone is not provider-absence
proof for cleanup.

A read-only Windows client built from `e34c2bf8` used the installed registered
control route and its normal ten-second preparation budget. One ping completed
in 46 ms; eight parallel pings completed in 67–101 ms. This confirms control
readiness only, not actual TCP data forwarding or the cause of #697.

The catalog-lock correction first failed component tests because existing state
parents may be owner-owned, non-writable by others, but readable at mode 0755.
The corrected design pins that protected parent and creates an owner-only child
for locks; it does not chmod the parent or permit other users to write it. State
and Workspace component regressions then passed in 8.86s. Installed acceptance
of the correction remains pending. Human login/notification/VS Code answers are
post-release acceptance, not a main-merge gate; prior CI failures remain failures.

Final local validation of the catalog-lock implementation passed: focused state/Workspace 9.55s, changed-code lint 18.78s, maintained full test entry 30.88s, race 12.11s, CLI E2E 4.18s, docs 8.50s, workflow policy 1.39s and native Incus test compilation 1.94s. After the later documentation/phase-recording edits, Windows-side documentation consistency, fixture syntax and diff checks also passed. The WSL root systemd-user-session warning was observed again; it was not repaired or counted as resolved. No installed acceptance or authenticated/GUI interaction is implied.

## Catalog-lock candidate CI and package

At #699 head `1ae5b410`, quality `34986231603`, test `34986231525` and Ubuntu
`34986231600` succeeded. Incus `34986231453` passed the standalone and Core jobs
but failed Btrfs job `104438909790` after all aggregate save/export/import/restore,
copy, retained Workspace/OCI and owned provider deletion checks passed. The final
fixture-directory cleanup rejected the newly persistent `lifecycle-locks` child.
This remains a failed run, not complete Incus acceptance. The fixture correction
retains this directory/inodes, removes completed recovery files only after a full
entry preflight, and still refuses unknown directories or symlinks. It does not
change provider-absence checks or product locking.

The normal ten-binary Linux/Windows installer package built from exact `1ae5b410`
in 69.93s. It was not installed or published; no running WSL was stopped.

## Sequential multi-head Git fetch

Commit `45555463` removes the helper's aggregate-size rejection while retaining
the 1024-head and 32 MiB per-response limits and separate exact-ref authorization.
Each response is indexed before the next request. A real Git regression with two
independent 17 MiB random-content branches failed on the previous implementation
(`git batch exceeds supported pack size`, 18.41s command) and passed after the
correction (9.69s command). Both commits' content was available after indexing;
the two packs totalled 35,662,881 bytes and each stayed below 33,554,432 bytes.
The initial test invocation had a host-shell parse error and did not run; the
recorded failure/pass came from corrected invocations of the actual regression.

Final local checks passed: Git regressions 18.23s, lint 28.19s, maintained whole
test entry 47.06s, race 42.43s, CLI 5.57s, docs 9.77s, workflow policy 2.06s and
native test compilation 1.72s. This is functional component evidence, not huge-repo
performance or authenticated installed Git acceptance. Single packs over 32 MiB
remain unsupported. `d4c264a3` separately corrected the Incus fixture cleanup,
with focused regression 23.41s (test execution 0.041s) and lint 17.36s passing.
Both changes were integrated locally without conflicts at `5980d18f`.

A focused Windows process regression also reproduced loss of completed stdout/stderr phases when the native acceptance wrapper timed out. The correction keeps bounded output on the timeout exception and emits it before failing, without increasing the 30-minute deadline or weakening required markers. The original regression failed with empty captured output; all seven native-runner tests then passed in 1.671s on Windows. This does not identify the running #699 Windows job's cause or establish its success. The integrated `77a4c8cc` normal ten-binary package built in 31.96s without installation or publication.

## Windows acceptance of the catalog-lock integration

#699 head `1ae5b410` passed Windows workflow `34986231470`, job `104438908869`
and evidence job `104450106625`. Normal installer/restart/reinstall, egress/DNS
refusal, cold parallel SSH, actual VS Code 1.136.1 editing and terminal, saved
project setup, reviewed requests, preview, export/import and retained-data
recreation all passed. The ordinary native TCP tunnel passed eight 1 MiB
half-close round trips and Ctrl+C listener cleanup. Its phase record shows
application ready at 234ms, Host ready at 26,234ms, listener at 27,405ms, native
owner confirmed at 29,875ms and exchange completed at 31,969ms. This successful
run does not explain #697's earlier reset/accept-timeout failure.

Installed Linux reclamation and the public Windows worker completed. Windows
allocation fell from 7,730,102,272 to 4,965,007,360 bytes (2,765,094,912 recovered),
virtual capacity remained 1,099,511,627,776 bytes, and the same WSL resumed.
Compaction was attempted once after 320 bounded open observations. The Host
sentinel, detached Workspace/OCI and snapshot restore passed after reclamation.
Native notification registration/ownership/stale-malformed refusal/subscription
checks passed. Human toast/fresh GUI decisions and VPN/NRPT were explicitly
skipped and remain post-release acceptance; no authenticated Git claim is made.

Quality, test, Ubuntu and Windows are successful for this head, but Incus remains
failed at the completed fixture's persistent lock directory described above.
Follow-up `d4c264a3` fixes that fixture locally. Main is still `ee8bf7fb`; this
receipt does not authorize merging a different unverified head.

## Setup outcome language

`e5a4e1e8` moves Host/project setup outcomes and next actions into the shared
English/Japanese catalog and reuses vertical help. Local checks passed: focused
8.62s, lint 18.14s, whole test entry 34.72s, race 13.73s, CLI 4.84s, docs 8.58s,
workflow policy 1.91s and native-test compilation 1.55s. Final lint including new
files passed in 22.42s; all nine approval-runner regressions passed on Linux in
0.57s. Windows ran the six applicable runner tests, with three Linux terminal
checks explicitly skipped (0.021s test time). The runner still requires the exact
success marker and exactly one valid English/Japanese completion line.

An initial harness invocation failed before tests because Linux Git could not
resolve Windows worktree metadata; the patch is now generated on Windows.
The first focused run caught double-formatting of Host result fields, which was
corrected before the successful suite; the first changed-line lint also rejected
unchecked output returns. Existing exit status, script output, private-result
access, explicit replay and structured diagnostic boundaries are preserved.
This is repository/component evidence, not a new installed or human-GUI pass.

## Main integration of retained-data and lifecycle fixes

[#699](https://github.com/SLktEx/Hacocoon/pull/699) merged as main
`e4d99700b976e2166a4a0b27dc9f37cf3aaaacc1`. Its exact head `51ba4f24`
passed quality `34989824960`, test `34989824791`, Ubuntu `34989824936`,
Incus `34989824882` and Windows `34989824868`. The merged tree is identical
to that head. Core, standalone, Btrfs and evidence jobs all passed, including
the aggregate fixture cleanup that failed at the earlier head.

Windows job `104451279682` passed ordinary installation, Environment HTTPS and
direct-egress refusal, native entry/SSH/editor/forwarding, Linux and public
reclamation, retained Workspace/OCI/snapshot restore and native notification
ownership/refusal/subscription. Public operation
`{68E10593-EC50-41D2-875E-F71D4F303386}` recovered **2,840,592,384 bytes**:
Windows allocation 7,797,211,136 → 4,956,618,752; virtual capacity
1,099,511,627,776 was unchanged. Compaction completed after 252 open attempts;
the same WSL resumed. Human toast clicks/fresh GUI decisions and VPN/NRPT
remain skipped. Earlier failures and the dedicated local enrollment issue
remain distinct; success does not establish their causes.

The setup-language follow-up was rebased without a tree difference from
`724adc2d` onto this main (`e5a4e1e8` → `8b95f79a`). Its normal ten-binary
Linux/Windows package built from `724adc2d` in 45.49s, without installation,
WSL termination or publication. These scopes do not claim a new release.

## Setup candidate Windows reclamation failure

At #700 head `5e2ee17bcdc6e2ee36766f00ed2877485777e2c8`, quality
`34993511402`, test `34993511349`, Ubuntu `34993511337` and Incus
`34993511396` passed. Windows `34993511328`, first-attempt job
`104463877953`, failed public reclamation: both Linux stages completed and the
Windows stop request succeeded, but 359 observations exhausted the existing
90-second detached-disk wait. `compact_attached` prevented compaction; the same
WSL resumed. Native notification acceptance was skipped. Ordinary installation,
SSH/editor/forwarding and the preceding installed Linux reclamation passed.

The cause is unproven. The setup change does not modify shutdown or compaction.
One failed-jobs rerun was requested for this exact head to check reproducibility;
it cannot erase the first attempt. No timeout, disk-attachment check or WSL-wide
setting was relaxed. Person-dependent acceptance remains post-release.

<a id="tcp-listener-cancellation-close"></a>

## TCP listener cancellation waits for closure

PR #748 head `9172e2bbe3337e85c53a25187e154d7bdd1568ec` failed
[test run 37935376919 / Go 1.26 job 113835981590](https://github.com/SLktEx/Hacocoon/actions/runs/37935376919/job/113835981590)
on 2026-10-09. `TestNetworkListenerValidatesLoopbackAndClosesOnCancellation`
returned from the command, then failed the immediate exact-address TCP rebind
with `address already in use`. The Workspace package, including the new
process-recovery cases, passed in that same job. This failure is retained;
the head was not rerun to obtain a pass.

A separate local follow-up based on `7a3a6b8b33b0265f2e79d46015836739542680c6`
reproduced the ownership gap with a channel-backed listener and Go `testing/synctest`:
`Accept` can unblock before an asynchronous `Close` finishes. The previous
`ServeTCP` stopped the callback without joining it; another caller's `Close`
could return while the first still held the socket. Three deterministic cases
failed on unchanged main and passed after joining the callback or closing
synchronously when cancellation preceded its start. Non-canceled accept errors
and invalid-listener refusal preserve caller ownership. No timing sleeps or
real sockets select the injected failure.

The local candidate passed the complete network package and unchanged CLI
listener/rebind tests on Go 1.26.7 and 1.27.2, plus focused race, repository vet
and differential lint. The strict immediate-rebind assertions, product deadlines
and retry policy are unchanged. This is component evidence; the corrected
published head and native provider/Windows acceptance remain separate.

## Network command language

`0387258d` adds shared English/Japanese network result and next-action messages.
Focused CLI/catalog/relay tests passed in 11.76s, changed-code lint in 26.63s,
the maintained full test entry in 44.33s, race in 15.72s, CLI E2E in 4.40s,
docs in 10.50s, workflow policy in 1.58s and native-test compilation in 2.00s.
Tests compare JSON across languages, preserve exact revocation and ask-rule
scope/expiry, keep unrelated Git approval rules and default denial while editing
Host registrations, and retain original error details. Result-write failure
does not replay the mutation. Actual Linux loopback TCP/UDP listeners were
opened, canceled and rebound at their exact emitted addresses without opening
an upstream connection. This is local CLI/relay evidence, not installed network,
external-service, VPN or human UI acceptance. Catalog selection does not change
structured log fields or the underlying Policy implementation.
The ordinary ten-binary Linux/Windows package built from #701 head
`f354464337f84e876a97628ded69249fa84f13f6` in 45.28s. No installation or
release was performed. #701 is a development-branch follow-up to #700.

## Configuration language guidance

`c3fb376f` adds bilingual configuration help, inspection/save guidance and
unconfirmed-save/retained-editor notices. Local focused tests passed in 10.64s,
changed-code lint in 15.93s, maintained full tests in 36.01s, race in 13.90s,
CLI E2E in 4.23s, docs in 9.80s, workflow policy in 1.37s and native-test
compilation in 1.71s. JSON inspection/apply receipts are byte-identical in both
languages; exact Policy and revision inputs, conflict retention, original errors
and one apply on output failure are covered. The first lint attempt found five
unhandled diagnostic writes; they were made explicit before the full pass.
These checks do not change a live Policy or establish installed/human acceptance.

## Forwarding fixture readiness correction

The one #700 retry at the same head also failed: Windows run `34993511328`,
job `104476272439`, stopped in ordinary native TCP forwarding. The application
was ready at 187 ms, the Host at 26,577 ms, the listener at 28,015 ms and native
ownership at 38,859 ms. Failure at 41,375 ms included application `accept`
`TimeoutError` and a mismatched binary response. The fixture's 40-second accept
wait started before Host preparation, so almost all its budget was consumed
before the actual exchange. Subsequent Linux/public reclamation and notification
steps were skipped; evidence job `104483624401` failed. This neither resolves
nor erases the first attempt's separate `compact_attached` failure.

The #701 follow-up arms the shared application only after listener readiness
and Windows native-owner confirmation. An independent 180-second startup bound
fails on missing readiness; the original 40-second accept budget, eight binary
exchanges, half-close, product lifetime and cancellation assertions remain.
The real-process/TCP regression delays preparation beyond the accept budget, then
checks all eight exact responses; missing clients, no arming and EOF still fail.
It runs in the maintained local and repository CI entries.

Local validation passed: application regression 1.80s, Windows observer regression
1.44s, focused tests 11.36s, changed-code lint 34.51s, full tests 96.28s, race
15.82s, CLI E2E 4.50s, docs 9.54s, workflow policy 1.47s and native-test compile
1.73s. This is component evidence; the updated installed Windows journey remains
pending. #701 contains #700's exact head and is the combined main candidate;
#700 stays open until that integration is proven.

## Streaming Git pack candidate

The development candidate removes whole-pack JSON/base64 buffering on the helper
and trusted-agent boundaries. A 40 MiB random-data ordinary Git fixture fails on
old product `e4cbd257` during `git pull --ff-only` (10.02s command / 9.19s test).
Only the large-data fixture was added to that archived product; its transport and
Policy were unchanged. The new implementation passes the same ordinary pull and
separate denied/approved fixed-content push journey with 40 MiB new data in both
directions. The Host-agent framing is exercised through pipes as well as the
actual Unix HTTP broker; this is not a native Incus or authenticated remote test.
A separately measured single fetch pack transferred **41,956,043 bytes**.

Local final checks passed: Git/Incus component tests 38.62s, changed-code lint
15.73s, maintained full tests 56.13s, Git race 71.11s, CLI E2E 6.89s, docs 12.96s,
workflow policy 2.12s and native compilation 1.84s. The first compile found one
remaining reconciliation reference to the removed byte-slice field; the first
full lint found seven unchecked closes and thirteen style findings. All were
corrected before this full pass. Follow-up documentation checks also pass.

Regressions retain exact-ref read decisions, push denial, changed-local-content
approval pinning, new-branch expected-absent leases and incremental history reuse.
Malformed frames, excess lengths, missing EOF/receipts, trailing bytes, wrong byte
counts and output failures remain failures. Forty MiB streaming does not prove
representative giant-repository speed/capacity; those measurements and installed
provider acceptance remain separate. See [ADR 0106](../adr/0106-streaming-git-packs.md).

## Main integration of setup, network and configuration guidance

[#701](https://github.com/SLktEx/Hacocoon/pull/701) merged as main
`f225e5c1a005358929a5c3bfe2b5154cc55a4ec6` after exact head `161f3854` passed
quality `35000716746`, test `35000716642`, Ubuntu `35000716870`, Incus
`35000716637` and Windows `35000716814`. The merged tree matches that head.
#700's exact head is an ancestor and its PR was closed as integrated.

Windows job `104488129559` and evidence job `104497872650` passed. Ordinary
installation/re-entry, HTTPS/direct-egress refusal, native SSH/editor/TCP, Linux
and public reclamation and native notification review routes passed. The forwarding
application was armed after native ownership at 26,202 ms. Public operation
`{40F8CDCB-AE50-4141-BC3B-F5A1A64B2E14}` recovered 2,683,305,984 bytes:
Windows allocation 7,629,438,976 → 4,946,132,992, unchanged virtual capacity
1,099,511,627,776, 255 open observations, completed compaction and same-WSL resume.
Human toast clicks/fresh GUI answers and VPN/NRPT remain skipped and post-release.
Earlier attached-disk failures remain distinct; this success does not prove their cause.

The following Git implementation `68135b19` is tree-identical to `ace86ee4`
after rebasing onto this main. The latter's normal ten-binary Linux/Windows
package built in 42.32s. No local installation, WSL termination or publication
was performed. Product code also matches the full-tested pre-rebase `9c9d2ed0`;
four documentation conflicts preserved both independent evidence sections.

### Named current-data selection

Development implementation `fe23cf539a8f27e56d8357354540c9a7f8cc5488`
([#703](https://github.com/SLktEx/Hacocoon/issues/703)) passed seven local Linux
selection regressions, including two real copied trees with Git metadata, dirty
files and symlinks, then independent changed and unrestored selections. It reused
the existing scanner/comparer; originals remained unchanged. Windows passed six
portable selection tests; the Linux filesystem case was explicitly skipped there.

The Linux commands passed: selection 0.24s, existing tree comparison 0.25s,
maintained full local test 84.69s, docs 18.25s and workflow policy 1.98s. A WSL
root systemd-user-session warning preceded these checks; no service repair or
permission change was used. Final paired documentation passed `check_docs.py`.
This is selected local filesystem and repository evidence, not installed Incus,
actual operator inventory completeness, independent retained storage, owner-idmap
equivalence or authenticated editor/build/OCI/Git acceptance. The helper never
grants deletion authority. Person-dependent checks remain post-release and do not
block main integration after the required exact-head CI succeeds.

### Git streaming main integration and selection rebase

Main `6cdfe5d02bd1531da37ecd08c8c1e91134498c4e` integrates #702, with the same
tree as head `821cecb6dae26efacb2ca3c77dc20e20e8c02c8c`. Quality35004194444,
test35004194464, Ubuntu35004194454, Incus35004194621 and Windows35004194439
all passed. Windows job104499736879 passed installed SSH/editor, Linux/public
reclamation and native notification review; evidence job104509492624 also passed.
This does not establish actual large Git through Incus or fresh human answers.

#704's selection implementation is rebased as `db1ac9aafc1f0833898d21e5609be056bd38754e`
with no changes to selection code/tests, local CI or workflow from `fe23cf53`.
Documentation conflicts preserve both independent results. Earlier #704 head
`9a8a7d07` had four successful workflows and Windows still running when replaced;
these are not proof of the new combined head. The new exact head needs its own CI.

### Named Base builder

Implementation `f79744c38445021f5498e70ed87956669e3345da` adds optional builder
names through CLI, controller and canonical Base orchestration. The normal Env
name validator is shared; each repeated name gets a fresh temporary Workspace.
Creation failure does not trigger cleanup, while uncertain publication and failed
cleanup preserve their named target. No network rule is changed or approval granted.

Local checks passed: focused Core/Base/workspace/controller/CLI/Packer tests13.09s,
changed-code lint19.97s, full test65.21s, related race18.46s, CLI E2E5.68s,
docs11.01s, workflow policy1.68s and native-test compile1.84s. After incorporating
#704 unchanged, selected-tree tests0.11s, comparison0.13s, combined full test76.30s,
docs10.65s and workflow policy1.43s passed. The same commit generated the normal
Linux/Windows ten-binary installer candidate in35.38s, without installation or release.
The generated `haco base build --help` also passed in English and Japanese, including the named-builder option.
The WSL root-user-session warning remains; no service or permission repair was used.

This is repository, local filesystem and package-build evidence. The earlier
ordinary Packer dependency HTTP403 remains unresolved until reviewed scoped settings
and actual download/build/publication/reuse succeed. The proposed all-Environment
rule remains unapplied after automatic review refusal; named builders introduce no
exception. Human approval/login acceptance remains post-release.

On 2026-09-16, a bounded read of the [official Packer 1.16.0 distribution](https://releases.hashicorp.com/packer/1.16.0/) checksum list and
ZIP central-directory metadata matched both pinned checksums in `prepare.py`.
The amd64 archive/executable sizes were 34,785,580 / 108,318,882 bytes; arm64
was 31,509,969 / 100,663,458 bytes. Both fit the existing 128 MiB limits. This
checks distribution metadata only: no Packer executable was downloaded in full
or run, and the earlier guest dependency HTTP403 is not resolved by this result.
Read-only inspection found another active Go process in the dedicated local WSL
and about 11 GB free on C:. Its installation, services and Policy were left intact;
no new distribution, restart or broader communication rule was used for acceptance.

### Repeated Windows detachment refusal and process observations

The following later Windows public reclamation attempts failed with
`compact_attached`, after successful Linux stages and stop requests. Each made
359 native open attempts without starting compaction. Ordinary installation,
HTTPS, SSH/editor/forwarding and the separate Linux reclamation gate passed;
the later notification gate was skipped.

| Exact head | Windows run / job | Same-target resume |
|---|---|---|
| `9a8a7d07f93677690c04cb6124c8d78bb5373965` (#704, before rebase) | 35006335176 / 104506895607 | failed |
| `4c94462ffd6e42d5f1e831cff41224fe051021ba` (#704) | 35008285127 / 104513489162 | passed |
| `c0692cdf327be7f809333c2a9ea23753bd1f5661` (#705, before documentation update) | 35009099013 / 104516251265 | passed |
| `428b2bdb53b1070ec4d870bf7589778f29f90042` (#705) | 35011032772 / 104522727488 | passed |
| `38aeae56ed769b41e59c0d9e89859f2f7e50c52e` (#706, counts only) | 35012950293 / 104529173573 | passed |
| `c3a2cc8678cdfc4e396d6ac0699eb1cca9d0c626` (#706, second attempt) | 35016859540 / 104544769661 | passed |

Earlier successful allocation recovery remains valid within its recorded scope;
these later failures remain unresolved under #381. Virtual observation handles
are closed between attempts, the public driver never reenters WSL before worker
completion, and its separate reader continues draining ConPTY. Existing receipts
cannot distinguish Linux shutdown delay, WSL detachment delay or another restart.

Implementation `356b8c509107aa0f37eec5ce04aefe274b604e44` adds Windows-only CIM
process counts to the public test's existing worker observation. Only changed
counts, elapsed time and fixed scope/state fields are logged. Counts describe all
visible WSL processes, not selected-distro ownership or detached-disk authority.
No product operation, timeout, permission, retry or automatic WSL entry changes.
Unavailable diagnostics preserve the original failure.

Windows regressions passed (13 observation/refusal tests, 12 retention tests).
The actual system PowerShell 5.1 query passed, observing three `wslhost.exe`, two
`wsl.exe`, one `vmmemWSL` and no helper processes, without entering or stopping WSL.
An independently archived Linux checkout passed observation tests0.21s,
retention0.23s, maintained local CI docs19.17s and workflow policy3.64s. The WSL
root-user-session warning was retained without repairs. Product Go code and
workflow definitions are unchanged; the earlier full repository results remain
scoped to their heads. These are diagnostic/component results, not a successful
installed reclamation with the new observations or a fix for the above failures.

The first installed observation at #706 head `38aeae56` failed as recorded above;
its four other required workflows passed. At 6.6 seconds, Windows-visible
`wslhost.exe` and `wsl.exe` counts reached zero. At 12 seconds `wsl.exe` returned
to two, then `wslhost.exe` reached two at 17.6 seconds. This is evidence of new
Windows WSL processes, not proof of which distribution or launcher restarted.
The follow-up classifies parent chains without entering WSL or changing product
behavior. Windows PowerShell 5.1 fixture/query regression and the 14 other tests
passed; the real read-only query classified four WSL processes as PowerShell
descendants. That local observation does not identify the CI restart source.
The archived Linux follow-up passed 14 observation tests (0.50s; the Windows-only
query test was SKIP), 12 retention tests (0.61s), maintained local docs (20.78s)
and workflow policy (1.86s). The same root-user-session warning remained.

At `c3a2cc86`, the first Windows attempt (job104542349177) failed before product
installation: the pinned Microsoft VS Code ZIP response ended prematurely.
Native Windows unit tests passed; installation and later stages were skipped.
One failed-job rerun successfully downloaded/verified that unchanged archive and
passed installation, HTTPS, SSH/editor and Linux reclaim. Public reclaim then
failed as above. At 6.9 seconds all observed WSL launch/host processes disappeared;
at 29.1 seconds two launch processes returned with `other` / `wsl/other` parents.
Thus that run did not identify an SSH, editor, terminal or reclamation parent.
Unknown does not exclude Hacocoon: the initial categories omitted `haco-review.exe`.
The follow-up includes installed notification/client executables and known Windows
WSL host/relay/service categories. It changes no product behavior or timeout.
The extended native PowerShell query passed all 15 Windows regressions (0.52s).
The combined Linux archive passed 14 observation tests (0.21s, one Windows-only
SKIP), local docs (11.48s) and workflow policy (2.16s).

### Current Git binding inventory

The M5 inventory follow-up projects saved single/collection Git bindings through
the existing repository-reference reader. Windows ran 25 tests successfully with
seven Linux-only skips. An archived Linux checkout passed inventory32 (0.36s),
association18 (0.28s), selection7 (0.15s), maintained local docs (13.75s) and
workflow policy (1.70s); the root-user-session warning remained.

Read-only use against the current dedicated WSL observed two repository records
and one binding, with complete reference projection and no unreviewed directory
entries. The preceding inventory had left `bindings` unprojected. The result is
retained privately; source metadata, services and Policy were not changed.
`authority` and binding `state_validated` remain false. This does not prove that
the saved connection is current, that all required data was selected, or that data
has been captured, restored or used for authenticated development.

### SSH and editor entry language

The ordinary SSH/open follow-up uses the shared catalog for Environment selection,
readiness, editor retry and cleanup notices. Common preview-option validation now
reports corrections before Workspace preparation; accepted requests are unchanged.
An isolated 1,669-file Linux copy passed CLI/catalog tests (12.35s), including
actual PTY selection/cancellation and client-owned SSH configuration in English
and Japanese. Maintained local CI `test` passed (80.54s), including all Go tests,
vet, maintenance helpers and notification client tests; local CI `docs` passed
(9.92s). The first Japanese PTY attempt failed because the test binary's common
initializer cleared the language override. The test now explicitly restores its
requested language in the child; both languages then passed. Product selection
or authority was not changed to make that test pass. The existing WSL root-user
session warning remains. These component checks do not prove installed Windows
editor connectivity or person-dependent approval. Those remain separate checks.

### Notification startup versus reclamation

At #707 head c0843ed4, retry job104622907624 passed installation, HTTPS,
SSH/editor and Linux reclamation but failed public compaction (359 opens,
compact_attached, compaction not started, resumed=true, notification stage SKIP).
WSL launch/host counts were zero at 7.0s; at 23.8s two launches returned with
notification/unavailable and wsl/notification/unavailable ancestry. This narrows
one restart source. It does not explain #708 b3cece34/job104626263843, where three
host processes remained after launches reached zero at 6.3s; that run also failed
compact_attached with no compaction and resumed=false. Previous successful
recovery and all preceding failures retain their scopes. #708's quality failure
was two unchecked output errors; both are corrected in the follow-up.

The shared startup reservation follows [ADR 0108](../adr/0108-background-wsl-start-coordination.md).
An isolated 1,672-file Linux copy passed maintained local CI test (73.45s), docs
(9.88s), native review/reclaim test compilation (1.26s/0.89s), and CI-equivalent
Linux changed-code lint 2.13.2 (12.14s, zero issues). Actual Windows passed three
private-peer tests (0.45s) and eight guard/continuation/detached-open tests (1.02s),
including separate-process exclusion and ordinary startup after release. The
first native invocation failed at PowerShell argument parsing before tests ran;
passing an argument array fixed invocation without changing product/test code.
Whole-repository Windows lint could not typecheck the existing Linux-only Incus
syscall.Stat_t test dependency. Its narrowed retry did not run: the local WSL
failed CreateInstance/E_FAIL while C: had zero free bytes. Generated test binaries
and source archive were removed; source and logs remain, with only about 20 MiB
free afterward. No WSL restart, user-data deletion or permission relaxation was
performed during that attempt. Subsequent installed acceptance is recorded below.

### Integration after the responsibility layout change

At #708 head `7e5971b5`, [Windows run 35052021171](https://github.com/SLktEx/Hacocoon/actions/runs/35052021171)
and job104654261663 passed ordinary installation, SSH/editor, retained
Workspace/OCI/snapshot restoration and native notification ownership/refusal.
Public reclamation completed after 254 open attempts and resumed successfully:
allocated bytes fell from 7,730,102,272 to 4,957,667,328 (2,772,434,944 recovered),
with virtual capacity unchanged. This is evidence for that candidate; it does not
erase preceding attached-disk failures or prove every external client closes.

The same head passed Incus, Ubuntu and quality workflows but
[test run 35052021206](https://github.com/SLktEx/Hacocoon/actions/runs/35052021206)
failed its race job104654262485: Japanese forwarding presentation returned while
the advertised listener still accepted a connection. The other product test jobs
passed; the evidence aggregate failed because race failed. Main `bfa19ecb` / #694
already fixes concurrent close completion in the shared stream implementation and
contains deterministic listener/accepted-connection/relay regressions. Integration
reuses this fix rather than weakening the failing assertion or rerunning the old
head until it passes.

After integrating `bfa19ecb`, an isolated copy on the new WSL passed the maintained
local CI `test` and `race` entry points. The focused CLI/forwarding/stream/Base
race checks passed first. Windows cross-builds and actual native review (three
tests) and guard/continuation/detached-open checks (eight tests, plus subtests)
also passed. These native checks did not stop WSL or compact the installed disk.
The shared coordination code now lives under `internal/platform/wsl/coord`.
The unmerged named-builder decision uses ADR 0109 to avoid colliding with main's
ADR 0107; its behavior is unchanged.

The new local WSL initially used installed `bfa19ecb`, Ubuntu 26.04.1 and Incus 7.0.1;
ordinary-user doctor passed all six checks. The user selected only five Hacocoon
development Git trees for evacuation. All archived contents, restored Git objects,
HEADs and working states were verified on Windows. Other application data and
non-Git test copies were explicitly excluded. This is source preservation, not
all-managed-data, guest-idmap or authenticated restored-development acceptance.

At head `d8ec1374`, Incus run 35090329488 passed all three product jobs
(104774859128, 104774859406 and 104774859443). Evidence job104778087319 failed:
artifact10444681175 records `needs_success=true`, no failed product attempts and
no unproven required steps, but the last job's conclusion remained null after
60 seconds, leaving `incus-owned-btrfs` missing. Later terminal job metadata
confirms success. This observation failure remains recorded; no test was rerun.
The bounded metadata observation is now 180 seconds, with missing jobs named in
the failure output. Regression checks cover delayed success, expiry, immediate
terminal failures and API errors; none grants success from `needs` alone.

The same candidate was packaged using pinned GoReleaser 2.17.1 and the normal
Windows packager/installer on the new `Hacocoon` WSL. Client and controller both
report `d8ec1374`; all six doctor checks passed. Ordinary public commands cloned
main into an independent Workspace, wrote a retained marker from a temporary
Env, and reopened it after cleanup. Snapshot, independent restore, deletion of
only the restored Env, then a new temporary Env retained the marker SHA-256,
guest owner `0:0`, Git administration directory and OCI attachment. This is a
small retained-data check, not populated OCI/application, authenticated Git or
huge-repository acceptance. The first marker probe's `git` subcommand failed
because the default Base has no Git executable; its shell's final hash command
returned zero. Later probes use fail-fast execution and do not claim Git execution.

One explicit cold WSL configuration call found no controller socket and did not
apply a change. A normal open management terminal, successful doctor and fresh
configuration inspection preceded the confirmed apply. Three ordinary
`require-approval` rules are limited to `packer-tools` and the exact Ubuntu
HTTP/HashiCorp HTTPS destinations, expiring at 2026-09-16T13:42:55Z; default deny
is retained. The attempt subsequently failed at `dependencies` with apt exit 100:
Ubuntu package downloads could not connect through the ordinary proxy. No human
GUI answer was confirmed. Packer download, HCL execution, publication and reuse
were not reached. Normal CLI inspection confirmed the temporary `packer-tools`
Env absent and no `roadmap-tools` Base published. The three temporary rules were
then removed through revision-bound configuration; default deny and zero rules
match the original settings. No implicit allow or automatic retry was used.

Windows run35090329311/job104774858507 at `d8ec1374` passed installation,
HTTPS, SSH/editor and Linux reclamation, but public reclamation failed with
`compact_attached`, 359 opens, no compaction and successful resume. Notification
acceptance was skipped. Launch/host counts reached zero at 6.7s; at 23.1s two
host processes appeared without a sampled live launcher, then disappeared at
39.4s. The 94.2s launcher ancestry is the worker's bounded resume. No new
notification ancestry was observed; this does not prove that no short-lived
launcher existed between samples. The cause remains unresolved. CI now projects
WSL host ancestry separately with the same bounded fixed categories, without
changing product shutdown, the 90-second detach budget, or refusal conditions.
All 16 Windows observation regressions passed, including actual PowerShell 5.1
projection. The earlier `7e5971b5` success does not resolve this failure.

The local `d8ec1374` public export produced 599,424,512 bytes; the Windows copy
matched SHA-256 `0e3fdd21e6eb257b53123f837a1e32cd60fc62d74d21dc311c5f5d62f922c9da`.
Import through the projected Windows path created independent managed data.
After deleting only that imported Env, a new temporary Env retained the same
marker hash, guest owner, Git directory and OCI attachment. Original source,
snapshot and bundle remain. This is one small same-PC transfer, not cross-machine
or authenticated development acceptance.

The same locally installed candidate subsequently completed public `haco reclaim`
on the new `Hacocoon` WSL, operation `1c487b24-df95-4cc6-85fe-cb0f97d490ae`.
Windows allocation fell from 9,603,907,584 to 7,861,174,272 bytes: 1,742,733,312
bytes recovered, 254 open attempts, virtual capacity unchanged and resume successful.
Normal public status reported both Linux stages and Windows complete; all six
doctor checks passed afterward. A temporary Env using the independent imported
Workspace retained the marker hash, guest owner `0:0`, Git directory and OCI
attachment, then confirmed cleanup. No global WSL shutdown or forced detach was used.
This does not explain or erase the earlier local enrollment and CI attached-disk failures.

An initial post-reclaim probe chose the source Workspace still leased by the
stopped `roadmap-save` Env. Creation correctly refused with `storage has active
sessions`; the CLI hid that reason behind unknown-cleanup guidance. The attempted
Env was absent on inspection. The source Env and its lease were retained, and the
successful probe used the independent imported Workspace instead. The follow-up
shares bounded failure classification with daily commands, adds bilingual `busy`
guidance and preserves JSON, exit/cleanup semantics and backend redaction.
The reason/lease regressions failed before the change and passed afterward in
both languages. Maintained local `test`, `docs` and `workflow-policy` checks plus
focused CLI race tests passed. A development CLI against the installed controller
reproduced the same busy refusal with Japanese reason/lease guidance; the source
Env remained the only retained Env. This is CLI-plus-installed-controller evidence,
not a new installer or release.

At `423fa602`, test35093944048, quality35093943917, Ubuntu35093944270 and
Incus35093944417 passed, including the Incus evidence aggregate. Windows
run35093944130/job104786573377 passed installation, HTTPS, parallel cold SSH,
actual VS Code editing, export/import and post-deletion retained-work recreation.
Its tunnel confirmed native listener ownership and eight 1 MiB exchanges, then
Ctrl+C produced the generic Windows connection failure and exit 1. The terminal
fixture timed out waiting for exit zero. Reclaim and notification steps were
skipped, so no new host-ancestry result exists. Main was not merged or the run retried.

The unchanged installed `d8ec1374` tunnel passed that complete ordinary Windows
terminal journey locally, including native ownership, exchanges, half-close and
Ctrl+C listener cleanup. The first local invocation could not start its terminal
driver because pywinpty was missing; after preparing CI's pinned 3.0.2 in the
test directory, the product path ran without repair or policy changes. The CI
cancellation failure remains unresolved. The companion now records bounded
failure stage/category, cancellation state and elapsed time, preserving nonzero
exit results and the ten-second forced-stop fallback rather than assuming cause.

<a id="incus-key-download"></a>

## Incus signing-key connection failure

At `1054688e`, test35097958384, quality35097958387, Ubuntu35097958415 and
Windows35097958377 passed. Windows job104799915515 confirmed the normal tunnel's
native ownership, eight 1 MiB exchanges, half-close and Ctrl+C cleanup. Public
reclamation completed and resumed, recovering 2,790,260,736 allocated bytes;
the installed notification route step passed. This is not a human approval answer
and does not explain the earlier intermittent tunnel/attached-disk failures.

Incus35097958382 failed: standalone job104800045423 could not connect to
`pkgs.zabbly.com:443` while retrieving the signing key (curl exit 7, 207 ms), before
product tests. Owned-Btrfs104800045764 and Core104800045879 passed; the evidence
aggregate correctly failed. No same-head rerun or main merge was performed.

The shared product/CI installer now retries only the key download for bounded
transient failures, retaining HTTPS and the exact primary-key check. Local real
curl against an isolated TLS server covers 503 recovery, exhaustion, permanent
404 refusal and recovered-but-untrusted key refusal before Host writes. The
first new fixture omitted Content-Length and failed on TLS EOF; after fixing
the fixture's HTTP framing, all 13 helper tests passed in 9.19 s. Corrected-head
packaged/native CI acceptance remains pending; earlier failures remain recorded.

At `a0303de9`, test35104226448, quality35104226544, Ubuntu35104226594 and
Incus35104226446 passed. The new 13 key-helper tests passed in CI (9.01 s).
A separate local real-curl probe observed actual connection refusal (exit 7),
then started the isolated TLS listener and confirmed recovery through the unchanged
helper. Key parsing and Host/package mutations remained command-boundary fixtures.

Windows35104226632/job104821130448 passed installation, HTTPS, SSH/editor,
tunnel Ctrl+C cleanup and Linux reclamation, then failed public reclamation:
`compact_attached`, 357 opens, no compaction attempted, resume succeeded;
notification was skipped. WSL host counts fell to zero at 7.7 s, returned at
29.1 s with `service/windows-service/other` ancestry, and fell to zero at 45.2 s.
No launcher was present in those snapshots; the shared VM remained. The 93.2 s
launch was the worker's recovery. Five-second sampling cannot exclude shorter
launchers or establish the exact distribution. This failure is unresolved.

A bounded read-only Windows process-start subscription now complements snapshots,
sharing parent classification and reporting no raw names, paths, IDs or arguments.
It never changes worker results, retries work or enters WSL. Local PowerShell 5.1
projection tests cover an already-exited child and a reused parent; reader tests
cover limits, malformed/private fields and diagnostic failure preserving the
original product failure. Actual local subscription was denied by Windows access
control, so event-provider acceptance remains pending CI. Another local WSL was
running and was left untouched; no new installed reclaim was attempted. No
same-head rerun or main merge was performed.

<a id="private-windows-launch-descendants"></a>

## Private Windows launch descendants

The preceding #708 head `a67a982acaaacb8f446db338d8b40808e096b467` passed all
five required workflows and was merged as `63bc41d14336b8c50895b4f689b12e2a49583a31`.
Windows run35110190568/job104841576457 recovered 2,805,989,376 allocated bytes,
resumed the same WSL and passed retained-data, native tunnel Ctrl+C and notification
routes. Its actual process-start observer saw no new WSL launch between stop and
recovery. That pass does not explain the earlier failed candidates above.

On that main baseline, a new native component regression reproduced a descendant
writing after failed readiness and `peer.Close` (2.21 s, failure). The private-job
fix is `aec8d4bca6ed9f07b2ad74c76f99ac6895082442`. Native review package tests then
passed (3.86 s), including delayed descendants, an exited wrapper, unrelated-peer
survival, cancellation and launch exclusion. The interactive toast surface test
was SKIP because no interactive notification session was enabled; human clicks
were not exercised. Windows amd64 test compilation and arm64 compilation passed,
as did the shared review/forward Linux packages. A one-off native read-only
`SessionPlan("Hacocoon")` handshake and confirmed close succeeded against the
installed `d8ec1374` distribution (0.06 s). This did not replace installed binaries
or submit an approval. Packaged acceptance of this fix remains pending.

Separate investigation of installed `d8ec1374` passed three ordinary Windows
native-owner / 8x1 MiB / half-close / Ctrl+C tunnel runs. The fourth stopped before
exchange when `Get-NetTCPConnection` observation exceeded its 15-second bound;
that is an unresolved observation failure, not another Ctrl+C failure. A bounded
pipe-only probe through ordinary Host entry passed 25 real WSL-to-Windows Ctrl+C
cancellations. The first probe attempted direct outer-WSL execution and failed
with exec-format error; it was not cancellation evidence. None of these passes
establishes the cause of the older `423fa602` tunnel nonzero exit.

No new local public reclamation ran: another distribution remained running and
was left untouched. The actual local Windows process-start subscription remained
unavailable due to Windows access control. The prior `a0303de9` attached-disk
failure remains unresolved; the new reproduced ownership defect is a concrete
fix, not proof of identity with every intermittent failure. See
[ADR 0110](../adr/0110-private-windows-process-ownership.md).

## M2 local Git diagnosis

At implementation commit `9635e2b0` (the provider inspection code), a read-only
probe in the `Hacocoon` WSL invoked production `InspectGitConnection` against the
existing stopped `roadmap-save` Environment, using its controller identity and
recorded managed Workspace. The owned proxy was recognized; a different expected
broker socket and a different Workspace owner were both refused. No native
resource, connection, repository or Policy was changed. Direct normal-user Incus
inspection was unavailable (permission denied); the provider probe ran as the
Incus administrator. This is not ordinary-user desktop or running-Env repair
acceptance. Human authenticated Git/GUI answers and actual repair remain pending.

The first M2 package run retained `TestOrdinaryLargeGitFetchAndApprovedPush`'s
10-second proposal-wait failure (88-second test) and a five-second PTY bootstrap
wait timeout. Neither establishes its cause or is erased by focused success.
The duplicate `--json` regression found in the same run was corrected; focused
Git doctor, controller roundtrip, provider wiring and offline/stale routing checks
and the shipped-command E2E subsequently passed. The earlier M1 milestone-wrapper
timeout remains separate; the full suite was not repeatedly rerun.

## M1/M2 integration gate

M1 #726 head `5b447f9c` passed four workflows, but Windows run36152506002
(job108128998385) failed public reclamation: Linux stages completed, 359 native
open attempts ended with `compact_attached`, compaction was not attempted and
resume succeeded. Notification acceptance was skipped. Its process observer saw
WSL launches during the wait, including unavailable origins; this does not identify
a proven owning process or justify stopping another distribution. The cause remains
unresolved; earlier successful reclamation does not clear this failure.

M2 #727 head `e5dfeaa8` passed four workflows; Windows run36156927382
(job108143623213) failed the doctor acceptance observer. The SSH fixture creates
an external-path Workspace, for which the new Git check correctly reports
`not_applicable`; the old observer rejected every status other than `ok`. The
observer now accepts that status only for `git_broker`, still requiring zero CLI
exit and refusing failed, skipped or unknown checks. Thirty native PowerShell
decision cases and fixture parsing passed. The broader diagnostic script failed
its existing three-second child-timeout evidence assertion locally; that failure
is retained separately and is not an installed E2E success. Both PRs remain
unmerged pending the required exact-head workflows.

A subsequent native diagnostic-script run outside the filesystem sandbox passed
the child-timeout and nonzero-exit regressions. The earlier failed local assertion
remains recorded; no installed Windows outcome is inferred from this pass.

## Editor fixture descendant cleanup

M1 #726 head `5b447f9c` failed Windows run36152506002/job108128998385
after successful Linux reclamation: 359 native open attempts, `compact_attached`,
compaction not attempted, same-target resume succeeded, notification route SKIP.
Process-start evidence includes unavailable origins and cannot identify a proven
restart owner. It does not authorize stopping another distribution.

Investigation found that `test_vscode_environment.ps1` killed only the exact
portable editor processes, leaving their non-editor descendants outside cleanup.
A native Windows parent/child regression reproduced a surviving child with the
old parent-only kill. The fixture now kills the selected process tree and waits
for its root to exit. The same regression passed, confirmed child exit, retained
an unrelated process and accepted repeated cleanup. Only the exact disposable
editor executable is selected; no installed user editor or WSL is killed.
This is a confirmed fixture defect, not proof that every earlier disk failure had
the same cause. No product disk identity, detach check, timeout or Policy changed.
Installed acceptance of the correction remains pending. The native regression is
part of both the Windows workflow and local release-config checks.

M2 doctor correction `038a44c0` passed the installed doctor checks in Windows
run36161613704/job108159280068, including `git_broker=not_applicable`. That run
then failed public reclamation and skipped native notification acceptance; it did
not yet include the editor cleanup correction. The combined local candidate
`fcb7fadb` passed focused CLI/Git/API/provider tests, bilingual pending-approval
and JSON preservation regressions, and the shipped-command E2E. This does not
replace final-head installed acceptance.

## Independent restart diagnostics

Combined PR #727 head `4459418c` passed four workflows but Windows run36164722897 failed with `compact_attached` after 359 opens, no compaction and unsuccessful resume; notification acceptance was skipped. The saved record lacked the resume error detail. This checkout preserves a separate fixed category and numeric code without changing the first failure. Earlier failures remain unresolved; new installed evidence is pending.

Local focused CLI/status and language regressions passed. Native Windows tests passed for a real child exit `0x8000FFFF`, independent resume categories, serialized preservation of the primary failure, existing operation retention and review. The CI projection suite passed 21 tests. No installed WSL stop/compact cycle was run for this change. Initial verification preparation failed due to script quoting and a missing temporary source directory after WSL restart; corrected preparation and the focused tests then passed.

## Interrupted cache collection cleanup

PR #737 implements creation-only cache enrollment scope and tracked interrupted
collection cleanup. Local WSL tests for resource/state/cache/Incus/CLI passed before
the final retry-guidance wording. The later Japanese assertion rejected the valid
instruction to stop before recollecting; it was narrowed to the obsolete mandatory
recovery warning. The final focused result is recorded below after rerun.

Real Incus 7.0.1 on WSL Hacocoon passed
`TestRealIncusEnvironmentDataPlacementE2E` in 216.16 seconds. The disposable fixture
lost one real operation-wait response, confirmed destination cleanup, preserved
source and Workspace contents, resumed the producer, recollected, reused independent
data with another Base name, and deleted owned test data. No production checks were
disabled. This proves the small ordinary-Env flow, not giant-repository performance.
Missing/expired operation receipts and human GUI acceptance remain separate.

Initial local preparation failed on Windows worktree-path and CRLF differences;
correcting the validation script allowed the tests to run. Broad static analysis
reported 61 issues, including two new Boolean-style suggestions, which were fixed;
unchanged baseline findings are not represented as a passing whole-tree lint.

Final local validation passed: cache CLI (including both languages), resource/state/cache race tests, Workspace/controller API regressions, and changed-line golangci-lint (zero findings, matching PR CI scope). `tools/check_docs.py` and `git diff --check` passed. Maintained `bash tools/ci-local.sh docs` passed, including all 19 checker regressions. No release, giant-repository benchmark or human approval acceptance was claimed.

## Native notification input during cache integration

PR #737 head `90cea4a0` passed quality, test, Ubuntu and Incus CI. Windows run
36260388616 failed only native notification acceptance (job 108455453249), then
its evidence gate. The first owned-history clear stopped at `native_progress=decode`
for 40,031 ms, before WinRT; `ConvertFrom-Json` is the intervening operation.
The precise Windows module-loading delay is not established. Replace that implicit
cmdlet dependency with fixed literal fields decoded through .NET; preserve the
restricted process environment, history clearing and all existing deadlines.
The original CI failure remains evidence, not a successful notification run.

Local Windows acceptance passed native owned-history clear, English/Japanese toast display and removal (34.81 s), plus input decoding without module autoload, malformed-frame rejection, cancellation/reaping and diagnostic redaction. An initial confined test process rejected .NET calls under ConstrainedLanguage; normal native execution passed without changing Windows policy or product environment. The hosted failure remains pending the updated CI run; no human answer was submitted.

<a id="nested-packer"></a>

## Nested Incus Packer replacement

Initial Issue #566 candidate based on main `d2bdbdff`: actual nested Packer acceptance was pending at this point.
The historical ordinary-builder failures and unverified records above remain historical
evidence; the new architecture does not retroactively turn them into passes.

Local validation on 2026-09-27 used Go 1.26.7 and golangci-lint 2.13.2.
Packer, Base build/import/manage, Incus, controller API and CLI tests, including
race checks, passed. Changed-code lint reported zero issues; whole-repository
`go vet ./...`, documentation checks (including 19 regressions), workflow-policy
checks and `git diff --check` passed. The full local test entry point failed at
`TestOrdinaryLargeGitFetchAndApprovedPush` (approval request timeout); the same
failure reproduced on unmodified main `d2bdbdff`. The initial Windows-share run
also timed out in the milestone black-box test; that test passed from the native
WSL validation checkout. The broad race attempt also observed the existing
transport timing failure `TestClientCannotSendInvalidRequests`; relevant candidate
package race checks passed separately. These are not full-suite passes.

The real Incus Packer E2E attempt failed during normal trusted-Host networking:
the existing `haco-host0` belongs to the installed Host in another project, so
ownership validation correctly refused reuse. No Packer stage ran. Exact retained
fixture project/pool: `haco-packer-e2e-b17070ddb4eaf6f0`.
An independent unprivileged test substrate `haco-566-physical` was then created on
its dedicated owned bridge `haco-566-test` (`10.71.201.0/24`). Outbound package
fetching timed out under the existing Docker forwarding policy. Fixture-only
firewall changes required explicit approval and had not yet been applied.

Consequently fresh nested daemon setup, actual Packer/plugin download and execution,
export/stream/import, immutable revision reuse, cleanup, and the added real failure
cases remained **unaccepted** at that stage. The maintained CI now requires
`TestRealIncusPackerBaseBuildE2E` through `ci_required_tests.py`; hosted CI has not run
for this candidate. No SKIP or partial run is counted as a successful Packer build.

Follow-up on 2026-09-27: the user approved two temporary forwarding rules limited
to `haco-566-test` and its established replies. The independent substrate then
installed and started Incus 7.0.1. Initial fresh-Host attempts exposed missing
`nftables` and an over-wide subordinate ID allocation in the extra nested test
substrate. Product Host setup now includes `nftables`; build profiles bound their
ID range to 65,536. The substrate's allocation was confined to its parent's range.

Fixture `haco-packer-e2e-fbe80b3c42466d65` passed normal fresh Host setup, actual
Packer 1.16.0 installation, nested daemon initialization, real Incus plugin 1.0.5
initialization/loading, validation and build instance creation. Build
`228843315a966b96e64b545df11a7078` FAILED during shell provisioning because guest
boot initialized `/tmp` after script upload. This is not a successful build.
Its stopped Host, nested instance and receipt are retained; its NIC was detached
only to release the fixture bridge. Templates now use Packer's documented
`remote_folder = "/root"`. A separate real Incus probe confirmed the default
profile must be removed with the private project, not deleted independently;
the worker and regression fixture were corrected accordingly.

The 64 GiB archive ceiling was replaced with a 1 TiB default and explicit
`--max-image-size` override. Size arithmetic, bounded stream rejection, a 65 GiB
sparse-file CLI check, related race tests, vet, changed-code lint, docs and workflow
checks passed. This does not establish TB-scale image acceptance. Re-running the
full local test entry point still failed the previously reproduced Git approval
timeout (`TestOrdinaryLargeGitFetchAndApprovedPush`, 89.81 s).

The corrected run in `haco-packer-e2e-ff391d7466b03e08` reached real shell script
execution, image publication and native export (561,748,480 bytes, nested image
`19964f9c33ccaa6344e485a2f9e1fe1689f6bb0f1bf2621250d26dc01b3cdf34`). Build
`d057e6789fdd9c195abd8ca75026b4c9` then FAILED at the canonical import Env's root-disk
quota: the unprivileged Physical Host fixture cannot apply the Btrfs quota.
The controller stream and validation ran; no canonical Base revision or normal Env
success is claimed. The transport artifact and receipt remain recovery-required.
No product resource limit was disabled. A separate privileged Physical Host
fixture was prepared; its product `haco-host` and ordinary Environments retain
normal unprivileged settings and trust boundaries.

Final run on 2026-09-27: `TestRealIncusPackerBaseBuildE2E` **PASS**, 2302.13 s,
in exact project/pool `haco-packer-e2e-5405bbdd220ba630`. Incus 7.0.1 ran in the
independent WSL Physical Host fixture `haco-566-physical-root`. Normal fresh Host
setup installed Packer 1.16.0 and initialized nested Incus; the real plugin 1.0.5
was downloaded/loaded, HCL validated, and `setup.sh` executed in a separate instance.
Native image export, bounded controller API stream, canonical archive import and
immutable Base publication all passed. A normal Env returned `hello-from-packer`.
A second build changed the new Env to `hello-from-packer-two`, while the original
Env retained its original revision and output. Both successful builds left no
nested temporary instance/image/project, artifact or context. Test Envs and
published test Bases were then deleted through existing lifecycle APIs.

Real invalid-HCL, plugin initialization and provisioner failures preserved the
current Base pointer and original Env. Their exact receipts were retained:
`7a666170a99a7cdc54b2a17af9437f0c`, `0e5b22a4938be0e81d0aee2c0e1e0b5f`,
`f29e7a76901f05b296049d0d597a941f`. Component regressions separately cover export,
interrupted stream, import/unknown acknowledgement, cleanup and cancellation;
these injected failures are not represented as real-provider failure acceptance.

The fixture's first publication used Incus's default compression. Its second
publication used supported `images.compression_algorithm=none` to reduce slow WSL
compression; production daemon settings were untouched. The Physical Host fixture
was privileged to exercise real Btrfs quota, while product Host/build/ordinary
instances remained unprivileged. The controller endpoint used the standard
transport/API/service with fixture-owned state, not the installed production catalog.
Both approved bridge firewall rules were removed and positively checked absent.
Both Physical Host fixtures were stopped. Failed-attempt resources remain for
diagnosis; no guessed cleanup was performed.

The maintained CI requires the real test and rejects a missing/conditional Packer
step, missing test or SKIP. Its bounded timeout was increased after the measured
38-minute local run. Workflow policy and contract regressions passed. Hosted CI,
arm64, TB-scale performance and installed production-controller acceptance remain
unverified. The previously recorded full-suite Git timeout remains a failure.

Follow-up on 2026-09-28: at the user's request, Base build/import now defaults to
no configured image-size or overall duration cap. Explicit `--max-image-size`
limits remain available. Inspection found the shared transfer still inherited the
Environment 64 GiB wire ceiling and 30-minute client/server deadline; the earlier
1 TiB CLI/sparse-file checks had not proved that full path. Base now selects its
own wire limit/deadline policy while Environment/Workspace retain theirs.
The prior 2302.13-second real Packer PASS remains evidence for that earlier revision,
not a newly executed unlimited-size or unlimited-duration acceptance run.
A real short-lived systemd probe confirmed `RuntimeMaxUSec=infinity` and
`LimitFSIZE=infinity`, then exited inactive. No Packer/HCL ran on the Physical Host.
Large-image acceptance remains unverified; arm64 is outside the requested scope.

Follow-up validation passed: related Go tests (including Incus), Base/Packer/CLI/controller race checks, whole-repository vet, changed-code lint (zero findings), docs consistency and diff whitespace checks. A 2 TiB-plus sparse file exercised metadata-only CLI streaming; real socket tests verified deadline policy, and frame-counter boundary tests crossed the former 64 GiB wire cap without claiming that volume of transferred data. The full Packer E2E was not rerun for this limit-policy change.

### Main integration and first hosted attempt

Candidate `8584817d22025204e91f3c83bb98d86d2d9f822e` integrates the current
Image/Environment creation contract. Its hosted Go 1.26/1.27 tests, race, vet,
lint, documentation and Ubuntu installer jobs passed. The new required Packer
step in [run 36360380538](https://github.com/SLktEx/Hacocoon/actions/runs/36360380538)
failed at Packer build after fresh Host setup, pinned tooling and nested daemon
initialization succeeded. The test failed after 80.32 seconds; this is not image
build/import acceptance. The first Sonar new-code coverage gate also failed
(61.7%). Follow-up regression coverage and fixed-category diagnostics address
these gaps without publishing raw subprocess output or weakening required tests.

### Fresh Ubuntu hosted Packer acceptance

Candidate `dced0b45eb83349cd107d51614891b9ae68f3b83` passed the mandatory real
Packer E2E in **250.81 seconds**, [run 36363265837 / job 108744536151](https://github.com/SLktEx/Hacocoon/actions/runs/36363265837/job/108744536151),
on 2026-09-28. Exact fixture project/pool: `haco-packer-e2e-c9f0f66fdab2131f`.
The initial hosted failure was reproduced with scoped AppArmor namespace denials.
The fresh Host had enabled nesting only after boot. Configuring nesting before
first boot lets Incus initialize its nesting mounts and AppArmor namespaces;
no privileged Host or AppArmor bypass was added. The creation regression requires
that configuration in the initial instance request.

Fresh normal setup, pinned Packer/plugin download and loading, validate/build,
actual shell provisioning, nested image generation/native export, controller
stream/validation/import and immutable publication passed. A normal Env returned
`hello-from-packer`. Rebuilding preserved that Env's revision and output while a
new Env used the new revision and returned `hello-from-packer-two`. Both successful
builds left no nested temporary instance/image/project, transport artifact or
build context. Real invalid HCL, plugin-init and provisioner failures preserved
the current Base and old Env; their diagnostic receipts remained owned.

The same candidate passed hosted Go 1.26/1.27, race, vet, lint/Sonar quality gate,
docs, Ubuntu installer and all Incus jobs, including JSON Base builds, snapshots,
Workspace/OCI transfer and cleanup. Integrated native WSL `ci-local.sh test` and
`ci-local.sh race` passed before the final setup-order fix; focused Host creation,
ownership and diagnostic race tests and changed-code lint passed after it. These
results supersede the earlier full-suite failures only for the integrated code.
Windows jobs were still running at this observation.
The earlier failures above remain historical failures. The E2E uses the standard
controller API/service with isolated fixture state; it is not installed production
controller acceptance. No >64 GiB/TB payload transfer or arm64 acceptance is claimed.
