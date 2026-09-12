# Documentation

[日本語](README.ja.md) | English

## First use

Follow [getting started](guides/getting-started.md) from installation through Environment
creation, connection, development, stop and resume. No design reading is required first.
Hacocoon is pre-1.0; check [available scope and limits](IMPLEMENTATION_STATUS.md).

## Task guides

- [Install and resume interrupted installation](guides/installation.md)
- [Git, repository collections and push approval](guides/git-workflow.md)
- [Stop, delete, recreate and understand retained data](guides/data-lifetime.md)
- [Transfer an Environment](design/environment-transfer.md#commands) and [review data evacuation/migration](guides/data-evacuation.md)
- [Reclaim allocation and inspect results](design/storage-reclamation.md#public-dispatch-and-result-inspection)
- [Host/project setup](design/project-setup.md), [Web preview](design/development-preview.md), [temporary execution](design/temporary-execution.md)

## Concepts

- [Host, Workspace, Environment, Base, OCI Store and data lifetime](guides/data-lifetime.md)
- [Canonical vocabulary and responsibilities](reference/terminology-and-boundaries.md)
- [Design principles](DESIGN_PRINCIPLES.md) and [security boundaries](security/security-architecture.md)

## Command and configuration reference

- [Current CLI](reference/cli.md) and [legacy CLI migration](reference/cli-migration.md)
- [Configuration and approval policy](reference/configuration.md), [egress authorization](design/egress-authorization.md), [AWS operations](design/aws-operations.md)
- [Client API](reference/client-adapter.md), [interaction events](reference/interaction-events.md), [logging](reference/logging.md)
- [Build and release identity](reference/build-release-identity.md)

## Internals and contribution

Start with [contribution policy, build and CI](../CONTRIBUTING.md).
Read the relevant [feature design](design) and [ADR](adr) for the subsystem you change.
Useful architectural entry points are [Workspace leases](design/workspace-abstraction-and-lease.md),
[controller transport](design/controller-client-transport.md), [trusted Host](design/trusted-host.md)
and [Core/Standard/Plugin boundaries](design/plugin-architecture.md).

[Documentation roles, authority and update rules](DOCUMENTATION_STYLE_GUIDE.md) are maintained
in one place. For publication use the [release checklist](guides/releasing.md) and
[release security](security/release-security.md).

## Status and plans

- [Implementation status](IMPLEMENTATION_STATUS.md): features, available scope, limits and remaining work
- [Acceptance evidence](status/acceptance-evidence.md): real-host results, failures and gaps
- [Roadmap](status/architecture-and-roadmap.md): unfinished work and future direction
- [Versioning and release status](status/versioning-and-release-status.md): checkpoint policy and history
