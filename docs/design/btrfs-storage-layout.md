# Hacocoon-managed Btrfs storage layout

Status: **repository implementation present; GitHub-hosted normal-user helper and real CLI acceptance are automated; physical compression/COW/compaction acceptance remains host-dependent.**

Milestones: **v0.20 Managed Btrfs Rootfs Storage** and **v0.21 Managed Btrfs Transparent Compression**.

Hacocoon local storage uses one sparse raw backing image per configured storage pool and formats that image as Btrfs. Incus uses the mounted Btrfs filesystem as the source for the corresponding Hacocoon-managed storage pool.

```text
HACO_ROOT/images/<storage-id>.raw   (sparse raw)
        |
        v
     loop device
        |
        v
  Btrfs filesystem
        |
        v
Incus pool: haco-<storage-id>
  |- cached Base image volumes
  |- Tooling Base builders
  |- Seed builders / cached Seed image volumes
  |- Environment rootfs volumes
  `- Incus snapshots / clones
```

The default local storage ID is `local-default`, so the default Incus pool is `haco-local-default`.

The storage ID and managed `.raw` backing path are the durable identity. A concrete `/dev/loopN` name is only an ephemeral runtime attachment: it may change after detach/reattach or Host reboot and may later be reused for an unrelated backing file. Hacocoon must therefore rediscover the current loop from the managed backing image before destructive loop operations rather than treating a cached device number as authoritative.

## Why these objects share one filesystem

The storage boundary is deliberate. Base, Tooling, Seed, and Environment rootfs data should stay on the same Hacocoon-managed Btrfs filesystem so storage-level optimizations apply across the complete lifecycle:

- transparent Btrfs compression can reduce physical bytes for all managed rootfs data;
- Incus Btrfs snapshots and clones can preserve copy-on-write sharing;
- Seed-derived Environments only need new extents for changed data where the storage driver can share unchanged blocks;
- filesystem-level maintenance and optional out-of-band deduplication can target only the Hacocoon filesystem rather than unrelated Host data;
- compaction can return unused extents to the sparse raw backing file.

Hacocoon must not create a separate Btrfs filesystem or sparse image per Environment or Seed merely for isolation. Incus volumes/subvolumes provide the logical isolation inside a shared storage pool.

## Compression policy

Managed Btrfs filesystems use `compress=zstd:3` by default. Hacocoon deliberately does **not** use `compress-force`; normal Btrfs compression heuristics may leave incompressible data uncompressed rather than repeatedly spending CPU on forced attempts.

When Hacocoon finds an already-mounted managed filesystem without the expected compression option, it remounts that managed filesystem with `compress=zstd:3`. A `compress-force` mount is not considered compliant with the managed desired state.

The compression mount option affects newly written extents. Hacocoon does not automatically defragment or recompress existing data because rewriting extents can reduce existing reflink/COW sharing. Physical compression ratio, CPU cost, and supported-host behavior remain acceptance concerns rather than repository-only claims.

## Runtime selection rule

The local composition configures a lazy Hacocoon Btrfs storage provider. Merely opening the local application for a command that does not need an Incus rootfs must not attach a loop image, mount Btrfs, or create an Incus storage pool.

Before the first Environment, Tooling Base builder, or Seed builder needs root storage, the Incus runtime resolves the configured provider, ensures the sparse-raw Btrfs storage and its `haco-<storage-id>` Incus pool, records that pool, and reuses it for subsequent Hacocoon-owned rootfs operations. These paths therefore do not inherit the Host's Incus default profile pool.

A low-level Incus runtime used directly without the Hacocoon local composition retains its legacy default-profile behavior for compatibility. That compatibility path is not the local Hacocoon storage architecture.

## Host privilege boundary

The ordinary `haco` process remains unprivileged. Sparse backing-file creation, sizing, state/lock files, and other operations that do not require Host privilege stay in that ordinary-user process. Only the fixed storage operations that require Host authority are delegated to the dedicated `haco-storage-helper` executable.

The release installer places the helper outside the normal command path at `/usr/local/libexec/hacocoon/haco-storage-helper`, owned by root and not writable by group/other. Before delegation, Hacocoon requires the helper itself to be a root-owned executable regular non-symlink beneath root-owned non-writable parent directories. Fixed OS-provided tools such as `/usr/bin/sudo` may be distribution symlinks; Hacocoon resolves them to a canonical target and validates that target and its parent chain as root-owned and non-writable. Hacocoon does **not** install a passwordless sudo rule. Whether sudo prompts, uses an existing credential cache, or is denied remains Host/operator policy; the CLI itself is not elevated wholesale.

The helper exposes typed operations rather than an executable/argv forwarding API. Its authority is restricted to Hacocoon-managed storage objects and fixed command shapes for loop discovery/attach/detach/rescan, filesystem-type probing, Btrfs format, mount/remount/unmount, usage/resize/minimum-size/balance, and trim. It does not provide arbitrary shell execution, arbitrary mount options, arbitrary block-device formatting, arbitrary loop devices, arbitrary Host paths, or arbitrary Btrfs subcommands.

Every privileged request is revalidated inside the root helper rather than trusting caller-side validation. In particular:

- `HACO_ROOT`, `images`, and `mounts` paths must be canonical real directories owned by the invoking UID and not group/other writable where ordinary-user ownership is expected;
- backing images must be exact `<HACO_ROOT>/images/<storage-id>.raw` regular files owned by the invoking UID, not symlinks, not group/other writable, and have exactly one hard link;
- loop devices must match `/dev/loopN` and report both the expected managed `BACK-FILE` and the same `BACK-INO` as the current managed raw file;
- a newly attached loop is immediately revalidated and detached if its path/inode identity does not match;
- `mkfs.btrfs` is permitted only after the helper itself sees the explicit `blkid` no-signature state, followed by another loop identity check immediately before formatting;
- mountpoints are restricted to exact `<HACO_ROOT>/mounts/<storage-id>` paths, the loop and mountpoint storage identities must match, and a new mount is revalidated after `mount`; an unexpected postcondition is unmounted immediately;
- only `compress=zstd:3`, the fixed targeted balance filters, and validated positive resize targets or `max` are accepted.

Storage lifecycle serialization still occurs in the ordinary storage layer through per-storage leases. Helper validation is independent defense in depth for direct invocation, stale state, partial failure, and confused-deputy cases. Cleanup remains fail-closed: a backing image is not deleted when loop detach fails, and mount/loop identity mismatches are rejected instead of guessing a destructive target.

`HACO_STORAGE_PRIVILEGE_MODE=direct` exists only for fake/test/development environments where the caller already has whatever authority commands need. It never grants privilege and is not a supported shortcut for normal managed Btrfs operation.

Repository CI uses two ordered disposable GitHub-hosted Ubuntu 26.04 acceptance stages. First, the Go test process runs as the ordinary runner user, delegates through the installed root-owned helper, and proves real sparse-image creation, loop attachment, Btrfs format, `compress=zstd:3` mount, inspection, idempotent reuse, unmount, loop detach, and backing-image deletion. Second, a fresh runner combines the same helper boundary with real Incus and executes the actual ordinary-user `haco` binary, proving lazy `haco-local-default` pool creation, `haco create`/`exec`/`delete`, managed-pool reuse through `haco run`, ephemeral cleanup, and exact pool/mount/loop cleanup. These acceptance stages prove that normal local CLI composition works on that hosted environment; they do not by themselves establish physical compression ratio, COW efficiency, compaction effectiveness, or every supported Host configuration.

## Workspace boundary

Host Workspaces remain bind-mounted into Environments and are not required to live inside the Hacocoon Btrfs pool. This storage layout applies to Hacocoon-owned Incus rootfs/image-volume data, not arbitrary user source trees.

## Multiple pools

The rule is one shared Btrfs filesystem **per configured Hacocoon storage pool**, not one hard global filesystem for every possible Hacocoon deployment. Runtime preparation or a configured storage provider selects the `incus_pool` from the storage attachment, so another configured storage ID can map to another `haco-<storage-id>` pool without falling back to the Host default pool.
