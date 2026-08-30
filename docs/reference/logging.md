# Logging

[**日本語**](logging.ja.md) | English

Hacocoon uses structured logging for operational diagnosis across Core, providers, networking, storage, plugins, and CI/E2E. Logging is observability infrastructure: it must make failures attributable without weakening Hacocoon's credential and trust boundaries.

## Principles

1. Log meaningful operations and state transitions, not a narration of every implementation detail.
2. Prefer structured attributes over embedding searchable state only in free-form messages.
3. A normal failure must be understandable at INFO/WARN/ERROR without requiring DEBUG.
4. DEBUG may add implementation detail, but it follows exactly the same secret-handling rules.
5. Logging must not change the outcome of an operation.
6. A failed operation is normally logged as ERROR once, at the boundary that owns/reports the operation. Lower layers return or wrap the error and may emit DEBUG diagnostics instead of duplicating ERROR entries.

## Levels

Hacocoon uses the standard `log/slog` levels:

- `DEBUG`: detailed diagnostics such as sanitized Host commands, retries, provider/backend steps, and internal state transitions.
- `INFO`: meaningful lifecycle events such as Environment create/exec/delete start and completion.
- `WARN`: degraded or fallback behavior where the requested operation can still continue.
- `ERROR`: the requested operation cannot complete successfully.

The default level is `INFO`.

Current executable configuration is environment-based:

```bash
HACO_LOG_LEVEL=debug haco doctor
HACO_LOG_FORMAT=json HACO_LOG_LEVEL=debug haco create --workspace /work demo
```

`haco`, `haco-vscode`, `haco-agent-host`, and `haco-notify` use the same configuration. Supported formats are `text` (default) and `json`. Logs are written to stderr so command output on stdout remains machine-consumable.

## Stable structured fields

Use an existing field name when it describes the value. Do not invent package-specific synonyms for the same concept.

| Field | Meaning |
|---|---|
| `component` | subsystem such as `core`, `incus`, `network`, `storage`, `git`, `oci`, `proxy`, or `host` |
| `operation` | stable operation name such as `create_environment` |
| `environment_id` | Hacocoon Environment identity |
| `runtime_ref` | provider/backend runtime reference when safe and useful |
| `backend` | selected provider/backend when needed for disambiguation |
| `duration_ms` | elapsed wall-clock time for the operation |
| `attempt` | retry/attempt number when retry behavior exists |
| `request_id` | stable request/capability correlation identity |
| `error` | sanitized error at the layer that owns the failure report |
| `exit_code` | child/Environment command exit code where relevant |
| `target_host` / `target_port` | normalized egress target, never a full URL/path/query |

Prefer stable identifiers over arbitrary object dumps, full filesystem structures, or unbounded provider output.

## Logger ownership and propagation

The executable entrypoint configures the process root logger. Internal packages do not create unrelated global loggers.

Operation-specific attributes are propagated through `context.Context`. A lower layer derives its logger from that context and adds its own `component` or other stable fields. This keeps one operation correlated across Core and the provider/Host command path without introducing a logging dependency into domain contracts.

```text
root logger
  -> operation context
      -> core
          -> incus / network / storage / git / oci / proxy / host
```

## Secrets and sensitive data

Secrets must never be written to logs at any level, including DEBUG.

This includes, but is not limited to:

- passwords and passphrases;
- access, refresh, bearer, approval, or session tokens;
- Git credentials and credential-helper output;
- SSH private keys;
- API keys;
- cookies;
- `Authorization` and `Proxy-Authorization` values;
- proxy credentials;
- credential-bearing URLs;
- environment variables or configuration values containing secrets.

Do not log complete HTTP headers, complete process environments, arbitrary configuration objects, request/response bodies, or raw child stdout/stderr merely for convenience.

Hacocoon's shared logging handler performs defense-in-depth redaction of known secret-shaped values. That redaction is not permission to pass arbitrary sensitive objects to the logger. Call sites must still select safe fields deliberately.

When uncertain whether a value is safe to log, omit it.

## Host command logging

Trusted Host commands may be logged at DEBUG when they materially help diagnosis. The shared Host runner:

- logs the executable and sanitized argv;
- classifies common commands into `incus`, `network`, `storage`, `git`, `oci`, or `host` components;
- records duration and exit code;
- does not automatically log captured stdout or stderr.

Never add a raw command-line log alongside the sanitized form. Arguments that may carry credentials must be omitted or redacted before emission.

## Errors

Do not log the same failure at every call layer.

Preferred flow:

```text
provider/Host layer -> return/wrap error + optional DEBUG diagnostic
Core operation boundary -> ERROR once with operation/environment/duration
CLI -> render the returned error for the user
```

An error that is retried, converted to fallback behavior, or otherwise handled is not automatically an ERROR event. If fallback materially changes behavior, WARN is appropriate.

Error values themselves can contain untrusted backend text. The shared logger redacts common credential patterns, but callers should avoid constructing errors that include secrets in the first place.

## Duration and external operations

Record `duration_ms` for operations where latency helps distinguish failure modes, especially:

- Environment create/exec/delete;
- Incus lifecycle operations;
- image acquisition and Seed construction;
- network/storage initialization;
- Git fetch/push;
- cleanup and recovery.

Do not add high-cardinality timing logs for trivial in-memory helpers.

## CI and E2E

CI logs should make it possible to distinguish at least:

1. runner/setup failure;
2. Incus substrate failure;
3. Hacocoon provider/backend integration failure;
4. Core Environment lifecycle failure;
5. networking/proxy/DNS failure;
6. storage or optional plugin failure.

JSON output is available for automation, but tests must not depend on the human-readable text format. Specialized CI diagnostic artifacts remain useful and are not replaced by application logging.

Enabling DEBUG in CI must not weaken redaction or secret handling.

## Adding or changing logs

Before adding a log event, check:

- Is this event operationally useful?
- Is its level consistent with this document?
- Can the same information be a structured field instead of message text?
- Does it duplicate an ERROR already owned by another layer?
- Could any value contain a credential, request body, private key, token, header, or arbitrary subprocess output?
- Will the field name remain stable enough for CI/debugging tools to consume?

Logging changes should include focused tests when they introduce a new redaction rule, field contract, format behavior, or failure boundary.
