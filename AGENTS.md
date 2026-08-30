# Hacocoon Agent Rules

These rules apply to the entire repository.

Hacocoon is a secure workspace runtime, not an AI orchestrator. Keep Core small, provider-neutral, and explicit about trust boundaries.

## Start here

Before changing code or documentation, read:

1. `docs/README.md` for documentation ownership and source-of-truth order.
2. `docs/IMPLEMENTATION_STATUS.md` for current repository reality.
3. `docs/DOCUMENTATION_STYLE_GUIDE.md` before changing README files or documentation.
4. The relevant semantic design document under `docs/design/` for the subsystem being changed.
5. `docs/reference/logging.md` before adding or changing logs.
6. `.github/security/ADVERSARIAL_AUDIT.md` for security-sensitive changes.

Do not add tool-specific handoff files, generated master documents, migration scratch notes, or duplicate "start here" documents to the repository root. Git history is the archive for obsolete implementation notes.

## Architecture

- Keep Workspace, Environment, Execution, Policy/Capability, provider, client, Standard-component, and plugin responsibilities separated.
- Provider-specific behavior belongs behind provider/adapter boundaries, not in Core conditionals.
- Standard implementations may be shipped and enabled by default while remaining replaceable implementations of Core contracts.
- Optional integrations must remain optional. Core must not gain mandatory dependencies merely because one maintained plugin uses them.
- Do not give untrusted workloads Hacocoon/Incus management authority, reusable Host credentials, Host control sockets, or protected Hacocoon state.
- Hacocoon is pre-1.0. Prefer deletion, replacement, and clear ownership over preserving accidental compatibility.

## Logging

Follow `docs/reference/logging.md` for every logging change.

- Use the shared structured logger and stable fields instead of package-local global loggers or ad-hoc `log.Printf` diagnostics.
- Keep DEBUG subject to the same secret/redaction rules as every other level.
- Never log raw credentials, authorization/cookie headers, private keys, complete process environments, arbitrary config objects, or raw subprocess output merely for convenience.
- Log a failed operation as ERROR at the boundary that owns the operation; lower layers should normally return/wrap the error and use DEBUG diagnostics rather than duplicating ERROR entries.
- Preserve stdout for command results and machine-readable output; application logs belong on stderr.
- Add focused regression coverage when introducing a new redaction rule, structured field contract, format, or failure boundary.

## Documentation

`docs/README.md` defines the authoritative documentation order. Avoid creating another index that duplicates it.

Long-lived documentation addresses are semantic. Normal documentation filenames must not encode a product version, milestone, or arbitrary reading order. Put feature contracts under `docs/design/`, security architecture under `docs/security/`, terminology/reference material under `docs/reference/`, and roadmap/version authority under `docs/status/`. ADR sequence numbers under `docs/adr/` are the intentional exception because the number is part of ADR identity.

When behavior changes:

- update the owning design/reference/security document before summary pages;
- update `docs/IMPLEMENTATION_STATUS.md` for current repository reality;
- update `docs/status/versioning-and-release-status.md` when a meaningful development checkpoint consumes or changes a milestone;
- treat pre-1.0 minor milestones as lightweight progress checkpoints: partial work or pending real-host acceptance in an earlier milestone does not block advancing to a later minor;
- use `docs/reference/terminology-and-boundaries.md` for canonical vocabulary;
- preserve `docs/security/security-architecture.md` trust-boundary rules;
- keep English/Japanese companion documents aligned in the same change when both exist;
- use explicit `implemented`, `partial`, `planned`, `deferred`, or `historical` wording;
- keep repository implementation claims separate from real-host/provider acceptance;
- treat README files as entry points and routing pages, not competing sources of truth;
- describe historical behavior as historical or recoverable from Git history rather than retaining stale duplicate snapshots.

When moving a document, update every inbound relative link in the same change. Prefer repository-relative Markdown links and current copy-pasteable commands.

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
