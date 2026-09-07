# Restore absent source guards before resuming an owned Environment

Status: accepted; repository regression covered, real reboot acceptance pending.

## Context

Local installed acceptance on 2026-09-07 found that WSL restart removed the
per-Environment nft table. The stopped Environment's canonical start refused it.
This failed closed but made ordinary daily resume unusable after reboot.

## Decision

Under the canonical lifecycle lock, verify the stopped Environment marker,
dedicated network ownership, port isolation, exact NIC/managed MAC and subnet.
Restore an absent volatile source table before starting that guest and then verify
the complete guard. An existing table must match; never delete/rebuild drift.
A running Environment cannot request this restoration. Unexpected inspection
failure remains an error. Post-start verification failure stops the resumed guest.

New creation also reuses only an exactly matching guard instead of deleting an
existing table. Failed partial installation returns an error; it cannot start a
guest. Host administrators remain trusted, but the runtime never relies on a
guest-supplied identity or a caller's claimed ownership.

## Rejected alternatives and acceptance

Starting first and repairing later creates an isolation gap. Disabling source
checks after reboot loses identity enforcement. Replacing any existing table
would hide drift. All three are rejected.

Tests cover missing stopped guards restored before start, missing running guards
refused, existing drift refused, foreign ownership and post-start failure.
The local failure is observed evidence; a successful test-only recreation is not
acceptance of the resume fix. Windows restart acceptance of the fix remains pending.
See [managed networking](../design/managed-sandbox-network.md).
