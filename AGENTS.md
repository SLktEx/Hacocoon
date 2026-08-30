# Hacocoon Agent Rules

These rules apply to the entire repository.

Hacocoon is a secure workspace runtime, not an AI orchestrator. Keep Core small, provider-neutral, and explicit about trust boundaries.

## Start here

Before changing code or documentation, read:

1. `docs/README.md` for documentation ownership and source-of-truth order.
2. `docs/IMPLEMENTATION_STATUS.md` for current repository reality.
3. The relevant architecture or versioned specification for the subsystem being changed.
4. `.github/security/ADVERSARIAL_AUDIT.md` for security-sensitive changes.

Do not add tool-specific handoff files, generated master documents, migration scratch notes, or duplicate "start here" documents to the repository root. Git history is the archive for obsolete implementation notes.

## Architecture

- Keep Workspace, Environment, Execution, Policy/Capability, provider, client, and plugin responsibilities separated.
- Provider-specific behavior belongs behind provider/adapter boundaries, not in Core conditionals.
- Optional integrations must remain optional. Core must not gain mandatory dependencies merely because one maintained plugin uses them.
- Do not give untrusted workloads Hacocoon/Incus management authority, reusable Host credentials, Host control sockets, or protected Hacocoon state.
- Hacocoon is pre-1.0. Prefer deletion, replacement, and clear ownership over preserving accidental compatibility.

## Documentation

`docs/README.md` defines the authoritative documentation order. Avoid creating another index that duplicates it.

When behavior changes:

- update the owning specification and `docs/IMPLEMENTATION_STATUS.md`;
- update `docs/00D_VERSIONING_AND_RELEASE_STATUS.md` only when an independently useful product feature consumes a new milestone;
- keep English/Japanese companion documents aligned when both exist;
- describe current behavior as current, and historical behavior as historical or recoverable from Git history;
- do not keep stale implementation snapshots solely for handoff convenience.

Run `python tools/check_docs.py` after documentation changes.

## Adversarial security review

Treat all external input and backend/plugin/guest output as untrusted. For security-sensitive changes, consider at minimum:

1. path traversal, absolute paths, symlinks, hardlinks, and TOCTOU;
2. shell, argument, and option injection, including values beginning with `-`;
3. authorization, ownership, capability, and confused-deputy bypasses;
4. secrets leaking through argv, environment variables, logs, or temporary files;
5. duplicate/concurrent operations and races with inverse operations;
6. timeout, cancellation, process crash, retry, and partial failure;
7. cleanup failure and destructive-operation target confusion;
8. hostile or malformed backend, provider, plugin, or guest responses.

Security-sensitive operations should fail closed. Do not use "internal API", "localhost", "single-user", "inside the VM", or caller-side validation as a security argument.

When fixing a vulnerability or serious bug, add a regression test when practical.

## Validation

Run the narrowest relevant tests while iterating, then run the maintained local CI entry point when practical:

```bash
bash tools/ci-local.sh
```

Keep repository tests distinct from real-host/provider acceptance. Do not claim real Incus, private-registry, Docker compatibility, Windows/WSL, or future cloud acceptance unless those environment-dependent checks actually ran.
