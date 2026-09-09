# Environment transfer

Status: **planned** for the public export/import flow. The native custom-volume
acceptance test is an internal prerequisite, not a usable Hacocoon importer.

## Incus foundation

Use Incus export/import for independent archives of rootfs and custom volumes.
An instance archive does not contain attached Workspace or OCI custom-volume
contents. Hacocoon must bind those archives to their intended roles and check that
all required components were saved before reporting a complete export. Base names
and fingerprints are provenance only; no extra Base filesystem component is needed.
See [instance backup scope](https://linuxcontainers.org/incus/docs/main/howto/instances_backup/)
and [custom-volume backups](https://linuxcontainers.org/incus/docs/main/howto/storage_backup_volume/).

Ordinary file archives can move between pools. Incus optimized archives are tied
to a compatible storage driver; do not treat this difference as merely speed.
Neither successful optimized export nor a new snapshot is a prerequisite for the
later damaged-storage evacuation workflow. That workflow remains separate.

## Data and authority

A future public import creates new managed resources through canonical lifecycle
ownership, preserving Workspace Git state and retained OCI data. It must not
replace an existing Environment or restore old approval, connection or management
authority. Host credentials and control sockets are outside Environment export.
An archive checksum detects changed bytes; it is not authorization to apply its
configuration. Imported owner labels are source metadata, not a newly issued lease.

The dedicated Incus 6.0.5 CLI has no instance-import config/device override flags,
although the current upstream documentation describes them. Do not implement
import by assuming newer flags or by starting an instance before replacing its
old configuration. Rootfs archive validation, current security reconstruction,
file-path handling and aggregate publication still need implementation and tests.

## Native custom-volume acceptance

`TestRealIncusVolumeTransferE2E` is opt-in with
`HACO_E2E_INCUS_VOLUME_TRANSFER=1` on a dedicated root Linux/WSL Incus+Btrfs host.
The runner must see the Incus storage mounts; for a daemon with a private mount
namespace, enter its freshly observed namespace within the same invocation.
It uses two new random pools and synthetic `work`/`oci` volumes. It writes an exact
ownership plan under `/var/lib` before creation and retains the plan and archives
outside both pools. Failure retains resources for explicit inspection. Cleanup
checks the test ownership marker and positive pool emptiness before pool deletion.
It never selects an existing Workspace, OCI Store, snapshot, shared image or pool.

The test uses non-optimized `--volume-only --compression=none` export and import
into the second pool. It checks unpushed Git commits, uncommitted and untracked
files, hardlinks, symlinks, file mode, independent writes, archive immutability and
source-deletion independence. It also checks that native import carries the old
user-config marker, making the need for fresh Hacocoon ownership explicit.

Rootfs import, UID/GID and extended-attribute coverage, real Docker/containerd
contents, public commands, Windows artifact delivery and cross-host acceptance
remain unverified. A passing volume test does not prove the complete G1 flow.

The first dedicated run failed before export because the fixture omitted the
`default_` volume path prefix and ran outside the daemon mount namespace. Its
exact owned resources were retained, then deleted after matching owner markers
and empty inventories. The corrected run inside the current daemon namespace
passed in 11.24s on Incus 6.0.5/Btrfs. Both test pools were cleaned; the exact plan
and two archives remain at `/var/lib/haco-volume-transfer-2481101147`. The original
failed-run plan remains at `/var/lib/haco-volume-transfer-3035986437`.
