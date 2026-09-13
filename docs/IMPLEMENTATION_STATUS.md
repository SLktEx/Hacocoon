# Implementation status

[日本語](IMPLEMENTATION_STATUS.ja.md) | English

The current milestone position is **v0.64**. See [versioning and release status](status/versioning-and-release-status.md) for numbering authority and history.

This page describes current code reality on this development candidate. Start with the [getting started guide](guides/getting-started.md) to use Hacocoon. [Acceptance evidence](status/acceptance-evidence.md) owns commit-bound real-host passes, failures and skips; the [roadmap](status/architecture-and-roadmap.md) owns remaining development direction.

**States:** implemented, partial, planned (not implemented), deferred (postponed). Repository implementation does not imply acceptance on every Host or provider.

| Feature | State | Available scope, limits and remaining work |
|---|---|---|
| [Daily entry / setup diagnostics](reference/daily-workflow.md) | implemented | Bounded setup stages and correlation IDs on stderr, final-frame validation and exclusion through disconnect; noninteractive confirmations do not wait. Dedicated Linux acceptance does not establish Windows default-entry/IDE acceptance. |
| [Workspace path entry / forks](design/workspace-workflow.md) | implemented | Explicit repository preparation, owner-pinned path reopen and stopped independent Git/OCI data forks through canonical lifecycle. Recovery-required copies retain ownership; Windows automatic entry and large-repository performance remain unverified. |
| [TCP/UDP development connections](design/network-connections.md) | implemented | Explicit guest loopback listeners, source-generation-bound Policy/approval, optional rule expiry and active revocation. Existing HTTP/SNI and source guards remain. Dedicated provider acceptance is scoped; outbound Internet/VPN and full Windows UI remain incomplete. |
| [Installation / Host](guides/installation.md) | implemented | Ubuntu 26.04+ / dedicated WSL 2, controller-backed setup and doctor, persistent trusted `haco-host`. Native Ubuntu retains its login shell; no native Windows `haco.exe`. Managed-user preparation tolerates a validated pre-existing non-root access group. Packaged English-Windows entry/interop passes; fresh Japanese-Windows entry remains unverified. |
| [Repository / Workspace](guides/git-workflow.md) | implemented | Clone an existing branch; create independent managed copies and collections. Exclusive leases survive stop. Membership editing and general interrupted-preparation recovery remain incomplete. |
| [Environment lifecycle](guides/data-lifetime.md) | implemented | Managed/external Workspace creation, status/list, stop/start/delete. Rootfs is disposable; Workspace and Store survive deletion. Ownership ambiguity blocks release. `switch-base` is disabled; select another Base through normal recreation. |
| [SSH / editor](design/client-and-interactive-access.md) | implemented | Repeatable key/config setup, `haco open` selection, pinned loopback SSH, default VS Code or `--client ssh`; proxy environment is automatic. Broader IDE/Windows and AHP acceptance remains client-dependent. |
| [Interactive terminal sizing](design/controller-client-transport.md#interactive-terminal-dimensions) | implemented | Host/Env shells carry initial dimensions and bounded, separately negotiated resize controls; Linux uses a private raw PTY. Component/real-PTY tests cover editing, resize, bytes, exit and restoration. Installed Incus/Windows/WSL acceptance remains pending. |
| [Ordinary Git](guides/git-workflow.md) | partial | All-heads fetch/pull (1024 heads, 32 MiB pack) and fixed-content push through controller-owned credentials. Single-ref branch creation and fast-forward updates receive separate exact-ref approvals; competing creation fails closed. Durable push status and read-only exact-ref reconciliation preserve unknown results without replay; large packs, branch deletion, force/multi-ref push, LFS/submodules and general recovery remain unsupported. Native all-heads/new-branch acceptance is pending. |
| [Policy / configuration](reference/configuration.md) | implemented | Revision-bound inspect/edit, exact request approval and saved scopes. Deny precedes require-approval, then allow. Broader provider/desktop acceptance is separate; failed notification delivery never grants permission. |
| [Network / DNS](design/egress-authorization.md) | implemented | Controller-owned Standard proxy, Incus lower-layer direct-egress guard and trusted source-bound DNS. Resolve and connect permissions are separate. A read-only kernel source-guard observer exists; full packaged Windows and spoofed-packet acceptance remain separate. VPN/NRPT, restart combinations and broad supported-Incus acceptance remain incomplete. |
| [Setup recipes / preview](design/project-setup.md) | partial | Host recipes apply once per incarnation with explicit script-only retry and private output/exit receipts; Environment Workspace setup, approved restricted HTTP preview and scoped doctor are implemented. Recreation/cancellation, default-browser and wider application acceptance remain. |
| [Temporary execution](design/temporary-execution.md) | implemented | `haco run` uses generation-bound cleanup and retains selected Workspace/OCI data. Captured output is default; `-i` streams pipes and `-it` uses a real terminal. Incus 7.0.1 pipe/PTY and ordinary Windows ConPTY acceptance passed; see scoped evidence. Failed cleanup retains ownership. |
| [Persistent OCI](design/persistent-oci-store.md) | partial | Automatic per-Workspace Store initialization/reuse, exclusive attach and independent stopped copies; optional `--no-oci`. Host area copy boundary and bounded completed-copy recovery exist. Broader installed runtime/version acceptance and Docker Store compatibility remain. |
| [Base build](design/base-images-and-custom-environments.md) | implemented | Definition-driven build, logical identity/revision inspect and reviewed image cleanup. Base selects initial rootfs; it is provenance, not a retained filesystem dependency of snapshots. |
| [Snapshots / restore / copy](design/environment-snapshots.md) | implemented | Stopped managed Workspace/OCI and independently saved rootfs; restore/copy creates a new Env and fresh authority. External Workspace capture, in-place replacement and arbitrary live application consistency are unsupported. |
| [Retained-object cleanup](guides/data-lifetime.md) | implemented | Reviewed Workspace, built Base, whole Store and source-repository deletion; references/native children protect retained objects. Positive absence is required before releasing ownership. |
| [Individual OCI images](design/oci-image-deletion.md) | partial | Attached, Host and detached nerdctl image list/delete, including reviewed unused candidates. Detached delivery is Linux amd64 only; full installed-controller acceptance and detached Docker remain incomplete. |
| [Storage / reclamation](design/storage-reclamation.md) | implemented | Incus-owned Btrfs pool (`compress=zstd:3`), managed rootfs/data routing and enrolled Windows/WSL reclaim with measured CI recovery. Absent current history yields a read-only no-result response; malformed history still fails. Existing-installation and real interrupted-worker review remain unverified. |
| [Environment export/import](design/environment-transfer.md) | partial | Stopped managed bundle, verified Linux delivery, installed controller and Windows projected-file route; one managed cross-WSL fixture and stopped containerd image/writable-data transfer accepted. Not live migration or a whole-installation backup; imported authenticated Git and broader runtime consistency remain incomplete. |
| [Evacuation / replacement](guides/data-evacuation.md) | partial | Read-only inventory and explicit ordinary-tree archives exist, including isolated failed-snapshot fixtures. Native Incus export/import accepted two split images; unified images/new-Env boot are unverified. Whole-installation classification/capture/restored comparison and final replacement are not complete. |
| [AWS S3](design/aws-operations.md) | partial | Approved bounded listing and verified object download, including source-bound guest requests. Repository and synthetic native tests exist; authenticated real AWS acceptance was skipped. This is not an EC2 Environment provider. |
| [Notifications / client APIs](reference/interaction-events.md) | implemented | `pkg/clientadapter`, minimized interaction events and `haco-notify` browser/native/VS Code adapters. Windows review has scoped acceptance; fresh human toast and Linux activation gaps remain. |
| [Legacy OCI Seed / Docker](reference/cli-migration.md) | partial | Optional `HACO_PLUGIN_OCI=nerdctl` or `docker` integration remains on temporary `hacoq`. Seed build/publish/hardening exists; private-registry/COW/failure breadth remains. It is not the current persistent Store workflow. |
| [Cloud / registry / management UI](status/architecture-and-roadmap.md) | deferred | Concrete cloud Environment provider, mandatory local registry, management UI, simultaneous writable Store sharing and live migration are not current features. Provider seams and explicit future directions remain. |

Canonical lifecycle finalization releases ownership only after complete provider
deletion, including source-guard cleanup. Status reports retained-but-absent or
incomplete runtimes as recovery-required. Independent catalog mutation APIs were
removed; temporary runs share bounded cleanup and marker outcomes. Incus rootfs
import accepts SDK architecture aliases while retaining the two supported CPU
families. See [lifecycle ownership](adr/0002-environment-lifecycle-ownership.md)
and [transfer](design/environment-transfer.md#incus-architecture-names-in-rootfs-archives).

## Verification boundary

Use the [CLI reference](reference/cli.md) for commands/defaults and [configuration reference](reference/configuration.md) for settings. Old root commands and Seed/Docker operations are separated into [CLI migration](reference/cli-migration.md).

CI distinguishes repository tests, real Incus substrate tests and packaged installation acceptance. Missing prerequisites for real AWS, private registries or desktop sessions are skips, not passes. Authority, leases and cleanup failures follow the [failure matrix](reliability/failure-injection-matrix.md) and owning designs.

Old development diaries remain in Git history. Decision-relevant unique evidence and unresolved failures are consolidated in [acceptance evidence](status/acceptance-evidence.md).

## Development candidate integration

Main already includes Workspace entry/forks, TCP/UDP connections and daily setup
diagnostics through #581. This candidate integrates main `74bc2205` with the later
roadmap slices: bilingual CLI, generation-bound interactive runs, Git improvements
and GUI/notification review. Main's human output, Host Git/gh and incarnation-bound
script handling are reused. Distribution and native acceptance of this combined
candidate remain separate; see [integration evidence](status/acceptance-evidence.md#main-sync-candidate).


M1 is **partial**: hierarchical bilingual help with arguments/options/defaults, localized daily guidance, shared
BAT final results and native failure-notification grouping are implemented.
Normalized Windows/WSL-to-Host language selection is implemented and matches
English Windows in packaged acceptance at `0c79f820`. All enabled Incus 7.0.1
Core/Btrfs jobs and packaged Ubuntu/Windows workflows pass, including ordinary
SSH/VS Code, transfer, public reclaim and notification routing. Remaining work
includes remaining result/error translations, the Japanese Windows flow, the original
SSH-failure reproduction and installed long-input/resize verification. Human toast
and new GUI decisions remain unverified. See [exact evidence](status/acceptance-evidence.md).
This is development-branch implementation and acceptance, not main integration or distribution.

VS Code review now completes inside a local GUI using exact common saved scopes and private snapshot-bound sessions. Installed rendering and stale-request refusal passed at `e7ba7987`; fresh human decisions and Windows notification-contained responses remain pending. See the [approval contract](design/pending-approval-review.md) and [acceptance evidence](status/acceptance-evidence.md).

Temporary-run cleanup now requires the exact creation identity through the common
lifecycle API, fences name reuse while a run remains unfinished, and preserves
legacy missing-identity records as recovery work. All maintained native workflows
passed at `9f4cf510`. The later stdin/TTY implementation uses that same lifecycle;
Incus pipe/PTY acceptance passed at `b3169814`; Windows input initially failed, then passed after the driver correction at `9767fd93`. See [temporary execution](design/temporary-execution.md).

Human-facing retained-data reviews, deletion consequences/confirmation/results and snapshot result headings now share the English/Japanese catalog. The five reviewed deletion paths use one confirmation function and refuse failed warning/prompt display. Original errors, JSON, ownership checks and default refusal are preserved; full result translation and Japanese Windows remain partial.

Windows review now uses native notification pages and selection controls with a hidden COM helper, fixed distribution routing and the common private approval session. Component coverage includes native COM and English/Japanese ToastGeneric history; human clicks, visible layout and fresh installed answers remain unverified. See [approval semantics](design/pending-approval-review.md) and [acceptance evidence](status/acceptance-evidence.md).

The development follow-up preserves stale-request refusal across duplicate native
COM launches and adds fixed failure diagnostics. Native Windows COM regressions
fail against the prior implementation and pass after correction. The installed
Windows failure at #611 remains unresolved until its route reruns; component
classification coverage is not proof of that failure's cause.

Combined #616 native acceptance is now partial: packaged Ubuntu and Incus
Core/standalone plus earlier Btrfs journeys passed. Base-build's stale implicit-JSON
fixture failed and is corrected in `codex/base-build-json-fixture`; its native
rerun and later skipped storage checks remain pending. See acceptance evidence.
