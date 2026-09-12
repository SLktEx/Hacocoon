# Data evacuation and migration review

[日本語](data-evacuation.ja.md) | English

Status: **partial maintenance workflow**. Whole-installation capture, reconstruction,
comparison and replacement are not complete. This guide is for the administrator
of the Physical Host; normal development uses [Environment export/import](../design/environment-transfer.md#commands).

## Decide what must survive

Inventory managed Workspaces (including all collection members and Git metadata),
OCI Stores, saved rootfs/snapshots, controller associations and Policy/settings.
Separately review trusted Host files/credentials, manual or unregistered trees,
external volumes/pools/VHDs and Windows references. Base image references are not
saved Base filesystem components.

A successful snapshot or export is not a prerequisite for evacuating readable data.
If snapshot create/delete fails, preserve its ownership receipts and original data.
A remaining snapshot can protect extents and prevent reclamation without making
readable files unavailable. Do not delete records, repair permissions, or create
additional snapshots merely to force the migration path.

## Read-only inventory

Run from a checkout on the Physical Host with its existing Incus administration
access. Protect the output because paths and ownership metadata can be sensitive.
Use the actual configured controller root when it differs:

```bash
umask 077
python3 tools/evacuation_inventory.py --catalog /var/lib/hacocoon/state/environments.json --repositories /var/lib/hacocoon/state/repositories > inventory.json
```

Omit unavailable catalog options; their absence remains a review gap. The helper
queries projects, pools, images, instances, custom volumes and snapshots, retaining
successful observations alongside failed queries. Native collection is bounded to
256 queries/five minutes between queries, with a 30-second per-query deadline.
Exit 1 / `native_queries_complete: false` means incomplete observation.

Only selected references and owner markers are projected, never arbitrary config or
credentials. URI-shaped sources are withheld; file references are not followed.
Images include full fingerprints, types, aliases and their native source project;
shared project views are not separate ownership. Catalog schemas 10–13 are read
without migration; schema 9 is unsupported. `state_validated` remains false.
Unprojected pending restore/copy/run records remain explicit counts requiring review.

Association comparison is bounded to 4096 rows. It distinguishes observed/missing/
mismatched markers, unsupported routes, ambiguous views and incomplete queries.
Reverse native review can report no supported reference; that is **not an orphan
or deletion list**. Generation, ownership, saved children and unknown references
still require investigation. `authority` stays false, `review_required` true and
`backup_complete` false, including on exit zero.

For manual files, add `--files /absolute/reviewed-root`. The Linux walk records bounded
metadata, never regular-file contents, ACL/xattr values or symlink targets.
It pins directories without following symlink components and checks mount identities.
Mount boundaries, symlinks and special files are deferred. Read errors, observed
changes and the 50,000-entry / 64-level / 60-second limits retain partial results.
An all-file listing is neither content capture nor permission to delete the old WSL.

## Capture a reviewed tree

Stop all writers first. Select a new empty mode-0700 destination owned by the caller,
outside the source and on storage that will survive replacement. Run from the
repository on Linux:

```bash
umask 077
mkdir -m 700 /absolute/private-capture
python3 tools/evacuation_capture.py /absolute/reviewed-source /absolute/private-capture --quiesced
```

Replace both absolute paths. The helper uses GNU tar directly and produces
`data.tar`, an intent, `data.tar.sha256` and a completion receipt. No encryption,
key generation, Incus snapshot or catalog mutation is required. Copy the archive
and receipts to the chosen retention location, then run there:

```bash
sha256sum --check --status data.tar.sha256
```

Completion requires tar success, synced output, unchanged observed source metadata
and pinned directory/output identities. Symlink path components, incomplete enumeration,
separate mounts and special files are refused; file symlinks are archived without
following targets. Numeric filesystem IDs, modes, links, ACLs, xattrs and sparse
metadata are preserved, **without converting Incus idmaps**.

`--byte-limit` and `--seconds` default to 64 GiB and 900 seconds. Failure retains partial
output/intent and writes no completion receipt. Existing files are never overwritten
or automatically removed; retry with a new destination. Only this call's child is terminated.

`--quiesced` is your confirmation, not writer detection or an atomic snapshot.
A checksum detects copy damage, not substitution of both archive and checksum.
Keep receipts private. Old optional encrypted captures still need their original
keys; newer archives or synthetic keys cannot recover missing old identities.
See [the retained failed-key evidence](../status/acceptance-evidence.md#transfer).

## Restore, compare, then decide

Restore ordinary trees into a new private destination using the appropriate native
tool; do not extract untrusted archives over live controller state or activate old
management configuration. Reconstruct current ownership, settings and permissions
deliberately. For managed bundles use the public import command and fresh connections.

Compare required contents, types, modes, guest-visible UID/GID, Git history/dirty
files and application data. Check hardlinks, symlinks, ACLs/xattrs and external
mounts where needed. Raw Host UID/GID equality can be wrong across Incus idmaps.
Exercise the new environment after restart and same-name recreation where required.

One managed cross-WSL fixture and isolated failed-snapshot/readable-rootfs fixtures
passed, with [explicit limits and failed attempts](../status/acceptance-evidence.md#transfer).
Whole-installation review and final G4 replacement remain unfinished. Keep the old
WSL and independently retained data until all required comparisons are complete;
inventory or a single successful import cannot authorize its deletion.

## Retain ordinary Incus images

Status: **partial G2/G3**, using native Incus commands. Retain an image when it is needed for future creation; an independent snapshot rootfs does not require its original Base image. This procedure does not add a snapshot component, Hacocoon catalog object or daily command.

On the source Physical Host, use the reviewed full fingerprint and its actual image namespace from the inventory. Choose a new private directory; stop if any command fails:

```bash
umask 077
mkdir -m 700 /absolute/new-image-export
incus image export FULL_FINGERPRINT /absolute/new-image-export/image --project SOURCE_PROJECT
ls -l /absolute/new-image-export
```

Keep every output part. In the tested split-image case, the prefix produces `image` (metadata) and `image.root` (rootfs); a unified image can instead produce `image.tar`. Do not infer missing output from the prefix alone or export again over existing files. Compute SHA-256 for the actual parts, copy them to a new retention directory outside the source WSL, and compare their checksums there before import.

On the destination Physical Host, record a new project name and unique ownership description before creating it. Enable its own image namespace explicitly; a shared namespace would not isolate this check:

```bash
incus project create RESTORE_PROJECT --description UNIQUE_RESTORE_DESCRIPTION -c features.images=true
incus project list --format=json
incus image import /absolute/retained/image /absolute/retained/image.root --project RESTORE_PROJECT
incus image list --project RESTORE_PROJECT --format=json
```

Verify the recorded project description and `features.images` before import. For a unified archive, supply only its actual archive path to `incus image import`. Require the imported full fingerprint and image type to match the source, then recheck the retained files. Record failures and exact resources already created; do not guess cleanup targets or replace an existing project. Keep the source and retained archives. Importing an image does not register a Hacocoon Base, restore aliases, adopt old authority, create an Env or prove it boots.

See [scoped acceptance and limits](../status/acceptance-evidence.md#transfer).
