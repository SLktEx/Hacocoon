# ADR 0047: Inspect retained Stores in disposable Environments

Status: partial implementation; native primitives accepted, public maintenance integration pending.

## Decision

Keep a detached guest Store outside the trusted Host. Use a scratch Environment
through canonical lifecycle and ephemeral-run ownership, then remove only that
runtime. Incus owns attachment and runtime operations; the OCI runtime owns image
inventory, references and deletion. Do not add an image database or layer cleanup.

A Workspace-associated Store may be reserved for a scratch run only when the
catalog contains a creating ephemeral-run record matching the exact Environment
and temporary Workspace identity. Check that record in the same transaction as
the reviewed Store owner, ready/non-source state, copy reservations and exclusive
lease. A temporary-looking name alone is insufficient. Ordinary Workspace
associations stay strict. Never rewrite the original Store's Workspace binding.

Retain canonical creation receipts and uncertain cleanup leases. Finalizing the
verified absent temporary runtime does not remove or rebind the retained Store.
Current permission generations and network guards remain mandatory.

## Runtime preparation

Implemented as an independent primitive; public integration is planned. Prepare
the disposable runtime before connecting retained data, so its
ordinary daemon startup cannot run stored containers merely for image inventory.
Do not mount guest Store data into the trusted Host. Do not mutate container
restart policies or hide an automatic backup/copy-and-swap implementation.
Docker daemon startup can automatically remove containers and volumes even with
its deprecated restart flag disabled; that flag alone is not adequate protection.
Unsupported native layouts/features must be refused explicitly, not guessed safe.

## Validation and compatibility

The focused catalog regression covers missing/different/active run records, Host
paths, read-only misuse, source-only resources, stale Store owners, conflicting
normal leases, deletion exclusion, ambiguous cleanup and retained data identity.
Before implementation, only its valid maintenance case failed; refusal cases
passed. Actual runtime acceptance remains separate. No schema change or data
migration is introduced by the catalog reservation rule.

The independent Incus preparation primitive passed focused unit tests and an
opt-in WSL Incus/Btrfs fixture in 57.21s. It prepares before attachment, refuses
late preparation, preserves a sentinel across restart with the fixture service
masked, retains the Store after runtime deletion, and cleans up only its owned
instance/volume. The shared image/pool and ownership receipt remain. This tests
real systemd/Incus preparation, not detached Docker/nerdctl image operations.
Catalog integration now validates admitted leases using the same exact scratch
identity in creating, active and cleanup-required run states. Admission still
requires creating. Put/delete of run evidence must preserve all persistent
resource invariants before writing; a failed cleanup cannot lose its supporting
record. Focused state/workspace/run race tests passed, including the formerly
failing positive paths and evidence replacement/deletion refusal. No public route
or daemon integration is enabled.

Until daemon startup and attachment are integrated, every Incus Environment
creation entry explicitly refuses maintenance requests before native access.
The independent preparation primitive is not an enabled maintenance lifecycle.

## Containerd metadata-only startup

An independent primitive targets pinned containerd 2.3.3. It checks
native Store ownership, a single exact consumer, the current Environment generation
and its unprivileged local mount before starting a transient guest service. The
ordinary daemons remain masked. A fresh private guest `/run` directory contains
explicit configuration and runtime state; retained configuration is never loaded.
Restart, CRI, NRI, task service/runtime and sandbox controllers are disabled.
Container records and restart labels are not rewritten. The dedicated Incus
6.0.5/Btrfs primitive test passed in 177.86s, including task API refusal, unchanged
restart-marked container metadata, retained used image, unused-alias deletion,
Store retention after runtime deletion and exact cleanup. Inactive plugin entries
are distinguished from loaded plugins. Fixture import config explicitly selects
the native unpack platform. Public maintenance creation remains unsupported.

The [containerd restart monitor](https://github.com/containerd/containerd/blob/v2.3.3/plugins/restart/monitor.go)
can start containers from persisted labels. Its
[plugin filter](https://github.com/containerd/containerd/blob/v2.3.3/cmd/containerd/server/config/config.go)
uses exact identities, so wildcard disabling is not a substitute for the explicit
list. Docker startup is a separate unresolved path; this does not assert Docker
safety or expose an arbitrary socket/executable option.
