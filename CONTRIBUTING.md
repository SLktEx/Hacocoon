# Contributing

Hacocoon optimizes for replacement, deletion and operational simplicity.

Before changing code, read `CODEX_START_HERE.md` and the current release specification.

## Design checks

A change should answer these questions:

1. Is this a stable Hacocoon concept, or a concrete provider/tool detail?
2. Can the implementation be deleted/replaced without rewriting Core?
3. Does the change preserve the Manager/Session trust boundary?
4. Does any long-lived parent credential enter the Session? If yes, the design is wrong.
5. Is a conditional encoding a real product rule, or should a strategy/module own the behavior?
6. Can failure be retried/reconciled without guessing provider state?

## Definition of done

As applicable, add positive tests, negative/security tests, cleanup/retry tests, no-secret logging checks, doctor coverage and prior-version regression tests.
