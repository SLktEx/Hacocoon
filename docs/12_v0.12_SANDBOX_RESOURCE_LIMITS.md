# v0.12 Sandbox Resource Limits

**Status:** first implementation slice implemented. Real supported-Incus enforcement acceptance remains pending.

v0.12 gives Hacocoon Environments explicit host-selected CPU, memory, PID and root-storage budgets. Resource limits constrain consumption inside an Environment; they are not Capabilities for crossing a trust boundary.

## CLI

```bash
haco create --cpu 4 --memory 8GiB --pids 1024 --root-size 40GiB --workspace . dev
haco run --cpu 2 --memory 4GiB --workspace . -- go test ./...
```

Each dimension accepts a finite value or `unlimited`. Invalid, ambiguous, zero, negative, overflowing, or unsupported finite requests fail before successful Environment creation.

## Provider-neutral model

```text
ResourceBudget
  CPU
  MemoryBytes
  PIDs
  RootBytes
```

The effective creation-time budget is persisted. Omitted dimensions resolve explicitly rather than inheriting an ambiguous provider default.

## Incus enforcement

The current Incus adapter applies finite limits to a stopped Environment, reads them back, verifies the requested values, and only then starts/exposes the Environment. Apply or verification failure follows normal cleanup/recovery semantics and cannot be reported as successful constrained creation.

Provider-native Incus keys remain adapter details rather than Core vocabulary.

## Provider boundary and fail-closed behavior

```text
Workspace
   |
EnvironmentSpec + ResourceBudget
   |
Environment provider
   +-- runtime.incus (current)
   +-- future provider adapters
```

If a selected provider cannot prove that it enforces a requested finite dimension, creation must **fail closed** rather than silently ignore the request.

The v0.7 provider-neutral routing seam remains, but concrete cloud implementation is currently deferred. There is no active EC2/AWS/EBS provider path whose ResourceBudget behavior should be described as current implementation.

## Workspace storage is separate

`--root-size` constrains the Environment root filesystem where the provider can enforce it. It is not a promise to quota an arbitrary Host Workspace mount.

## Agent/Base relationship

Per-agent Environments receive their own ResourceBudget. Coding agents do not receive Hacocoon/Incus authority to raise the ceiling. A custom v0.11 Base controls guest contents but cannot raise or disable host-selected limits.

## Failure and recovery

Invalid input, unsupported dimensions, provider rejection, read-back mismatch, later attach/start failure, persistence failure, and crashes during creation participate in normal failure/recovery handling. When Hacocoon cannot prove that a requested finite budget was applied, it must not report success.

## Non-goals

v0.12 does not implement aggregate cluster scheduling, autoscaling, live mutation, Host Workspace quota management, arbitrary provider config passthrough, or an agent capability to raise its own limits.

> **v0.12 gives each Hacocoon Environment an explicit host-selected resource budget and fails closed when a requested finite ceiling cannot be proven before the sandbox becomes available.**
