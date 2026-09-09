# ADR 0047: Inspect retained Stores in disposable Environments

Status: partial implementation; public commands and native acceptance pending.

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

Planned: prepare the disposable runtime before connecting retained data, so its
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
