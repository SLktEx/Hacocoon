# ADR 0050: Own temporary native rootfs images explicitly

Status: accepted for the internal Linux/WSL export adapter; public G1 integration is planned.

## Context

Incus can publish the independent saved rootfs as a unified image archive.
That image need not survive successful export. Neither the source Base filesystem
nor a second retained Base component is necessary. Incus CLI image export selects
and renames output files; unlike volume export, it cannot use the same single
anonymous output path. A failed publication can still leave a native image.

## Decision

Keep Incus API and storage behavior in its adapter, using the official 6.0.5 client.
Support the configured private local Unix daemon first; reject HTTPS/cluster
configurations explicitly rather than changing daemon selection or adding a
fallback backend. Keep Core contracts independent of SDK types.

Record a random image owner, socket, project and protected source before creating
anything native. Append the returned operation and fingerprint durably. Require
terminal operation success, verify exact native ownership and stream a unified
archive to the existing anonymous read-only output. Use an explicit request
context and selected Unix transport, bounded metadata and archive writes, no
redirects, no response-selected paths and an independent full SHA-256 check.

The caller holds the canonical saved-source reservation through completion.
Before returning success, recheck the saved rootfs and remove only the verified
owned temporary image. Unknown publication or cleanup keeps its private receipt
and reports failure. Cleanup uses a bounded independent context; it does
not replay publication, repair the Environment or delete a saved component.
Receipt deletion requires positive native absence and unchanged file identity.

## Rejected alternatives

Do not retain or rename the image as an extra Base object. Do not use a restore
backup or an automatic full rollback workflow. Do not accept incomplete operation
wait results as success, discard uncertain native ownership, run downloaded
archive contents or export native archives through the bounded diagnostic logger.
SDK 6.0.5 image downloads can try a different local socket and do not inherit the
connection context; use a context-bound request over its selected transport.

## Scope

This is one native component producer, not public Env export/import or disaster
recovery. Public G1 must still handle a stopped Env, all managed Workspace/OCI data,
fresh import identity/current security, start and development access without a
separate user snapshot step. Source and transport metadata never grant destination
authority. Existing saved data and catalog schema are unchanged.

## Aggregate export composition

The stopped-Env exporter reuses canonical capture/read/delete, holding the saved
source reservation through native production and archive closure. Its temporary
capture is an export consistency source, not an import/restore backup. Do not
replace these calls with separate metadata/lease mutations or another transfer
recovery catalog. A bundle remains anonymous until all production, exact capture
cleanup and whole-envelope verification succeed. Failed cleanup keeps its existing
snapshot reservation ID. Public import authority remains future work.

## Public export delivery

Linux management-stream delivery sends no client filesystem path to the controller.
It requires explicit digest/count completion after owned source cleanup, bounded
frames and EOF. The client independently verifies the envelope before publishing
an anonymous inode without replacement. Do not substitute bare EOF success,
client-selected controller paths, named partial-file cleanup or overwrite races.
The output is data, not import authority. Windows publication and public import
remain separate unfinished work; Linux output does not imply desktop delivery.
