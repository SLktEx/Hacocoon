# ADR 0051: Assign fresh volume ownership before native import

> Implementation/acceptance statements below describe the stage when this decision was recorded. See the [current contract and scope](../design/environment-transfer.md) for subsequent implementation and remaining limits. The decision and rejected alternatives are retained.

Status: accepted for the initial Linux Incus/Btrfs volume import adapter.

## Context

Incus custom-volume import restores config from `backup/index.yaml`. Importing
first and relabeling afterward would temporarily adopt source ownership and leave
an ambiguous native resource if relabeling failed. The source's idmap is different:
it describes the numeric IDs already stored in the archive, not destination authority.

## Decision

Use the existing persistent-resource creation path: generate a new owner, plan the
native name, persist `creating`, call the adapter, verify the owned volume, then
commit `ready`. The import method substitutes native archive creation for empty
volume creation; it does not add a catalog, state machine or automatic replay.
Failed creation/verification retains the existing exact identity for explicit deletion.

Before native creation, rewrite the bounded native index in an anonymous archive.
Set the planned destination name, pool, project and current Hacocoon owner/resource
config. Discard old user/config authority, attachment metadata and snapshot/source
identities. Preserve only structurally validated `volatile.idmap.last/next` as data
encoding. Incus performs actual extraction, volume creation and later idmap handling.
Never extract an archive into an arbitrary Host directory or mount source devices.

The initial adapter accepts uncompressed, non-optimized Btrfs filesystem volume
archives without native child snapshots. It rejects ambiguous index fields,
traversal, repeated paths, descendants of symlinks, external/forward hardlinks,
unsupported header types, incomplete closing blocks and nonzero trailing content.
Archive size, index size, path length and entry count are bounded. Normal file
bytes, links, modes, numeric IDs and supported extended metadata are retained.
Existing destination volumes are refused; imported metadata cannot select a target.

## Scope and rejected alternatives

Do not import old ownership and then repair it. Do not create an extra Base object,
pre-import backup, generic fallback extractor or crash-replay framework. A failed
import is not a ready Store merely because bytes exist. Cleanup uses the existing
owner/attachment/saved-object checks and requires positive native absence.

Public aggregate import still needs rootfs and Workspace registration, whole-bundle
composition, fresh Environment identity and current security/connection setup.
An adapter round trip is not boot, SSH or live Docker/containerd acceptance.
Existing catalog schema and saved archives are unchanged.
