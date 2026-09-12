# Secure Workspace Runtime

Status: **implemented foundation**. Current feature availability and acceptance
limits belong to [implementation status](../IMPLEMENTATION_STATUS.md).

The runtime connects user-selected Workspace data to an isolated Environment,
executes commands with explicit results and deletes disposable execution resources
while retaining independently owned data.

```text
Workspace → canonical Environment lifecycle → provider runtime
                                            → Execution / client connection
                                            → verified cleanup
```

The current local provider uses Incus system containers. A selected external
Workspace refers to the controller's filesystem, not automatically to the client's
current directory. Managed Workspaces provide independent data copies.
[Getting started](../guides/getting-started.md) owns the current product workflow;
[CLI migration](../reference/cli-migration.md) separates the original commands.

Core defines stable data/lifecycle/authority contracts; the Incus adapter owns native
resources. Prefer existing Incus operations and add only missing development,
data-lifetime and security behavior. Do not constrain Incus for hypothetical backends
or make optional Git/OCI/client implementations mandatory in Core.

Only selected development data is exposed. Host HOME, reusable credentials,
protected Hacocoon state and Incus management authority remain outside the Env.
Guest root is not Host root. A writable Workspace may be modified or deleted by its
workload; isolation is not backup or protection from authorized writes.

Creation, execution and cleanup must preserve argument boundaries, exit status,
bounded untrusted output and exact ownership. Record native creation before further
fallible setup. Unknown cleanup retains leases and recovery evidence.
[ADR 0002](../adr/0002-environment-lifecycle-ownership.md) defines the canonical sequence.

The original first gate proved external-directory mount, read/write, execution,
interactive access and cleanup before later integrations. Its obsolete implementation
checklist and CLI targets remain in Git history; they do not override current
lifecycle, Policy or client contracts. Real provider tests must establish native
isolation/data behavior separately from unit tests or mocked adapters.
