# ADR 0090: Collect complete generations from stopped ordinary Environments

Status: accepted, implementation candidate. [日本語](0090-stopped-cache-collection.ja.md)

## Decision

A configured cache is an Env-owned disposable area. An ordinary Environment
produces its contents. Trusted Host settings select paths and compatibility before
creation. Host management never executes this data to produce the cache.

Collect under the existing Environment and Workspace locks. Share snapshot's
complete lease/instance verification, require a stopped runtime and resolve the
selected area from the current lease. Atomically reserve the exact child and a
fresh source-only destination before the provider request. Generic resource copy
continues to reject attached sources. The exception requires an active exact parent,
an enrolled child and a generation destination with immutable producer/origin receipts.

The provider verifies one ordinary consumer, creation identity, explicit data
device, stopped state and disabled autostart. No trusted Host pause/resume or guest
helper is involved. Persistent CopySource reservations fence start, client access
and deletion, including after process exit. Positive completion commits the copy;
the shared generation CAS then adopts it. A stale producer cannot replace a newer
source. Ambiguous copies and selection writes keep ownership. Source provenance
survives completion but does not keep the producer or old Workspace lease alive.

Expose settings and collection only through the existing trusted controller
transport. Use configuration revision comparison and private, locked, synced
atomic replacement. New Environments re-read settings; existing placements remain
immutable. Commands show configured names, paths, generation numbers and next steps.

## Rejected alternatives and remaining work

Allowing every attached volume copy would bypass exclusive OCI writers and parent
ownership. Temporarily dropping leases, detaching arbitrary devices, disabling
source guards or giving the guest management authority would weaken the existing
lifecycle. File-by-file merging would make partial or incompatible data current.

Existence alone cannot prove an interrupted copy completed. Self-service recovery,
history/clearing, existing-Env enrollment and added-data snapshot/transfer remain
explicitly incomplete. No old-version compatibility layer is introduced. Native
functional acceptance and large-repository measurements are separate evidence.
