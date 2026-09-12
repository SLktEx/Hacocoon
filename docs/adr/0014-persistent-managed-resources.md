# ADR 0014: Attach persistent managed resources through Environment lifecycle

Status: accepted  
Date: 2026-09-07

## Decision

An Environment owns its disposable runtime and reserves its persistent data.
Workspace and an optional additional persistent resource are reserved in the
same canonical Environment creation transition. The initial additional resource
is an OCI Store. Core records provider-neutral resource identity and ownership;
the optional OCI plugin defines its contents and the Incus adapter owns volumes.
This PoC accepts one additional exclusive read/write resource per Environment.

The persistent resource catalog shares the controller's Environment state
transaction and lock. Creation durably records the planned provider identity and
ownership before the provider call. Only a verified volume becomes ready.
Attach reserves a ready exact identity; deleting a reserved resource fails closed.
Environment stop retains reservations. Environment delete releases reservations
only after confirmed runtime absence and never deletes the persistent volume.
An explicit resource delete first excludes attachment, verifies exact ownership,
deletes the volume, confirms absence, then forgets the catalog record. Ambiguous
creation/deletion leaves the record reserved for explicit retry/recovery.

The OCI Store retains containerd's persistent root and BuildKit's reusable cache
on an Incus-owned Btrfs custom volume. Runtime processes, sockets and `/run` are
guest-local and disposable. Stored container metadata does not restore running
tasks. Host registry credentials, Host runtime stores and management sockets
are never mounted. Initial runtime support is containerd/nerdctl; Docker Store
compatibility and concurrent sharing are deferred.

## Superseded decisions

ADR 0012's image delivery implementation is historical and replaced by this
persistent resource model. Its observed acceptance remains historical evidence.
Delivery-only CLI/API/backend code is removed rather than kept as current B4.

ADR 0011's switch-base command is disabled at the user-facing boundary and is
not a Stage B requirement. Internal lifecycle composition/tests remain reusable
historical assets. Stage D or later must reconsider need, semantics and UX;
this does not block Stage A–C. No replacement convenience command is introduced.

## Rejected alternatives

- Binding Store lifetime to Environment deletion loses reusable data.
- Plugin-local leases separate from Environment transactions allow partial
  ownership publication and unsafe cleanup.
- Sharing a Host daemon/socket/store grants Host authority to untrusted guests.
- Saving/loading images alone is delivery and does not model persistent storage.
- Keeping `/run` or automatically resuming tasks conflates data with execution.
