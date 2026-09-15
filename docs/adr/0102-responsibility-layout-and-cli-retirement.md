# Responsibility layout and CLI retirement

Status: accepted. Refs #654. Based on main `ee8bf7fbd2e858f9b6fd1d22ecce77fdde6bf082`.

## Decision

Repository paths identify the feature or external system that owns the code.
Core, Standard and Plugin describe architectural roles, not parallel directory
trees. Use ordinary Go packages and explicit composition; do not add compatibility
aliases for moved internal packages. [The repository map](../../CONTRIBUTING.md#repository-map)
owns navigation; [extension architecture](../design/plugin-architecture.md) owns
the architectural rules.

All shipped binaries retain independent thin entries in `cmd/`. The current
product is built from `cmd/haco`, with parsing and presentation in `internal/cli`.
The previous `hacoq` binary is removed from source, release archives and installers.
Its direct GitHub capability, optional Docker status/prepare service and
`HACO_PLUGIN_OCI` composition selector are retired. Current managed Git,
OCI Store/image operations, controller APIs, notifications and client adapters
remain separate supported implementations. Generic capability/event APIs remain
because current client and notification consumers use them.

The retired Session manager, its private JSON store, unused runtime/storage
contracts and Incus Session create/exec entry points are removed with their
obsolete tests. Current Environment state and lifecycle ownership have separate
catalogs and canonical transitions; they do not read that Session store. The
unregistered `switch-base` implementation is also removed. Its existing CLI
refusal remains, with no replacement operation or automatic data migration.

The Agent Host helper also uses one explicit command dispatch. Its `init`-time
interceptor, duplicate prepare parser, stdout capture and legacy-output adapter
are removed. Prepare and lookup render the same typed session descriptor directly;
the existing helper binary and trusted session broker remain separate from Core.
SSH grant rotation compares the Environment incarnation, Workspace, access mode
and service while allowing a new grant ID. Comparing the whole target would refuse
every grant replacement; comparing only its human-readable name would allow a
different owned target. The previous grant is revoked only after the replacement
configuration is ready.

Environment creation has one Router implementation that preserves the provider's
Base identity, effective resource budget and ownership receipt. The separate
BaseRouter creation override is removed; Base catalog and snapshot/archive
operations use the same Router. Unreferenced disabled/no-finite-resource provider
wrappers and the provider-level `PrepareSSH` compatibility alias are removed.
Current clients use `PrepareSSHAccess` and its paired revocation contract; public
client-adapter APIs and readers needed to clean up retained ownership stay intact.

The stored route format is `haco-runtime-v1:<provider>:<base64url-native-ref>`.
Provider IDs containing `:` are rejected during registration because the parser
uses that separator to identify the owner; accepting one would create an
unresolvable ownership record. Native references remain opaque. Base publication
requires equal persisted Env and lease routes before either is unwrapped. Rewriting
both from the Env alone would erase a mismatch before the provider validates the
lease. Regressions cover valid routes, unsupported and ambiguous registrations,
and publication with a different provider or native owner in the lease.

Installer source moves from `scripts/` to `install/`. Direct source URLs change;
there are no forwarding scripts. Release archive names and installer bundle
filenames stay the same. The installer rejects archives containing the retired
binary. This change does not erase an old installation's files or data.

Installed Linux and Windows/WSL journey drivers live under `test/e2e/installed`
and `test/e2e/windows`. Build, packaging, diagnostics and test-observer tools stay
in `tools/`. The two Windows restart forwarding scripts are removed: one install
driver owns initial install, restart and reinstall, with an explicit argument
selecting the shipped `-UseCachedWslImage` option. It does not monkey-patch terminal
writes or add product provisioning to the test path.

## Boundaries retained

Base build, asset retention and reviewed image cleanup remain separate packages.
Env routing, copy and transfer remain distinct from snapshot restore and the
canonical Workspace lifecycle. A saved snapshot is not owned by a live Env.
SSH configuration and public-key validation have distinct clients and provider
consumers; moving them does not give a workload management authority.

Git process/wire code belongs to its adapter. Repository ownership and approval
coordination remain with the Git feature. OCI download/extraction belongs to its
adapter; retained Store ownership remains with storage services. Windows/WSL
identity, enrollment and worker code stays separate from reclamation values and
the client that dispatches it. All creation receipts, exact-owner cleanup,
approval and isolation checks still apply across the new paths.

Existing retained Base/snapshot ownership records still need verified cleanup.
Those readers and cleanup checks are not disposable aliases: removing them would
orphan owned resources or invite adoption based on a name. They remain until their
data-lifetime contract is explicitly replaced.

## Rejected alternatives

- Keeping both CLI implementations or wrapping the old one from the product:
  this preserves two compositions and an obsolete authority surface.
- Merging helpers to reduce binary count: helpers have different execution
  locations, platforms and authority.
- Moving every Standard implementation into adapters: approval, cache selection
  and Workspace orchestration are product behavior, not external-system calls.
- Combining Env, snapshot and Workspace mutation services: their data lifetimes
  differ and canonical lifecycle transitions must remain the sole ownership writer.
- Claiming native acceptance from repository tests: packaged Ubuntu/Windows,
  Incus and authenticated service acceptance remain separate checks.
