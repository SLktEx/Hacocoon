# ADR 0057: Compose native bundle import through retained-data owners

Status: accepted; Linux public CLI/controller integration is implemented, acceptance pending.

## Decision

Verify the complete versioned bundle in bounded anonymous staging before reserving
or creating destination resources. Existing Workspace import and persistent-resource
creation own native volumes, metadata publication and exact receipts. Canonical Env
creation consumes the rootfs archive through the current Incus sandbox and router.
No import catalog, automatic backup, guessed native path or replay state is added.

A fresh random Workspace name and independently generated owners identify new data.
OCI import records the Workspace association before its native create, so deleting
and recreating an Env does not lose that association. New short member IDs derive
from the recorded member owner when a portable repository name would exceed the
existing ID bound. Repository names are preserved; existing restored-member ID rules
remain readable without catalog migration or rewriting saved data.

GitHub routing metadata is preserved as routing only: the existing broker still
requires a matching trusted Host source and current Env approval/generation. Source
Host file URLs are registered offline, never interpreted as destination Host access.
The original bundle and guest Git data remain unchanged. Version-1 bundles lack
routing descriptors; their component labels become offline repository names.

## Failure and lifetime

Unknown Workspace/OCI creation retains exact existing records and reports public
resource names with cleanup-required. It cannot be treated as confirmed absence.
After data publication, failed Env creation uses the existing Workspace lifecycle
lock and lease guard before cleanup. Delete the exact ready OCI generation first,
then the owned Workspace; failed OCI cleanup keeps its Workspace. A retained Env
lease blocks both. Startup failure preserves the Env and data for retry/inspection.

Saved input and pre-existing data are never replacement targets. Name collisions
fail before new native mutation, with canonical creation providing the final race
check. No automatic backup or rollback of the disposable Env is required.

## Scope

The current managed Workspace collection limit is eight. Larger bundles are rejected
before native mutation. Empty routing is supported; reconnection is separate work.
The byte limit, private staging root and OCI kind belong to trusted composition.
Public CLI/controller upload is connected; SSH handshake and live OCI runtime
acceptance remain separate requirements. Native aggregate tests must distinguish these from internal
bundle-to-running-Env acceptance.

## Management upload boundary

The typed import stream sends bounded byte frames and an explicit byte-count/SHA-256
end record. The existing importer must consume and verify the complete staged bundle
before native mutation. No controller-side input path, owner, OCI kind or configurable
budget is accepted from the client. This reuses the management stream; it adds no
native backend or import catalog. Register it only on the management endpoint.

After upload, disconnect or additional input cancels activation. A terminal response
preserves the importer failure receipt and identifies retained resources by public
names. Success requires the upload digest/count, running destination and terminal EOF;
EOF alone is failure. The client does not retry a failed or disconnected import.
The shipped Linux controller registers this stream only on its management socket.
The Linux CLI accepts one required file and an optional destination name; no input
path is passed to the controller. Native Windows file input remains unsupported.
