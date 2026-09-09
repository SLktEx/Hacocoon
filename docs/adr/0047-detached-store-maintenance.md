# ADR 0047: Inspect retained Stores in disposable Environments

Status: partial implementation; native primitives accepted, public routing partial; automatic tooling and composed native acceptance pending.

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

Implemented through receipt-based SandboxProvider creation. Prepare
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
failing positive paths and evidence replacement/deletion refusal. Public image routing now uses the canonical maintenance callback.

The SandboxProvider receipt-based create path now composes preparation before
attachment and metadata startup after current network guards and verification.
It requires a temporary Workspace and an owned ready guest OCI Store. The caller
records exact runtime ownership immediately after init and owns failure cleanup.
Runtime/BaseProvider, receipt-free creation and snapshot restore cannot enter
maintenance. Public image routing uses this path; default tooling provisioning remains incomplete.

The existing run service holds its ownership lock across a maintenance callback
and bounded cleanup. It pins the reviewed resource owner in canonical creation,
skips a default Store copy, and verifies the returned binding before executing.
It reuses the ordinary run marker, cancellation and cleanup-required handling;
only the scratch Workspace's own resources are eligible for temporary cleanup.
No new state machine or borrowed-Store deletion path is introduced.

## Containerd metadata-only startup

An independent primitive targets pinned containerd 2.3.3. It checks
native Store ownership, a single exact consumer, the current Environment generation
and its unprivileged local mount before starting a transient guest service. The
ordinary daemons remain masked. A fresh private guest `/run` directory contains
explicit configuration and runtime state; retained configuration is never loaded.
Restart, CRI, NRI, task service/runtime and sandbox controllers are disabled.
Container records and restart labels are not rewritten. The dedicated Incus
6.0.5/Btrfs primitive test passed in 179.66s, including task API refusal, unchanged
restart-marked container metadata, retained used image, unused-alias deletion,
Store retention after runtime deletion and exact cleanup. Inactive plugin entries
are distinguished from loaded plugins. Fixture import config explicitly selects
the native unpack platform. Public image routing is connected; the receipt-based create sequence
is covered by focused adapter regressions, not yet full native integration acceptance.

The [containerd restart monitor](https://github.com/containerd/containerd/blob/v2.3.3/plugins/restart/monitor.go)
can start containers from persisted labels. Its
[plugin filter](https://github.com/containerd/containerd/blob/v2.3.3/cmd/containerd/server/config/config.go)
uses exact identities, so wildcard disabling is not a substitute for the explicit
list. Docker startup is a separate unresolved path; this does not assert Docker
safety or expose an arbitrary socket/executable option.

## Reviewed public target

The existing list/delete target accepts a retained `oci:` Store ID. Reviews retain
only its exact owner and runtime, never a temporary Environment name/generation.
Each operation acquires one run, then scopes all runtime calls to its new generation
and fixed metadata socket. The canonical run service owns reservation, cancellation
and uncertain cleanup. The OCI module owns inventory and non-force deletion.
No new lifecycle coordinator, schema state or CLI operation is introduced.

Plain Ubuntu does not supply compatible containerd/ctr/nerdctl executables.
Automatic provisioning and installed-controller native acceptance remain required
before claiming the default public workflow complete. Detached Docker is refused.
The native image fixture exercises product OCI commands with fixture catalog/run
identities; it cannot establish complete canonical creation or tool delivery.

The expanded native fixture passed in 224.64s, including product image operations
on the fixed metadata socket, actual referenced-digest refusal, selected unused
digest deletion/absence, unchanged container records and exact owned cleanup.
It preserves every unselected used-image digest, without assuming that all digests
with the same displayed tag are referenced by a container.
