# ADR 0063: Keep official package repositories as a narrow egress baseline

Status: accepted
Date: 2026-09-13

## Context

Ordinary Environments are default-deny for outbound network access. That remains
the security baseline, but it makes the official Ubuntu development Base awkward:
`apt update` and normal package installation require administrator Policy edits
before the Environment can even use the package repositories already selected by
the product Base. SSH first-use exposed the same problem when runtime provisioning
needed `openssh-server`.

Granting authority to the `apt` executable is too broad, while deriving authority
from guest `/etc/apt/sources*` is unsafe because an untrusted workload can edit
those files. Persisting product defaults into each operator `policy.json` would
also mix shipped authority with revision-bound administrator configuration and
would make upgrades depend on mutating user-owned Policy.

## Decision

Hacocoon has a product-owned baseline allow for the exact Ubuntu package
repository destinations used by the official Base contract:

- `archive.ubuntu.com` on HTTP 80 and HTTPS 443;
- `security.ubuntu.com` on HTTP 80 and HTTPS 443;
- `ports.ubuntu.com` on HTTP 80 and HTTPS 443.

These are ordinary `network.egress/connect` grants. They still pass through the
Standard proxy, trusted Host DNS resolution, public-address validation and pinned
connection path. No process identity, URL path or guest package-manager state is
used as authority.

The evaluator applies precedence in this order:

1. matching administrator rules and saved decisions, using the existing
   `deny > require-approval > allow` rule from ADR 0023;
2. the narrow product baseline above;
3. the configured Policy default, which remains `deny` when absent.

Therefore an administrator can explicitly deny or require approval for any
baseline package destination, globally or for one Environment, without changing
the shipped baseline. `haco config` continues to represent revision-bound
operator Policy only; the product baseline is not serialized into `policy.json`.

The baseline is destination-scoped rather than selected-Base-scoped. A custom or
built Base may legitimately retain Ubuntu package sources inherited from an
official Base, and the fixed destination grant does not become more powerful when
used by a different Base. It grants no third-party repository. Runtime edits to
APT sources never change the baseline.

## Rejected alternatives

- Trusting any connection made by `apt` would let a changed source list reach an
  arbitrary host.
- Reading guest APT sources and automatically extending Policy would let an
  untrusted workload mint network authority.
- Injecting these rules into `policy.json` would blur product defaults with
  operator-owned, revision-bound configuration and complicate upgrades.
- Restricting the grant to only the current official Base name would break built
  or derived Bases that preserve the same standard Ubuntu repositories without
  improving destination isolation.
- Allowing broad Ubuntu domains or wildcard ports would exceed the package
  transport contract.

## Consequences

A fresh installed Environment can run ordinary Ubuntu package update/install
operations without first editing Policy. Third-party repositories, arbitrary web
hosts and direct TCP egress remain on the normal deny/approval path. Explicit
operator restrictions still win over the baseline. Acceptance must exercise both
standard package success and a guest-added third-party APT source that remains
refused.

Issue #603 may separately preinstall OpenSSH in official Bases so SSH setup does
not depend on package networking at all; this ADR only defines general package
repository egress behavior.
