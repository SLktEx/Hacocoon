# ADR 0023: Matching Policy restrictions outrank grants

Status: accepted.

## Context

Roadmap D1 requires deny > ask > allow across matching Environment and global
rules. The previous evaluator returned the first matching rule, allowing an
earlier grant to hide a later restriction. Manually editing/reordering config
must not silently widen authority.

## Decision

Evaluate all matching explicit rules. Deny outranks require-approval, which
outranks allow. Matching predicates remain unchanged: Environment/resource
wildcards are explicit and every authority-sensitive attribute must match.
The policy default is used only when no explicit rule matches. For equal
decisions, the first matching reason remains the diagnostic reason.

## Consequences

This intentionally replaces first-match behavior in the pre-1.0 configuration.
An overlapping allow can no longer bypass deny or approval. Remove or narrow
the conflicting restriction explicitly when a different grant is intended.
No credentials, approval decisions or provider operations are changed by merely
reading a policy. Invalid config and unavailable approval/audit remain closed.

Rejected: first-match ordering, specificity-based allow overriding global deny,
and treating default deny as an explicit deny that blocks all explicit grants.
Tests enumerate all rule permutations and verify Environment scope and fallback.
Four persistent approval choices and their user interface remain separate D1 work.
