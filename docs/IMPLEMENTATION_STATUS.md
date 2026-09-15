# Implementation status

[日本語](IMPLEMENTATION_STATUS.ja.md) | English

The current milestone position is **v0.67**. See [versioning and release status](status/versioning-and-release-status.md) for numbering authority and history.

This page describes current code reality on main. Start with the [getting started guide](guides/getting-started.md) to use Hacocoon. [Acceptance evidence](status/acceptance-evidence.md) owns commit-bound real-host passes, failures and skips; the [roadmap](status/architecture-and-roadmap.md) owns remaining development direction.

**States:** implemented, partial, planned (not implemented), deferred (postponed). Repository implementation does not imply acceptance on every Host or provider.

| Feature | State | Available scope, limits and remaining work |
|---|---|---|
| [Experimental VS Code](reference/experimental-vscode.md) | implemented | Shared YAML subtree editor/file/JSON commands, Env Remote settings and age/pre-release/exact-version extension resolution, including dependencies. Stable desktop, default server paths and Linux x64/arm64 only; live Marketplace/editor and Windows/WSL acceptance pending. |
| [Standard Host tools](design/trusted-host.md#standard-host-tools) | implemented | Normal local setup supplies Git/gh and pinned containerd/nerdctl/BuildKit before user recipes, using managed OCI data and Host-local sockets. Repeat setup preserves data. Released Windows installer, arm64 runtime and custom existing-installation acceptance remain separate. |
| [Daily entry / setup diagnostics](reference/daily-workflow.md) | implemented | Bounded setup stages and correlation IDs on stderr, final-frame validation and exclusion through disconnect; noninteractive confirmations do not wait. Dedicated Linux acceptance does not establish Windows default-entry/IDE acceptance. |
| [Workspace path entry / forks](design/workspace-workflow.md) | implemented | Explicit repository preparation, owner-pinned path reopen and stopped independent Git/OCI data forks through canonical lifecycle. Recovery-required copies retain ownership; Windows automatic entry and large-repository performance remain unverified. |
| [TCP/UDP development connections](design/network-connections.md) | implemented | Explicit guest loopback listeners, source-generation-bound Policy/approval, optional rule expiry and active revocation. Existing HTTP/SNI and source guards remain. Dedicated provider acceptance is scoped; outbound Internet/VPN and full Windows UI remain incomplete. |
| [Installation / Host](guides/installation.md) | implemented | Ubuntu 26.04+ / dedicated WSL 2, controller-backed setup and doctor, persistent trusted `haco-host`. Native Ubuntu retains its login shell; no native Windows `haco.exe`. Managed-user preparation tolerates a validated pre-existing non-root access group. Current binfmt P/PF and fresh Japanese-Windows entry still need packaged acceptance. |
| [Repository / Workspace](guides/git-workflow.md) | implemented | Clone an existing branch; create independent managed copies and collections. Exclusive leases survive stop. Selected membership in independent forks is implemented in the candidate; in-place editing is unsupported. General interrupted-preparation recovery remains incomplete. |
| [Environment lifecycle](guides/data-lifetime.md) | implemented | Managed/external Workspace creation, status/list, stop/start/delete. Rootfs is disposable; Workspace and Store survive deletion. Ownership ambiguity blocks release. `switch-base` is disabled/on hold. |
| [SSH / editor](design/client-and-interactive-access.md) | implemented | Repeatable key/config setup, `haco open` selection, pinned portless SSH through ProxyCommand and controller UDS, default VS Code or `--client ssh`; proxy environment is automatic. Broader IDE/Windows and AHP acceptance remains client-dependent. |
| [Interactive terminal sizing](design/controller-client-transport.md#interactive-terminal-dimensions) | implemented | Host/Env shells carry initial dimensions and bounded, separately negotiated resize controls; Linux uses a private raw PTY. Component/real-PTY tests cover editing, resize, bytes, exit and restoration. Installed Incus/Windows/WSL acceptance remains pending. |
| [Ordinary Git](guides/git-workflow.md) | partial | All-head fetch with independent per-ref read checks; one new branch or existing fast-forward push with fixed-content approval. Clone/fetch grants no push authority; main remains reviewable. 32 MiB packs, LFS/submodules, force/deletion/multi-ref and general ambiguous-result recovery remain limited or unsupported. Native authenticated use and large repositories need separate acceptance. |
| [Policy / configuration](reference/configuration.md) | implemented | Revision-bound inspect/edit, exact request approval and saved scopes. Deny precedes require-approval, then allow. Broader provider/desktop acceptance is separate; failed notification delivery never grants permission. |
| [Network / DNS](design/egress-authorization.md) | implemented | Controller-owned Standard proxy, Incus lower-layer direct-egress guard and trusted source-bound DNS. Resolve and connect permissions are separate. A read-only kernel source-guard observer exists; full packaged Windows and spoofed-packet acceptance remain separate. VPN/NRPT, restart combinations and broad supported-Incus acceptance remain incomplete. |
| [Setup recipes / preview](design/project-setup.md) | partial | Host recipes apply once per incarnation with explicit script-only retry and private output/exit receipts; Environment Workspace setup, approved restricted HTTP preview and scoped doctor are implemented. Recreation/cancellation, default-browser and wider application acceptance remain. |
| [Temporary execution](design/temporary-execution.md) | implemented | `haco run` creates an ephemeral Env and always requests cleanup, retaining explicit Workspaces. Captured output by default; `-i/-it` enables bounded stdin/TTY with real Incus acceptance. Windows streamed use remains unverified. Failed cleanup retains ownership. |
| [Persistent OCI](design/persistent-oci-store.md) | partial | Automatic per-Workspace Store initialization/reuse, exclusive attach and independent stopped copies; optional `--no-oci`. Host area copy boundary and bounded completed-copy recovery exist. Broader installed runtime/version acceptance and Docker Store compatibility remain. |
| [Base build](design/base-images-and-custom-environments.md) | implemented | Definition-driven build, logical identity/revision inspect and reviewed image cleanup. Base selects initial rootfs; it is provenance, not a retained filesystem dependency of snapshots. |
| [Snapshots / restore / copy](design/environment-snapshots.md) | implemented | Stopped managed Workspace/OCI, named disposable data and independently saved rootfs; restore/copy creates a new Env and fresh authority. External Workspace capture, in-place replacement and arbitrary live application consistency are unsupported. |
| [Retained-object cleanup](guides/data-lifetime.md) | implemented | Reviewed Workspace, built Base, whole Store and source-repository deletion; references/native children protect retained objects. Positive absence is required before releasing ownership. |
| [Individual OCI images](design/oci-image-deletion.md) | partial | Attached, Host and detached nerdctl image list/delete, including reviewed unused candidates. Detached delivery is Linux amd64 only; full installed-controller acceptance and detached Docker remain incomplete. |
| [Storage / reclamation](design/storage-reclamation.md) | implemented | Incus-owned Btrfs pool (`compress=zstd:3`), managed rootfs/data routing and enrolled Windows/WSL reclaim with measured CI recovery. Absent current history yields a read-only no-result response; malformed history still fails. Existing-installation and real interrupted-worker review remain unverified. |
| [Environment export/import](design/environment-transfer.md) | partial | Stopped managed bundle, verified Linux delivery, installed controller and Windows projected-file route; one managed cross-WSL fixture and stopped containerd image/writable-data transfer accepted. Not live migration or a whole-installation backup; imported authenticated Git and broader runtime consistency remain incomplete. |
| [Evacuation / replacement](guides/data-evacuation.md) | partial | Read-only inventory includes current schema16 named data, generation references and pending lifecycle receipts; explicit ordinary-tree archives exist, including isolated failed-snapshot fixtures. Native Incus export/import accepted two split images; unified images/new-Env boot are unverified. Whole-installation classification/capture/restored comparison and final replacement are not complete. |
| [AWS S3](design/aws-operations.md) | partial | Approved bounded listing and verified object download, including source-bound guest requests. Repository and synthetic native tests exist; authenticated real AWS acceptance was skipped. This is not an EC2 Environment provider. |
| [Notifications / client APIs](reference/interaction-events.md) | implemented | Minimized events and optional adapters. VS Code GUI and Windows notification pages complete explicit answers through common review/Policy; opening alone never answers. Fresh installed GUI/human answers and Linux activation remain unverified; native/component evidence is scoped separately. |
| [Seed retirement](design/oci-seed-and-cow.md) | implemented candidate | Seed runtime/build/harvest/catalog/sampling/recommendation and its old image deletion/re-enable state are removed. Current Base, managed images and OCI Stores remain; optional Docker integration is independent. Old-version compatibility/migration is out of scope. |
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

Use the [CLI reference](reference/cli.md) for commands/defaults and [configuration reference](reference/configuration.md) for settings. Old root commands and Seed/Docker operations are separated into [CLI migration](reference/cli-migration.md).

CI distinguishes repository tests, real Incus substrate tests and packaged installation acceptance. Missing prerequisites for real AWS, private registries or desktop sessions are skips, not passes. Authority, leases and cleanup failures follow the [failure matrix](reliability/failure-injection-matrix.md) and owning designs.

Old development diaries remain in Git history. Decision-relevant unique evidence and unresolved failures are consolidated in [acceptance evidence](status/acceptance-evidence.md).

## Main CLI language integration candidate

**Partial:** daily English/Japanese CLI presentation and shared vertical help are
reused from #580/#583 on current main. Current JSON opt-in, portless SSH, Git
connection and Experimental VS Code behavior remain. Normal Windows entry and
validated per-Host-session language handoff are implemented; the installer
preserves OS locale. Full result translation and fresh installed acceptance
remain incomplete. See
[language scope](reference/cli-language.md) and
[validation](status/acceptance-evidence.md#main-cli-language).

## Notification and installer integration candidate

**Implemented candidate:** reuse #583's one-minute grouping for repeated native
failures and the BAT final result/key wait. Approval/recovery events, audit/cursor
integrity and original exit codes remain. Local notification regressions and
Windows BAT/ConPTY checks pass; fresh packaged Windows/SSH and Explorer acceptance
remain separate. See [interaction events](reference/interaction-events.md#repeated-native-failure-notifications)
and [installer results](design/installer.md#windows-final-result).

## Detailed guidance and canonical setup

**Implemented candidate:** #592/#593's bilingual argument/option explanations and
retained-data results are adapted to current main. Deletion warnings share one
client confirmation helper and preserve controller ownership checks. #659's
obsolete preliminary Host-tool provisioning is removed. Local validation and
fresh installed acceptance remain distinct; old-version compatibility/migration
are outside the requested M0–M5 scope.


## Interactive temporary execution candidate

**Implemented candidate:** `haco run -i/-it` uses bounded process streams with the
canonical run lifecycle. Exact creation identities fence cleanup and same-name
recreation; current split ownership and cleanup-outcome handling remain. Old-version
migration/fallback cleanup is excluded. New local and native acceptance are separate.

Notification setup follow-up: service refresh identifies fixed failing operations and user-path observers stop when Host entry has already failed. Reuses `5a6fb54c` on current main and GUI #664. This improves diagnosis and avoids idle waits; it does not establish that Windows activation or service startup is fixed.



## Cache generation foundation

**Partial:** Host-configured creation-time cache enrollment, stopped whole-area collection and independent generation reuse are available through `haco cache settings/configure/status/collect`. Named history/clear and positive-completion recovery are implemented. Snapshot/copy and portable transfer preserve uncollected data with fresh destination ownership; existing-Env enrollment remains incomplete. Supported-Incus transfer acceptance is pending. Workspace/OCI retention remains separate. See [cache generations](design/cache-generations.md).

## Packer Base build candidate

**Partial:** actual Packer HCL2 and external scripts run inside an ordinary builder Env. The optional adapter shares canonical Base publication and cleanup. Full Packer/download/installed Windows acceptance remains pending; see [Packer builds](design/packer-base-builds.md).

Cache history/clear follow-up: implemented candidate. Named history separates
current selection from retained attempts. Revision-bound clear resets reuse and
uses canonical exact-owner cleanup, retaining existing Env/Workspace/OCI data and
uncertain copies. Orphan-source browsing remains incomplete; named positive-completion recovery and added-data
transfer are implemented. See [cache operations](design/cache-generations.md#inspect-and-clear-collected-data).

Push reconciliation follow-up: implemented candidate reusing42aa706f. Durable dispatch/confirmation records and current-owner exact-ref reads distinguish original failure from current remote state, without replaying writes or restoring approval. Main clone/fetch grants no push authority. Fresh authenticated installed use and larger Git transport remain separate.

Cache completion recovery: implemented candidate for named, positively completed copies and generation selection. Common recovery pins the exact target owner, including OCI callers. Unknown native completion, orphan-source recovery and existing-Env enrollment remain incomplete; new real-host recovery acceptance is separate.

## Client TCP access

**Implemented candidate:** `haco env tunnel --target-port 8080 demo` opens a loopback listener for applications. Native Linux stays local; ordinary WSL/Host entry delegates to the installed Windows client, retaining the exact Env creation and WSL registration. Closing the foreground client closes its listener and connections. Shared parsing, process framing, cancellation and installer placement reuse existing development work. Fresh installed acceptance remains separate; DNS modes and VPN/NRPT acceptance remain incomplete. See [client transport](design/controller-client-transport.md#client-tcp-listeners).

Resolver selection: implemented candidate. Environment creation accepts `--dns host|backend|disabled`, defaults to the Physical Host, and preserves the setting through snapshot/copy/transfer. Disabled mode refuses controller lookups even if guest tooling is restarted. Three-mode installed acceptance is pending; see [name resolution](design/name-resolution.md).

The Incus network dialer identifies the calling Host thread when preparing concurrent SSH/forwarding connections; see [ADR0096](adr/0096-calling-thread-network-identity.md). The Host-namespace guard remains enforced. Native thread regression is separate from installed Windows reconnect acceptance.

Snapshot planning now reuses the create/resume/import placement binding for data inside a repository, preserving its Workspace storage identity. The supported Incus7.0.1 regression covers capture, copy and portable transfer; see the acceptance evidence.

## Workspace membership selection

Implemented candidate: `workspace fork --repo first,third` retains selected saved
Git state and independently adds registered Host repositories through the same
restore/cleanup transition. Source work and OCI remain intact. Independent linked-worktree
input is described below; giant-repository measurement remains deferred. See
[the contract](design/workspace-workflow.md#choose-the-copys-repositories).

## Existing Git working-directory input

Implemented candidate: `workspace import` copies a Linux/WSL checkout or linked
worktree into an independent managed Workspace, retaining dirty files, selected
HEAD/index and objects. Host Git config/hooks and other worktree administration
are excluded. Import shares the existing ownership and upload transitions;
unknown results retain a local reference and recovery receipts. See
[the input contract](design/workspace-input.md). Sparse/partial clones, submodules,
Windows-native input and giant-repository performance remain outside this slice.
