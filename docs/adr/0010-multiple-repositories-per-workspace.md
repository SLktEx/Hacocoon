# ADR 0010: Lease a repository collection as one Workspace

Status: accepted for implementation  
Date: 2026-09-06

An immutable managed Workspace manifest may contain several repository copies.
Each member owns a separate Incus Btrfs volume and independent `.git`; the
manifest owns their identities before creation starts. Members are not exposed
as separately leaseable Workspaces. One canonical Workspace lease protects the
entire collection, so two Environments cannot acquire overlapping subsets.
Partial preparation retains the manifest and provider ownership for recovery.

The Incus adapter maps members to `/workspace/<repository-id>`. Core still sees
one opaque Workspace identity/path. Existing single-repository Workspaces keep
their `/workspace` layout. Rootfs files outside member mounts are not Workspace
data and do not survive runtime replacement.

The Git endpoint is bound to the Environment, immutable Workspace manifest and
registered repository allowlist. A request can select only a member repository;
registration supplies its remote/ref. Approval remains specific to repository,
remote, ref, operation and old/new OIDs. A helper URL never adds authority.

Rejected alternatives are hot-attaching independently leased Workspaces,
sharing a writable trusted `.git`, and accepting arbitrary guest paths/remotes.
Adding/removing members after preparation and generalized recovery are deferred.
