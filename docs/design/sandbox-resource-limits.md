# Sandbox resource limits

[日本語](sandbox-resource-limits.ja.md) | English

Status: **implemented provider/legacy slice**; broad real-Incus enforcement acceptance
remains incomplete. Product `haco env create/run` does not expose these budget flags.
Use [CLI migration](../reference/cli-migration.md) for the temporary legacy boundary.

ResourceBudget has CPU, MemoryBytes, PIDs and RootBytes. It limits consumption inside
an Env; it is not a Capability to cross the Host boundary. Each dimension accepts
a finite positive value or `unlimited`; omitted dimensions resolve to unlimited.
Invalid, zero, negative, overflowing, ambiguous or unsupported values fail closed.

Legacy Physical Host examples (not the installed product workflow):

```bash
hacoq create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace /absolute/work dev
hacoq run --cpu 2 --memory 4GiB --workspace /absolute/work -- go test ./...
```

CPU and PID values are positive integers. Storage/memory sizes use the parser's
binary units; prefer explicit `MiB`/`GiB` rather than ambiguous human shorthand.
The effective creation budget is persisted. It cannot be raised by selecting a Base
or by an untrusted agent.

The Incus adapter applies finite limits while stopped, reads them back, then starts
the Env only after verification. Canonical creation records the native ownership
receipt **before** later fallible device/resource configuration; cleanup retains
leases until positive absence. Apply/readback/start/persistence failure cannot
be reported as successful constrained creation.

Provider-native keys stay in the adapter. A provider that cannot enforce a requested
finite dimension refuses creation; there is no silent ignore or weaker platform
fallback. The provider routing seam remains, but concrete cloud implementation is
deferred; there is no active EC2/EBS enforcement contract.

RootBytes limits rootfs where enforceable, not an arbitrary attached Workspace.
Cluster scheduling, autoscaling, live budget mutation, Host Workspace quotas and
arbitrary provider-setting passthrough are outside this slice.
See [lifecycle ownership](../adr/0002-environment-lifecycle-ownership.md).
