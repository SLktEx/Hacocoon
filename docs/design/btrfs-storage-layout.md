# Btrfs storage layout

Status: **the supported local storage path is Incus-owned loop-backed Btrfs.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage**, **v0.21 Managed Btrfs Transparent Compression**, and **v0.25 Incus-owned Btrfs Storage Acceptance**.

## Default local layout

The local composition lazily asks Incus to create the default pool without a `source=` override:

```text
Incus pool: haco-local-default
  driver=btrfs
  size=128GiB
  btrfs.mount_options=compress=zstd:3,noatime,nodiscard
        |
        v
/var/lib/incus/disks/haco-local-default.img
  (Incus-owned sparse Linux file)
        |
        v
     loop device
        |
        v
  Btrfs filesystem
        |
        v
/var/lib/incus/storage-pools/haco-local-default
  |- cached Base image volumes
  |- Tooling Base builders
  |- Seed builders / cached Seed image volumes
  |- trusted haco-host rootfs
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

Incus owns creation of the backing image, loop attachment, Btrfs formatting, mount/unmount lifecycle, and supported loop-pool growth. Hacocoon owns only the desired pool identity and policy.

There is no second Host-managed block or mount lifecycle in Hacocoon. Installation and runtime code target this single storage shape directly.

The ownership decision and rejected alternatives are recorded in [ADR 0003](../adr/0003-incus-owned-storage.md). Unsupported older layouts are not migrated or deleted by the installer.

## Sparse file versus WSL sparse VHD

Incus' loop-backed Btrfs pool uses a sparse **Linux file**. The logical 128 GiB pool size is not eagerly allocated in full.

That is separate from WSL's `sparseVhd` / sparse-VHDX mode. Hacocoon does not enable WSL sparse-VHD mode as part of this storage design. Windows-host VHDX space reclamation is not exposed as a Hacocoon CLI command; when needed it remains an explicit Windows/WSL operational concern.

## Why rootfs objects share one pool

Base, Tooling, Seed, trusted-host, and Environment rootfs data share the Hacocoon Btrfs pool so Incus can apply its Btrfs storage-driver behavior across their lifecycle:

- transparent Btrfs compression reduces physical bytes where data is compressible;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- Seed-derived Environments can share unchanged extents where supported;
- storage maintenance stays scoped to Hacocoon rootfs data rather than arbitrary Host data.

Hacocoon does not create a separate Btrfs filesystem or loop image per Environment or Seed merely for isolation. Incus volumes/subvolumes provide logical isolation inside the shared pool.

## Compression and defragmentation policy

The default pool requests `compress=zstd:3,noatime,nodiscard`. `compress-force` is intentionally not desired state; normal Btrfs compression heuristics may leave incompressible data uncompressed.

`noatime` avoids access-time metadata writes and COW churn. Applications that depend on access-time updates need a different future policy. `nodiscard` keeps discard out of normal I/O; batch trim and Windows VHDX reclamation remain explicit maintenance, not automatic installer work.

`autodefrag` is also intentionally disabled by default. Automatic defragmentation can rewrite extents and reduce reflink/COW sharing, which is a poor default for a snapshot/clone-heavy rootfs pool.

Compression mount options mainly affect newly written extents. Hacocoon does not automatically rewrite all existing data merely to recompress it.

## Runtime selection rule

The local composition configures a lazy storage provider. Opening a command that does not need Incus root storage does not create the pool.

Before the first Environment, Tooling Base builder, Seed builder, or trusted host needs root storage, the provider checks for `haco-local-default`. If it does not exist, Hacocoon asks Incus to create the Btrfs loop pool with the desired size and mount options. Subsequent Hacocoon-owned rootfs operations use that pool rather than the Host's unrelated Incus default-profile pool.

The runtime expects the current Incus-owned pool shape and does not carry alternate storage ownership paths. A runtime attachment contains only `incus_pool`; external `driver`/`source` attachments are rejected before Incus access. An unavailable attached pool fails without creating a replacement from a Host path.

For an existing pool, the runtime reads `btrfs.mount_options` through Incus. An exact match causes no write. A stale value is replaced with `incus storage set` and read back; a failed read, write, or mismatched readback fails the rootfs operation. Hacocoon never remounts behind Incus. Configuration readback proves desired configuration, not the live kernel mount: real acceptance separately inspects the live mount and must not use a Host remount to make it pass.

## Acceptance coverage

The `incus-owned-btrfs` job in the existing Incus workflow runs the ordinary-user temporary `hacoq` implementation against real Incus. It checks sparse backing, live zstd/noatime policy without discard, compress-force or autodefrag, stale-policy reconciliation, and existing rootfs/Workspace data retention. This is substrate acceptance, not proof of the new product `haco` journey. Unit tests cover matching reuse, stale updates, and read/write/readback failure.

These checks establish lifecycle and policy behavior on the hosted environment. They do not by themselves establish compression ratio, COW efficiency, Windows-host VHDX compaction effectiveness, or every supported Host configuration.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

Independent COW repo clones with their own `.git` and Workspace volumes that survive Environment deletion/Base changes are **planned**. Seed code still uses this pool; its removal is planned separately after Base dependency separation, as described in [the Seed design](oci-seed-and-cow.md).

## Multiple pools

The default local pool is `haco-local-default`. Future explicitly configured pools may use their own Incus-managed storage identity while preserving the same single-owner lifecycle.
