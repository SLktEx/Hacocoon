# Architecture and roadmap

G1 internal export now composes canonical stopped-Env capture, all protected native
archives and anonymous verified output. Dedicated native aggregate export passed
in 314.12s; Linux public export is partial and public CLI/fixture-controller import is verified; installed/desktop import and Windows-file projection are verified. See [Environment transfer](../design/environment-transfer.md#internal-stopped-environment-export).

> **Architecture baseline · Updated 2026-08-31**
>
> Hacocoon is a **Secure Workspace Runtime**. Use [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md) for current code reality and [`versioning-and-release-status.md`](versioning-and-release-status.md) for authoritative development-checkpoint numbering/history.

This document describes product boundaries and forward direction. It intentionally does **not** duplicate the current checkpoint table or the full implementation-status matrix.

Hacocoon gives developer tools and coding agents broad freedom inside isolated Environments while keeping Host and external authority behind explicit trusted boundaries.

## Current shape

```text
Client / IDE / Agent / Orchestrator
              |
           Workspace
              v
       +-------------+
       |  Hacocoon   |
       | Environment |
       | Policy      |
       | Capability  |
       +------+------+ 
              |
      provider / adapter
              |
          Incus (current)
```

On the supported local path, the Physical Host remains the authority for Incus and privileged platform operations while a persistent trusted logical `haco-host` provides the normal management entry point. Incus owns the local Btrfs backing-image, loop, filesystem, and mount lifecycle. `haco-host` is TCB infrastructure, not an untrusted Environment.

The provider-neutral routing seam remains, but **cloud implementation is currently deferred**. Concrete EC2/AWS/EBS code is intentionally absent from the active tree.

Container tooling is also not Core. Optional OCI plugins may provide nerdctl/Docker behavior; with `HACO_PLUGIN_OCI` unset, Core remains usable without containerd, nerdctl, Docker, or a local Registry.

## Product boundaries

Hacocoon owns Workspace identity and leases, Environment lifecycle, execution, client-access primitives, ResourceBudget, Policy/Approval/Audit, narrow capabilities, interaction contracts, trusted Host management boundaries, and trusted session-to-Environment binding.

Hacocoon does not own IDE/AI chat UX, model routing, task DAGs, Git worktree orchestration, a mandatory OCI runtime, or a mandatory local Registry. Provider, client, and developer-tool specifics stay behind adapters, Standard implementations, or Plugins.

## Core, Standard, Plugin

- **Core** owns stable product semantics and security boundaries.
- **Standard** contains project-maintained, replaceable default implementations used by normal installations, including the current Incus backend and hostname-aware egress enforcement.
- **Plugin** contains optional or specialized integrations, including nerdctl/Docker/OCI tooling.

See [`../design/plugin-architecture.md`](../design/plugin-architecture.md) and [`../DESIGN_PRINCIPLES.md`](../DESIGN_PRINCIPLES.md).

## Roadmap model

Development checkpoints are chronological progress markers, not roadmap phases that must all close before later work starts. The complete checkpoint history and current number live only in [`versioning-and-release-status.md`](versioning-and-release-status.md).

The roadmap is organized by architectural direction instead of copying per-checkpoint implementation status:

- strengthen the trusted Host/controller boundary while keeping untrusted Environments free of Host management authority;
- make the persistent logical `haco-host` the normal local/WSL operating surface without moving raw Incus authority into it;
- keep client integrations reusable and client-neutral, including interaction events, notification delivery, VS Code, browser, and future IDEs;
- preserve provider-neutral Environment/Core contracts while keeping concrete cloud backends deferred until local contracts settle;
- keep OCI/container tooling optional and separate from Core;
- keep local rootfs storage on one Incus-owned Btrfs/COW lifecycle and strengthen real-host acceptance around it;
- continue tightening real-host acceptance, especially Windows/WSL, networking, storage behavior, and client integration.

**Local OCI Registry is not a required roadmap gate.** It remains deferred optional infrastructure and may be reconsidered only if measured bandwidth, rate-limit, restricted-network, or centralized-policy needs justify it.

## Current Stage B scope

- B1: fresh Windows installation, native WSL direct `.exe` execution, Windows
  PATH, and projection of actual Windows drives including non-C drives into
  trusted `haco-host`; restart and setup rerun are acceptance requirements.
- B2: preserve multiple independent repository Workspaces and approved Git push.
- Former B3: `switch-base` is currently disabled, not a Stage B requirement,
  and on hold with no reintroduction planned by this roadmap. Historical evidence remains. It does
  not block Stage A-C completion.
- B4: [Persistent OCI Store](../design/persistent-oci-store.md), with explicit
  create/attach/reuse/delete lifecycle and independent offline COW copies. Trusted
  Host image acquisition/publication and full runtime acceptance remain partial.
  Ordinary Environment creation must perform the image COW copy automatically;
  only disabling it is an optional user step. See the owning Store contract.
- B5: real Windows native OpenSSH access through Windows/WSL loopback and Incus
  proxy to Environment sshd, with Windows-owned private keys and strict pinning.
- B6: retain readable Environment location/state and next-action guidance.

## Incus-first lifetime and recovery scope

Incus+Btrfs is the current foundation. Use native instance, volume, snapshot/copy,
device and network operations, adding only data associations and security guards.
Disposable Env recreation must preserve Workspace/Git, retained OCI and saved
snapshots. Snapshot rootfs is independently copied; Base is provenance only and
restore creates no automatic backup. See [snapshot contract](../design/environment-snapshots.md).

Cleanup requires exact ownership and positive absence, not complete runtime
recovery. Doctor should report Incus/storage/network state, data accessibility and
whether a new Env can be created. Backup/WSL migration should move required
persistent data and settings and recreate execution environments. Use existing
Incus tools where suitable; do not build a generic recovery/storage platform.
DB volumes and a management UI are deferred and outside this refactor.

## User-facing development order

The revised user roadmap prioritizes ordinary development with a small `haco`
surface. Preserve the completed A workflow and commit-bound B evidence; do not
reinterpret older acceptance as proof of new requirements. B4 still needs the
full trusted Host preparation -> independent COW Store -> Environment image-use
path. Windows standard SSH is accepted on the recorded B candidate; manual
VS Code Remote - SSH editing/build/test is not yet separately accepted.

After those B gaps, proceed in this order:

- C: repeatable `haco ssh setup`, optional VS Code environment/workspace selection,
  Host customization, Windows DNS for both Host and Environment, project setup,
  restricted preview and concise diagnostics. `haco open` stays editor-neutral.
- After VS Code connection/edit/build/test is usable: expose a short temporary
  Environment execution flow, analogous to `docker run --rm`, with automatic
  runtime cleanup through the existing canonical ephemeral-run service. Retained
  Workspaces and persistent data must not be silently deleted.
- D: human-editable Git/network/AWS approval policy, exact target and scope,
  OS/optional VS Code decisions, and optional AWS operations. Domain resolution
  does not itself grant a network connection.
- E: start/stop and Workspace reuse, snapshots/restore, copy, Base building and
  explicit cleanup of retained Workspaces/images.
- F: storage reclamation across Btrfs/Incus loop/WSL disk, optional management UI,
  diagnostics, reinstall and upgrade.
- G: export/import required Workspace/OCI/configuration, then create new Environments
  in the new WSL. Exact old runtime or WSL reproduction is not required.

Agent orchestration stays outside Core. `switch-base` remains disabled and on
hold; this roadmap does not schedule its return. Native Windows haco.exe, optional
registry/broker infrastructure, concurrent Store sharing and live migration do
not block the simpler workflows. Prefer extending an existing operation with
optional configuration over introducing new required commands/arguments.
Runtime acceptance belongs in [implementation status](../IMPLEMENTATION_STATUS.md).

## Trusted Host direction

On the local Incus/WSL path, Hacocoon distinguishes the **Physical Host** from the persistent trusted logical **`haco-host`**. The Physical Host retains Incus and platform authority; Incus itself owns the Btrfs pool backing, loop, filesystem, and mount lifecycle. `haco-host` is trusted infrastructure inside the TCB, not an untrusted Environment.

The implemented lifecycle/default-entry slice does not expose the raw Incus control socket. Follow-up work should move ordinary Hacocoon operations toward the logical Host through narrow controller/client contracts while preserving explicit Physical-Host recovery and bootstrap paths.

See [`../design/trusted-host.md`](../design/trusted-host.md) and [`../WINDOWS_WSL_BOOTSTRAP.md`](../WINDOWS_WSL_BOOTSTRAP.md).

## Base and OCI separation

```text
haco base list
haco base inspect <base>

haco plugin oci store create dev
haco env create --workspace managed:work --resource oci:dev example
```

`haco base` describes Environment starting identity. OCI/container lifecycle is an optional Plugin responsibility. Current persistent Stores use containerd/nerdctl and BuildKit data; Docker Store compatibility is deferred. Runtime tooling is optional, and its process/socket stays within each Environment.

## OCI storage direction

Current B4 uses a [Persistent OCI Store](../design/persistent-oci-store.md):
Environment-local containerd image/layer/snapshot metadata and BuildKit cache
live on an independently managed Incus Btrfs volume. The controller reserves it
exclusively with the Workspace. Environment deletion releases the attachment
without deleting the Store; explicit Store deletion verifies ownership and
absence. `/run`, processes, sockets and Host authority are not persistent data.
No Seed, image delivery service, registry or credential broker is required.
Earlier Seed/storage work is historical implementation material and is not the
current Stage B direction. Never share one writable runtime data root between
active Environments.

Local rootfs storage routes Hacocoon-owned Base, trusted-host, Environment rootfs and persistent data paths through `haco-local-default`, an Incus-owned loop-backed Btrfs pool, rather than inheriting an unrelated Host default pool. Pool creation requests `compress=zstd:3`; `compress-force` and `autodefrag` are intentionally not desired defaults, and Hacocoon does not automatically rewrite old extents because doing so could reduce reflink/COW sharing.

The ordinary CLI remains non-root. Hacocoon asks Incus to provide the storage pool through the normal runtime boundary and does not implement a separate block-device or mount lifecycle.

See [`../design/oci-seed-and-cow.md`](../design/oci-seed-and-cow.md), [`../design/btrfs-storage-layout.md`](../design/btrfs-storage-layout.md), [`../design/docker-compatibility-plugin.md`](../design/docker-compatibility-plugin.md), and [`../OPTIONAL_LOCAL_OCI_REGISTRY.md`](../OPTIONAL_LOCAL_OCI_REGISTRY.md).

## Egress direction

Hostname-aware egress keeps the authorization contract in Core and concrete default HTTP/HTTPS proxy enforcement in Standard. The Host resolves and pins public destinations only after authorization; the Incus path remains proxy-only at the lower transport layer. Real supported-Incus bridge/nftables/dnsmasq behavior remains an acceptance concern rather than a reason to duplicate status here.

See [`../EGRESS_AUTHORIZATION.md`](../EGRESS_AUTHORIZATION.md).

## Client direction

Clients use generic Hacocoon contracts rather than becoming Core dependencies. `pkg/clientadapter` provides Environment/access operations and composes `pkg/interaction` for client-neutral event observation. Browser/native notifications and the optional VS Code notification extension consume the same minimized event boundary; observation/delivery never becomes an authorization path.

VS Code is the first convenience client. code-server, JetBrains, browser UIs, and future clients should reuse the same boundaries rather than introducing client-specific authority into Core.

See [`../CLIENT_ADAPTER_CONTRACT.md`](../CLIENT_ADAPTER_CONTRACT.md) and [`../INTERACTION_EVENTS.md`](../INTERACTION_EVENTS.md).

## Operational confidence direction

Real-Incus CI acceptance proves the substrate independently before Core lifecycle checks, making substrate failures distinguishable from Hacocoon regressions. Incus-owned storage acceptance exercises the ordinary-user CLI against real Incus and verifies backing-image, loop, Btrfs mount, compression policy, pool reuse, and guarded cleanup; trusted-host acceptance verifies lifecycle/ownership/control-socket isolation.

Structured logging uses `log/slog`, stable operation context, sanitized Host-command diagnostics, and defense-in-depth secret redaction across maintained executables. See [`../reference/logging.md`](../reference/logging.md).

These operational checkpoints improve support confidence and diagnosability without claiming universal Host support. Exact current acceptance remains in [`../IMPLEMENTATION_STATUS.md`](../IMPLEMENTATION_STATUS.md).

## Numbering rule

Minor versions are pragmatic pre-1.0 progress checkpoints. Meaningful product, implementation, operator-experience, observability, or acceptance slices may take the next minor even when follow-up work or real-host acceptance remains. Small fixes and maintenance do not automatically consume another version, but substantial support/operability checkpoints may. During pre-1.0 development, visible progression is preferred over conserving minor numbers.

Published tags/releases and acceptance/support evidence are separate concepts. See [`versioning-and-release-status.md`](versioning-and-release-status.md).

## Historical note

Old commits, branches, PRs, and document versions may use superseded checkpoint assignments or describe removed cloud implementations. Git history is the archive for those states; it does not override the current status/version authority.

Stopped-source Environment copy is implemented through existing Incus COW and canonical creation; see [its contract](../design/environment-copy.md). The representative E4 definition-driven Base build/create/SSH workflow is implemented with acceptance tracked in the [Base contract](../design/base-images-and-custom-environments.md). Broader retained-data cleanup, reclamation and export/migration remain separate roadmap work.

E5 implements reviewed deletion of retained Workspaces, built Base revisions,
whole OCI Stores and unused source repositories. Their existing references,
exact ownership and native saved children remain protected. Individual images in
attached Stores are partial through the current runtime-backed OCI plugin; see
[image operations](../design/oci-image-deletion.md). Host-source image operations extend this same partial checkpoint through the existing
Host-copy/ownership boundary. Detached-Store image routing is partial; automatic compatible tooling is implemented on Linux amd64; native delivery and bare controller/CLI acceptance passed at bd1c9a5. Full installed-controller acceptance remains pending. Candidate-selected GC, F
reclamation/operability and G export/migration remain planned. These remaining stages preserve data and permissions while using
Incus capabilities; they do not require full disposable-Env reconstruction.

G1 remains partial overall. Linux export/import and the installed controller path,
source Env deletion, fresh pinned Windows SSH, continued work and retained-data
recreation passed. Windows-file delivery through the existing drive projection
also passed at c4449e1; native Windows CLI/direct DrvFS publication and migration
to another WSL are not implied. Live OCI runtime consistency and actual Git
reconnection remain incomplete. G2–G4 still require whole-installation inventory,
readable-data evacuation independent of snapshot creation/deletion, restoration
into a new WSL/pool, data comparison and explicit replacement after acceptance.
Base filesystem retention and automatic backup remain absent. See
[Environment transfer](../design/environment-transfer.md).
