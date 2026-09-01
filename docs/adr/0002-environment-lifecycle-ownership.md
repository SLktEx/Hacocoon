# ADR 0002: Make Environment lifecycle and resource ownership explicit

Status: accepted  
Date: 2026-09-02

## Context

Hacocoon Environment operations span several independently fallible resources:

- authoritative Environment metadata;
- Workspace leases/locks;
- provider runtime/instance state;
- managed storage;
- Environment networking and security enforcement;
- connection/runtime resources.

Historically, locally reasonable code could mutate these resources independently. Several failures then appeared in different forms:

- an in-flight Workspace lease was once indistinguishable from a stale lease when Environment metadata had not yet been persisted;
- runtime cleanup failure could leave a runtime alive after the Workspace lease had been released;
- cleanup timeout/recovery policy was fixed on one lifecycle path and later reappeared on another;
- real-user E2E exposed partial startup/retry states that component-oriented paths did not model as one lifecycle.

The problem is not that callers were insufficiently careful. The mutation surface allowed callers to construct ambiguous durable state.

## Decision

Environment lifecycle is a resource aggregate with one canonical mutation path.

The Core/Workspace service must not independently compose low-level Environment metadata and Workspace-lease mutations. It uses lifecycle transitions instead:

```text
BeginEnvironmentCreate
  -> provider creation
  -> RecordEnvironmentRuntime
  -> validation/materialization
  -> CommitEnvironmentCreate

failure after provider ownership is known
  -> provider cleanup
  -> FinalizeEnvironmentDelete
     or
  -> MarkEnvironmentRecoveryRequired

Delete
  -> prove provider runtime absent
  -> FinalizeEnvironmentDelete
```

For the JSON state store these transitions are atomic with respect to the Environment state file.

### Runtime ownership must be durable before later fallible work

Immediately after provider creation succeeds, Hacocoon must durably record the provider runtime reference while the Workspace lease is still `acquiring`.

Only after all required validation has succeeded may the store atomically publish:

```text
ready Environment metadata
+
active Workspace lease
```

An active lease without corresponding ready Environment metadata is not a normal intermediate state.

### Cleanup is fail-closed

The Workspace reservation may be released only after the provider runtime is proven absent.

If cleanup fails or resource ownership is uncertain:

- retain the reservation;
- preserve any known runtime reference;
- transition to `cleanup-required`;
- return `ErrRecoveryRequired` with actionable context.

Absence of Environment metadata alone is never proof that a provider resource is gone.

### Delete finalization is one durable transition

After the provider runtime is absent, Environment metadata and its Workspace lease are forgotten together. A persistence failure leaves the previous durable state retryable instead of making only one half disappear.

## Consequences

### Positive

- callers cannot accidentally publish an active Workspace lease separately from the ready Environment;
- provider ownership survives failures between runtime creation and ready-state publication;
- delete cannot free the Workspace while retaining stale Environment metadata, or remove metadata while retaining an apparently unrelated lease;
- failure-injection tests can target a small semantic lifecycle surface rather than many implementation-specific calls;
- AI/code changes have one canonical lifecycle implementation to copy.

### Costs

- state backends used by the Workspace service must implement the lifecycle transition contract;
- provider APIs must return a durable runtime identity after successful creation;
- a provider that cannot establish exact ownership must fail closed rather than return an anonymous successful runtime.

## Rejected alternatives

### Keep independent `PutEnvironment` / `PutWorkspaceLease` calls and rely on ordering

Rejected because ordering conventions are easy to duplicate incorrectly and have already produced partial-state bugs. Review/checklists are weaker than a restricted API.

### Treat missing metadata as proof of stale provider state

Rejected because metadata can legitimately lag a provider transition or be lost during a partial failure. Destructive recovery requires positive ownership/absence proof.

### Release the Workspace lease when cleanup is attempted

Rejected because attempted cleanup is not proof that the runtime stopped using the Workspace.

### Cover the behavior only with user-journey E2E

Rejected. E2E remains the product guarantee, but deterministic lifecycle invariants must also fail at state/service test layers.

## Related decisions and work

- #20 / #21: Workspace lease lifecycle and cleanup safety failures
- #63: cleanup timeout class reappearing on another path
- #370: authoritative real-user/release acceptance
- #380: shift E2E-discovered regressions to the lowest faithful layer
- #381: deterministic failure injection and recovery
- #382: cross-cutting security invariants
- #383: Environment-owned network/security lifecycle
- #386: mistake-proofing umbrella

## Reconsider when

Revisit this ADR only if Hacocoon replaces the JSON state backend or introduces a distributed control plane where atomic mutation of Environment metadata and Workspace reservation cannot share one local transaction boundary.

Even then, preserve the semantic invariants: explicit lifecycle state, durable ownership before later fallible work, fail-closed ambiguity, and one canonical transition API.
