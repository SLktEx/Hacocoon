# ADR 0055: Retain offline Workspace data without Git authority

Status: accepted. Extends [ADR 0053](0053-workspace-native-import.md).

## Decision

An empty remote **and** branch means a Workspace has no managed Git route.
Keep its ordinary name, data, ownership and lifecycle. Source repository records
still require a nonempty remote and branch. Partial routing remains invalid.
No new schema, lifecycle state or synthetic repository is introduced.

Native import accepts explicit empty routing, while refusing source Host file
URLs. Snapshot copies with absent routing can restore offline without inventing
a route from guest Git configuration or a recycled source repository name.
Existing nonempty catalog fields and saved archives are not rewritten or discarded.

The Git broker excludes offline members from mixed bindings. An entirely offline
Workspace cannot create a Git endpoint. For every online member, both connection
and request validation require the current Host source's remote and branch to
match the protected Workspace registration. Same-name collisions grant nothing.
Existing Environment/generation, source-owner and approval checks remain.

Offline members do not reserve a coincidentally same-named Host Git source.
Explicit source deletion therefore leaves their independent data untouched.
Workspace native ownership and lease-based deletion protections remain unchanged.

## Scope

This is the internal data/authority boundary needed by public import. Mapping
source file/legacy metadata to explicit offline import, user-facing reconnection
and aggregate import/activation remain follow-up work. Native import never runs
guest Git configuration as trusted Host code; it preserves that configuration as
data. This decision does not authorize network access or transfer credentials.
