# Implementation status

[日本語](IMPLEMENTATION_STATUS.ja.md) | English

The current milestone position is **v0.68**. See [versioning and release status](status/versioning-and-release-status.md) for numbering authority and history.

This page describes current code reality on main. Start with the [getting started guide](guides/getting-started.md) to use Hacocoon. [Acceptance evidence](status/acceptance-evidence.md) owns commit-bound real-host passes, failures and skips; the [roadmap](status/architecture-and-roadmap.md) owns remaining development direction.

**States:** implemented, partial, planned (not implemented), deferred (postponed). Repository implementation does not imply acceptance on every Host or provider.

| Feature | State | Available scope, limits and remaining work |
|---|---|---|
| [Experimental VS Code](reference/experimental-vscode.md) | implemented | Shared YAML subtree editor/file/JSON commands, Env Remote settings and age/pre-release/exact-version extension resolution, including dependencies. Stable desktop, default server paths and Linux x64/arm64 only; live Marketplace/editor and Windows/WSL acceptance pending. |
| [Standard Host tools](design/trusted-host.md#standard-host-tools) | implemented | Normal local setup supplies Git/gh and pinned containerd/nerdctl/BuildKit before user recipes, using managed OCI data and Host-local sockets. Repeat setup preserves data. Released Windows installer, arm64 runtime and custom existing-installation acceptance remain separate. |
| [Daily entry / setup diagnostics](reference/daily-workflow.md) | implemented | Bounded setup stages and correlation IDs on stderr, final-frame validation and exclusion through disconnect; noninteractive confirmations do not wait. Dedicated Linux acceptance does not establish Windows default-entry/IDE acceptance. |
| [Workspace path entry / forks](design/workspace-workflow.md) | implemented | Explicit repository preparation, owner-pinned path reopen and stopped independent Git/OCI data forks through canonical lifecycle. Recovery-required copies retain ownership; Windows automatic entry and large-repository performance remain unverified. |
| [TCP/UDP development connections](design/network-connections.md) | implemented | Explicit guest loopback listeners, source-generation-bound Policy/approval, optional rule expiry and active revocation. Existing HTTP/SNI and source guards remain. Dedicated provider acceptance is scoped; outbound Internet/VPN and full Windows UI remain incomplete. |
| [Installation / Host](guides/installation.md) | implemented | Ubuntu 26.04+ / dedicated WSL 2, controller-backed setup and doctor, persistent trusted `haco-host`. Native Ubuntu retains its login shell; no native Windows `haco.exe`. Managed-user preparation tolerates a validated pre-existing non-root access group. Native interop/ordinary entry have candidate-bound packaged Windows CI evidence; wider installations remain separate. |
| [Repository / Workspace](guides/git-workflow.md) | implemented | Clone an existing branch; create independent managed copies and collections. Exclusive leases survive stop. Selected membership in independent forks and checkout/linked-worktree input are implemented; in-place membership editing is unsupported. General interrupted-preparation recovery remains incomplete. |
| [Environment lifecycle](guides/data-lifetime.md) | implemented | Managed/external Workspace creation, status/list, stop/start/delete. Rootfs is disposable; Workspace and Store survive deletion. Ownership ambiguity blocks release. `switch-base` is disabled/on hold. |
| [SSH / editor](design/client-and-interactive-access.md) | implemented | Repeatable key/config setup, `haco open` selection, pinned portless SSH through ProxyCommand and controller UDS, default VS Code or `--client ssh`; proxy environment is automatic. Broader IDE/Windows and AHP acceptance remains client-dependent. |
| [Interactive terminal sizing](design/controller-client-transport.md#interactive-terminal-dimensions) | implemented | Host/Env shells carry initial dimensions and bounded, separately negotiated resize controls; Linux uses a private raw PTY. Component/real-PTY tests cover editing, resize, bytes, exit and restoration. Installed Incus/Windows/WSL acceptance remains pending. |
| [Ordinary Git](guides/git-workflow.md) | partial | All-head fetch with independent per-ref read checks; one new branch or existing fast-forward push with fixed-content approval. Clone/fetch grants no push authority; main remains reviewable. Exact old/new remote observation reconciles a dispatched push without replay. Fetch reuses verified ancestor history; existing-target push omits the available old history. New-target push reuses one available advertised ancestor through a fresh exact-ref read. Multi-head fetch indexes each bounded pack in sequence, so aggregate batches may exceed 32 MiB. Single full or incremental packs above 32 MiB, LFS/submodules, force/deletion/multi-ref and general recovery remain limited or unsupported. Native authenticated use and large repositories need separate acceptance. |
| [Policy / configuration](reference/configuration.md) | implemented | Revision-bound inspect/edit, exact request approval and saved scopes. Deny precedes require-approval, then allow. Broader provider/desktop acceptance is separate; failed notification delivery never grants permission. |
| [Network / DNS](design/egress-authorization.md) | implemented | Controller-owned Standard proxy, Incus lower-layer direct-egress guard and trusted source-bound DNS. Resolve and connect permissions are separate. Per-Env host/backend/disabled selection survives snapshot/copy/import; client loopback TCP forwarding uses controller streams. A read-only kernel source-guard observer exists; full packaged Windows and spoofed-packet acceptance remain separate. VPN/NRPT, restart combinations and broad supported-Incus acceptance remain incomplete. |
| [Setup recipes / preview](design/project-setup.md) | partial | Host recipes apply once per incarnation with explicit script-only retry and private output/exit receipts; Environment Workspace setup, approved restricted HTTP preview and scoped doctor are implemented. Recreation/cancellation, default-browser and wider application acceptance remain. |
| [Temporary execution](design/temporary-execution.md) | implemented | `haco run` creates an ephemeral Env and always requests cleanup, retaining explicit Workspaces. Captured output by default; `-i/-it` enables bounded stdin/TTY with real Incus acceptance. Windows streamed use remains unverified. Failed cleanup retains ownership. |
| [Persistent OCI](design/persistent-oci-store.md) | partial | Automatic per-Workspace Store initialization/reuse, exclusive attach and independent stopped copies; optional `--no-oci`. Host area copy boundary and bounded completed-copy recovery exist. Broader installed runtime/version acceptance and Docker Store compatibility remain. |
| [Base build](design/base-images-and-custom-environments.md) | implemented | Definition-driven build, logical identity/revision inspect and reviewed image cleanup. Explicit uncompressed container archive import shares publication/cleanup. Base selects initial rootfs; it is provenance, not a retained filesystem dependency of snapshots. |
| [Packer Base build](design/packer-base-builds.md) | partial | Real Packer 1.16.0 evaluates HCL2 and external scripts in an ordinary builder; publication/cleanup are shared. Actual installed build remains blocked at ordinary dependency download permissions. Custom plugins, arm64 and full reuse acceptance remain. |
| [Cache generations](design/cache-generations.md) | partial | Host-configured creation-time enrollment, stopped whole-area collection, independent CoW reuse, history/positive-completion recovery/source clear and reviewed Env-local emptying. Snapshot/copy/transfer preserves uncollected data. Late enrollment and unknown native-copy cancellation remain incomplete; giant performance is deferred. |
| [Snapshots / restore / copy](design/environment-snapshots.md) | implemented | Stopped managed Workspace/OCI, named disposable data and independently saved rootfs; restore/copy creates a new Env and fresh authority. Component inspection reports presence, references and retry guidance when deletion fails. External Workspace capture, in-place replacement and arbitrary live application consistency are unsupported. |
| [Retained-object cleanup](guides/data-lifetime.md) | implemented | Reviewed Workspace, built Base, whole Store and source-repository deletion; references/native children protect retained objects. Positive absence is required before releasing ownership. |
| [Individual OCI images](design/oci-image-deletion.md) | partial | Attached, Host and detached nerdctl image list/delete, including reviewed unused candidates. Detached delivery is Linux amd64 only; full installed-controller acceptance and detached Docker remain incomplete. |
| [Storage / reclamation](design/storage-reclamation.md) | implemented | Incus-owned Btrfs pool (`compress=zstd:3`), managed rootfs/data routing and enrolled Windows/WSL reclaim with measured CI recovery. Absent current history yields a read-only no-result response; malformed history still fails. Bounded bilingual preparation/launch diagnostics are implemented. The dedicated local start failure and real interrupted-worker acceptance remain separate. |
| [Environment export/import](design/environment-transfer.md) | partial | Stopped managed bundle, verified Linux delivery, installed controller and Windows projected-file route; one managed cross-WSL fixture and stopped containerd image/writable-data transfer accepted. Not live migration or a whole-installation backup; imported authenticated Git and broader runtime consistency remain incomplete. |
| [Evacuation / replacement](guides/data-evacuation.md) | partial | Read-only inventory includes current schema16 named data, generation references and pending lifecycle receipts; explicit ordinary-tree archives exist, including isolated failed-snapshot fixtures. Native Incus export/import accepted two split images; unified images/new-Env boot are unverified. Required-current-data classification and restored development remain incomplete. Old-version reconstruction/replacement is excluded from current M0–M5. |
| [AWS S3](design/aws-operations.md) | partial | Approved bounded listing and verified object download, including source-bound guest requests. Repository and synthetic native tests exist; authenticated real AWS acceptance was skipped. This is not an EC2 Environment provider. |
| [Notifications / client APIs](reference/interaction-events.md) | implemented | Minimized events and optional adapters. VS Code GUI and Windows notification pages complete explicit answers through common review/Policy; opening alone never answers. Fresh installed GUI/human answers and Linux activation remain unverified; native/component evidence is scoped separately. |
| [Seed retirement](design/oci-seed-and-cow.md) | implemented | Seed runtime/build/harvest/catalog/sampling/recommendation and its old image deletion/re-enable state are removed. Current Base, managed images and OCI Stores remain; optional Docker integration is independent. Old-version compatibility/migration is out of scope. |
| [Cloud / registry / management UI](status/architecture-and-roadmap.md) | deferred | Concrete cloud Environment provider, mandatory local registry, management UI, simultaneous writable Store sharing and live migration are not current features. Provider seams and explicit future directions remain. |

Canonical lifecycle finalization releases ownership only after complete provider
deletion, including source-guard cleanup. Status reports retained-but-absent or
incomplete runtimes as recovery-required. Independent catalog mutation APIs were
removed; temporary runs share bounded cleanup and marker outcomes. Incus rootfs
import accepts SDK architecture aliases while retaining the two supported CPU
families. See [lifecycle ownership](adr/0002-environment-lifecycle-ownership.md)
and [transfer](design/environment-transfer.md#incus-architecture-names-in-rootfs-archives).

## Verification boundary

Implemented: [Incus 7.0 LTS installation](design/installer.md#incus-package-baseline)
is shared by Ubuntu, Windows/WSL and both native CI setup paths, with patch updates
and actual-server version validation. Doctor reports unsupported servers; 6.0
fallbacks remain best effort. Vendor daemon recognition and anonymous volume
export preserve ownership checks. [Acceptance evidence](status/acceptance-evidence.md#incus-lts)
separates the successful integrated candidate from this main-targeted extraction.

Use the [CLI reference](reference/cli.md) for commands/defaults and [configuration reference](reference/configuration.md) for settings.

CI distinguishes repository tests, real Incus substrate tests and packaged installation acceptance. Missing prerequisites for real AWS, private registries or desktop sessions are skips, not passes. Authority, leases and cleanup failures follow the [failure matrix](reliability/failure-injection-matrix.md) and owning designs.

Old development diaries remain in Git history. Decision-relevant unique evidence and unresolved failures are consolidated in [acceptance evidence](status/acceptance-evidence.md).

## Main integration and development candidate

Main `2f421006` (#687) integrates daily bilingual/vertical help, Host language
handoff, repeated-failure grouping, installer final results, Git/GUI, Workspace
composition/input, interactive run, DNS/forwarding, cache/Packer and saved-data
work. `ee8bf7fb` (#688) adds reclamation failure diagnosis. Each exact PR head passed
all five CI workflows. Detailed [language coverage](reference/cli-language.md)
remains partial; original tool errors and structured values are not translated.

The normal Windows journey has passed installed SSH/editor/forwarding, retained
Workspace/OCI and measured public reclaim. Native notification refusal/subscription
also passed, but fresh human notification/VS Code answers and visible layout remain
unperformed. These results do not erase the earlier unexplained Windows failures.

**Implemented development candidate:** [#690](https://github.com/SLktEx/Hacocoon/pull/690)
includes #689's read-only restored-tree manifests/comparison and Japanese reclamation
results/review. Comparison covers contents, mode, numeric owners, internal hardlinks
and regular-file/directory xattrs. A small retained tree restored across WSLs matched;
whole-current-data and authenticated development are not proven. The same installed
candidate passed Japanese ordinary entry/help/read-only status. See
[comparison](guides/data-evacuation.md#compare-a-restored-tree) and
[language evidence](status/acceptance-evidence.md#reclamation-language).

Candidate, main and released artifacts remain distinct. There is no new release.
Person-dependent login and GUI acceptance will be performed after release and do
not block integration of implemented features into main; unperformed checks remain
unperformed. Known CI failures still require resolution before merging.
Performance/additional strict validation and excluded old-version reconstruction
are separated from required current-data retention in the
[M0–M5 roadmap](status/architecture-and-roadmap.md).

Development follow-up: `snapshot restore --latest <source-env> [new-env]` selects
by persisted capture time and reuses canonical restore. Unknown/tied times need an
explicit ID; failed restoration never selects a different save. Local and native
acceptance remain separately recorded.

Development follow-up: reclamation now closes an attached virtual-disk observation
handle before waiting to reopen the same pinned path. File/parent ownership pins
remain held; installed acceptance is pending. See [the lifetime decision](adr/0103-virtual-disk-observation-lifetime.md).

Lifecycle operations now use mandatory catalog-scoped locks rather than shared temporary names. This fixes the ownership collision observed during installed Env creation; see [ADR 0105](adr/0105-catalog-lifecycle-locks.md). Installed acceptance of this correction is pending.

## Repository layout and retired CLI

The product entry is `cmd/haco`; implementation locations are in the [repository map](../CONTRIBUTING.md#repository-map). `hacoq`, its direct GitHub capability and Docker status/prepare commands are removed. Current Git/OCI and client helpers remain. Native Ubuntu has controller-backed management commands but no product interactive trusted-Host shell command. Windows login entry remains. See [the decision](adr/0106-responsibility-layout-and-cli-retirement.md).
