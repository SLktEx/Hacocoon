# ADR 0111: Compose the default development session in the client

[日本語](0111-default-development-session.ja.md) | English

Status: accepted. Refs #715; depends on branch-independent registration (#709).

## Decision

The ordinary entry is repository registration followed by argument-free `open`.
The client selects registered sources and uses the existing Workspace workflow,
canonical Environment lifecycle and desktop SSH client. Keep a durable, locked
navigation reference before preparation, then pin the returned owners. This
extends [directory entry](0062-workspace-entry-and-data-fork.md) without a second
controller lifecycle/catalog, a provider-specific Core default, or new authority.

Explicit Environment/directory opens and all advanced commands remain. Existing
Environment selection moves to `open --select`. Default Base acquisition and
SSH provisioning use normal provider operations, including Policy/Approval.
The CLI reports high-level progress; it neither broadens permissions nor
automatically deletes/replaces failed state.

## Alternatives and consequences

Selecting an arbitrary existing Environment could open unrelated work. A fixed
name without an owner pin could adopt recycled state. A new orchestration catalog
would duplicate the existing preparation/reference and lifecycle mechanisms.
Manually composing lease and metadata writes would violate
[lifecycle ownership](0002-environment-lifecycle-ownership.md).

Rebuilding a prepared collection when registrations change could discard edits,
rootfs changes or OCI state. Preserve immutable membership and report the need
for an explicit fork until a separate data-preserving extension contract exists.
The client reference is per management user's home; it does not synchronize
default-session selection across different client homes. Native acceptance of
the combined flow is separate from repository E2E evidence.

The [owning design](../design/default-development-session.md) defines behavior,
failure, retry and limits. No advanced functionality is removed.
