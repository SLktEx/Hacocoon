# ADR 0113: One creation service and immutable Workspace binding

Status: accepted

Issue #728 adopts `open --new [IMAGE]` as the daily create/start/open workflow and
retains `create IMAGE` as a stopped creation primitive. A separate temporary
execution CLI and post-creation Workspace rebinding conflict with this model.

Both entries call one source-selection service and the canonical lifecycle.
Snapshot input selects saved data; otherwise explicit Image precedes the configured
default. Preferences are protected catalog references. They never rewrite existing
Environment metadata. Provider creation can finish without booting; first start
performs repeatable guest initialization using its creation identity.

Automatic Workspaces belong to their Environment; explicit Volumes have an
independent lifetime. Both use existing exclusive leases. Neither missing data nor
an editor failure authorizes replacing a completed Environment. Canonical ownership
receipts and fail-closed cleanup remain necessary; a new orchestration progress
state machine, implicit volume swaps and in-place Snapshot rollback are rejected.

Snapshot capture creates an ordinary reusable Image in addition to independently
saved Workspace/OCI data. It preserves source running/stopped state; callers must
quiesce applications themselves when application consistency is required.

See [the owning contract](../design/environment-creation.md).

Existing-Environment execution uses the same lifecycle exclusion as deletion and
requires a running provider observation. It never obtains temporary creation or
cleanup authority. Workspace deletion transfers the exact ownership relation to
a pending cleanup reference atomically with Environment finalization. Losing that
relation on partial failure, or reusing its name before absence, is rejected.
