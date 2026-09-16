# ADR 0109: Name build Environments without changing ownership

[日本語](0109-named-build-environments.ja.md) | English

Status: accepted for implementation; installed acceptance remains separate.

## Problem and decision

Random builder names cannot be selected in advance by an administrator's ordinary
Environment-scoped communication rules. Requiring an all-Environment rule for
dependency downloads makes routine Packer builds unnecessarily difficult to scope.

Base definitions accept an optional `builder_name`; the CLI exposes `--builder`.
The common Environment name validator applies at the request boundary. Omission
keeps random names, including archive-import builders. The selected name goes only
through ordinary canonical creation. An existing Environment is never resumed,
adopted, replaced or cleaned up when creation fails.

Every attempt still allocates a fresh temporary Workspace and obtains the normal
creation identity/lease. Execution, stop, publication and cleanup check that exact
ownership. A name is neither an ownership identity nor permission to connect.
Policy settings are not changed by a build. Administrator name rules intentionally
apply to later incarnations of that name; ordinary saved Environment answers stay
bound to one incarnation. HCL and scripts never choose Host authority or credentials.

## Rejected alternatives and checks

Do not install broad allow rules, infer permission from a download URL, run Packer
on the Host, or reuse a surviving builder just because its name matches. A named
builder does not establish a successful Packer download/build or application reuse.

Regressions cover name delivery through CLI/controller and the shared build path,
fresh temporary identities across repeated names, no cleanup after creation failure,
retained publication/cleanup failures, and refusal of invalid names before creation.
Existing lifecycle collision, inverse-operation and recycled-name tests remain
authoritative for provider ownership.
