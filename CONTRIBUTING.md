# Contributing

Hacocoon optimizes for replacement, deletion, clear responsibility boundaries, and operational simplicity.

Before changing code, read `CODEX_START_HERE.md`, `docs/README.md`, and the current release specification.

## Design checks

A change should answer these questions:

1. Is this a stable Hacocoon concept, or a concrete provider/tool detail?
2. Does it belong to the **current release gate**, or is it future work that should remain deferred?
3. Can the implementation be deleted/replaced without rewriting Core?
4. Does the change preserve the trust boundary between the untrusted Environment workload and host/external-service authority?
5. Does any long-lived parent credential, host HOME, Incus control socket, or Hacocoon control state enter the Environment? If yes, redesign it.
6. Is a conditional encoding a real product rule, or should an adapter/strategy own the behavior?
7. Can failure and cleanup be retried or diagnosed without guessing external state?

## Architecture rule

Prefer the current vocabulary: Workspace, Environment, Execution, and—when their release arrives—WorkspaceLease, CapabilityRequest, PolicyDecision, and ApprovalRequest.

Historical `Session`, Runtime/Storage-centric, and plugin-heavy code may remain while the rebaseline is implemented, but new public APIs and architecture should not deepen those old couplings.

## Definition of done

As applicable, add positive tests, negative/security tests, cleanup/retry tests, no-secret logging checks, and regression tests for retained behavior.

For documentation changes, run:

```bash
python tools/check_docs.py
```

Do not expand a release merely to preserve historical implementation surface.
