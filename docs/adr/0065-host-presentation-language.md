# ADR 0065: Pass a normalized Host presentation language

Status: accepted for implementation

Date: 2026-09-13

## Context

The WSL client and trusted Host may have different OS locales. Forwarding arbitrary
environment variables or changing installed OS locales would affect credentials,
tool behavior and child workloads merely to translate Hacocoon's messages.

## Decision

The CLI resolves a presentation value once for each Host-shell request. Explicit
`HACO_UI_LANGUAGE=en` or `ja` takes precedence over POSIX locale selection; an
unsupported nonempty override selects English. The management API accepts only
empty, `en` or `ja`, rejecting other values before Host preparation. The hint lives
in the existing request/session terminal metadata, never controller-global state.

The Incus adapter supplies only the normalized value as `HACO_UI_LANGUAGE` to
that trusted Host shell. It does not forward this hint into an Environment shell,
change `LANG`/`LC_*`, persist instance configuration or inspect an arbitrary client
environment. Existing catalog selection, JSON values, exit codes and approval
semantics are unchanged. Old clients may omit the hint.

## Rejected alternatives and remaining work

Do not copy the caller's complete environment, initialize the OS locale from the
Windows display language, or localize controller protocol values. The hint is
presentation data, never authorization. Windows automatic language detection and
full CLI translation remain separate M1 work; explicit WSL invocation and local
transport regressions do not establish native Windows/Incus acceptance.
