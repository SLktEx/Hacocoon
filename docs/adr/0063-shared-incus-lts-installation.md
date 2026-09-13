# ADR 0063: Shared Incus LTS installation

Status: accepted.

## Context

Issue #479 identified a mismatch: standalone real-Incus CI installed Zabbly 7.0 LTS while
the Ubuntu/WSL product installer and separate Core/Btrfs CI setup selected Ubuntu's 6.0 package. Historical 6.0.5
acceptance does not establish the intended 7.0 baseline.

## Decision

The supported server range is `>= 7.0.1` and `< 7.1`. Product installation and
disposable CI call the same packaged [helper](../../scripts/incus-lts.sh).
It verifies the pinned Zabbly primary key, rejects additional keys, configures
the `lts-7.0` source, and selects the greatest supported package version
available from that exact source. Persistent APT preferences allow all
`1:7.0.*` patches and exclude other series for the Incus packages. A version
selected for one installation is not a permanent patch pin.

Installation needs Physical Host administrative authority. No repository key or
management credential is exposed to ordinary Environments. The helper does not
initialize Incus, create resources, delete data, alter instance confinement or
grant a user management access. Existing installer and runtime lifecycle owners
retain these responsibilities.

An installed newer series is refused before source changes instead of being
automatically downgraded. Existing 6.0 compatibility paths remain, but `haco
doctor` reports unsupported servers and skips dependent probes. The actual
server version must also pass the shared installer/CI check. Unknown or malformed
versions fail closed without printing raw backend text. Fresh native installation
acceptance is separate from command-boundary and diagnostic regressions.
The [acceptance record](../status/acceptance-evidence.md#incus-lts) distinguishes
the integrated development candidate from its main-targeted extraction.

## Rejected alternatives

- Separate CI and product logic lets their trust/version contracts drift.
- A permanent patch pin prevents routine security/bugfix updates.
- Following `stable`, or trusting only the first downloaded key, expands the
  selected series or signing authority without an explicit decision.
- Deleting 6.0 compatibility now conflates migration with later cleanup.

References: [Zabbly package instructions](https://github.com/zabbly/incus) and
[Incus installation documentation](https://linuxcontainers.org/incus/docs/main/installing/).
