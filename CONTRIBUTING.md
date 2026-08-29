# Contributing

Hacocoon optimizes for replacement, deletion, clear responsibility boundaries, operational simplicity, and explicit security boundaries.

Before changing code, read `CODEX_START_HERE.md`, `docs/README.md`, `docs/IMPLEMENTATION_STATUS.md`, and the versioned specification/design document relevant to the subsystem you are changing.

Hacocoon is still **pre-1.0**. Breaking changes are allowed when they make the system smaller, safer, or more coherent. Existing behavior on `main` is not automatically a permanent compatibility contract.

## Design checks

A change should answer these questions:

1. Is this a stable Hacocoon concept, or a concrete provider/tool detail?
2. Does it fit an existing v0.1-v0.7 architecture boundary, or is it genuinely new post-v0.7 product scope that needs a design decision first?
3. Can the implementation be deleted/replaced without rewriting Core?
4. Does the change preserve the trust boundary between the untrusted Environment workload and host/external-service authority?
5. Does any long-lived parent credential, host HOME, Incus control socket, or Hacocoon control state enter the Environment? If yes, redesign it.
6. Is a conditional encoding a real product rule, or should an adapter/strategy own the behavior?
7. Can failure and cleanup be retried or diagnosed without guessing external state?
8. What happens under concurrent calls, cancellation, partial failure, process death, and retry?
9. Does the change accidentally turn an implementation detail into a compatibility promise?
10. If the change is breaking, is the incompatibility deliberate and documented rather than incidental churn?

## Architecture rule

Prefer the current vocabulary: Workspace, WorkspaceLease, Environment, Execution, CapabilityRequest, PolicyDecision, and ApprovalRequest.

Historical `Session`, Runtime/Storage-centric, and plugin-heavy code may remain as migration inventory, but new public APIs and architecture should not deepen those old couplings.

Provider-specific concepts stay outside Core. Incus, Git/GitHub, AWS/EC2/EBS, IDE brands, Btrfs/QCOW2, and orchestrator-specific models belong behind their respective boundaries.

## Compatibility rule

Do not preserve accidental behavior solely because it already exists.

A breaking change can be the correct choice when it:

- fixes an unsafe or ambiguous contract;
- removes authority leakage;
- makes recovery/failure semantics explicit;
- separates provider-specific behavior from Core;
- deletes an abstraction that no longer earns its complexity.

When breaking behavior, document operator-visible impact, avoid silent data loss, update tests, and provide migration guidance when a supported migration path exists.

## Security review

For security-sensitive changes, review the code adversarially rather than only validating the happy path. Follow `.github/security/ADVERSARIAL_AUDIT.md` for a full hostile audit.

At minimum consider:

- path traversal / symlink / TOCTOU;
- shell, argument, and option injection;
- authorization and confused-deputy failures;
- secret exposure through argv/env/log/tempfiles;
- concurrent and duplicate operations;
- timeout/cancellation/process crash;
- destructive retry and partial cleanup;
- hostile backend/provider/guest output.

## Definition of done

As applicable, add positive tests, negative/security tests, cleanup/retry tests, concurrency/race coverage, no-secret logging checks, and regression tests for retained behavior.

Run the maintained baseline:

```bash
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/haco
python tools/check_docs.py
```

Run relevant integration/E2E coverage as well. Do not claim real Incus or real AWS acceptance unless those provider-backed tests actually ran.
