# ADR 0048: Reclaim allocation without a second storage lifecycle

Status: in progress; no public reclaim command yet.

Use Incus's pool and mount model. Native Btrfs discard and Windows VHD compaction
must preserve data and capacity; they do not implement object deletion or generic
pool shrinking. Keep logical capacity and actual allocation separate.

Authorize the configured Hacocoon target, then pin and verify native filesystem,
loop and backing-file handles. Path names or a successful doctor observation alone
cannot authorize a mutation. Refuse symlink/hardlink ambiguity, replaced identities,
unknown layouts and unsupported kernel guarantees. Do not weaken checks for WSL
or independently mount an Incus pool to make a probe pass.

The Windows continuation must target one verified distribution/disk and record
partial failure across its stop/compact/resume boundary. No automatic backup,
rollback snapshot or full runtime-recovery framework is introduced.

See [storage reclamation](../design/storage-reclamation.md). The current internal slice pins identity and performs Btrfs/outer ext4 discard.
Outer discard requires separate managed-distribution authorization, beyond the
pool permission. Windows measurement/compaction now have internal native acceptance;
the public all-layer entry remains pending; Linux kernel trim counts are not Windows recovered allocation.

Windows measurement uses native file/ancestor handles and explicit sharing
exclusions. Attribute-only opens do not enforce the rename exclusion required by
this design; native regression caught that failure. File length and physical
allocation remain separate. The actual WSL registration and virtual disk must
still be bound before mutation. This read-only primitive is not that authority.

The initial native compaction attempt failed with a sharing violation. Subsequent
native fixture and dedicated WSL compaction passed with the same pins held,
disproving the earlier claimed incompatibility. Do not release pins or introduce
a path race. Only native open sharing violations receive a bounded pre-mutation
wait; compaction is never retried. The initial 30-second immediate-stop attempt
failed. A read-only probe then observed natural readiness after about 58 seconds
on WSL 2.7.13; a 90-second budget allowed immediate-stop compaction to pass.
This budget never authorizes stopping unrelated distributions or altering the
shared VM idle timeout. Preserve each attempt's result and resume the exact
registration. A registration GUID/value observation is not an authorization
record: the public continuation must still bind the managed installation and
pinned file at mutation boundaries, and reject changed/reused identities.

The local composition selects its configured pool without caller paths. The
Incus entry binds its exact standard backing layout, configured mount policy and
live native objects, rereading correspondence before each discard. Unsupported
custom layouts fail closed; no independent mount or pool-creation fallback is
introduced. The entry owns handles only, so closing it never deletes storage.

Use fixed GUID-bound WSL launches for shutdown and resumption. The installed
WSL terminate command resolves a name again, so it is unsuitable for the
identity-bound sequence. Systemd poweroff inside the selected distribution,
followed by native detached-disk validation, preserves the existing boundary.
Retain disk/ancestor pins throughout and clear inherited execution context.
After any stop attempt, attempt bounded same-GUID resumption independently of
caller cancellation while preserving both failures. This is stop-release
protection, not complete Environment recovery. Native sequence acceptance
passed; durable operation ownership and managed-installation authorization
remain mandatory before public activation.

Exclude live continuations with an exclusively created Windows kernel object
keyed by user SID and WSL GUID. Keep its noninheritable handle through resume
and pin cleanup; an existing object always refuses, without waiting or takeover.
The global namespace covers sessions, while its protected DACL limits access.
Do not confuse automatic kernel-handle cleanup with a successful operation or
durable recovery record. This adds no storage lifecycle or Core interface.

Persist the last operation's exact registration/file identity and stage observations
outside WSL before requesting shutdown. Refuse pending/failed or unrecognized
records, without automatic replay, rollback or deletion. A completed record can
be superseded only for the same target. Registry flush is used at intent, Linux handoff/result and terminal-result
boundaries; it is not a general progress store. This record does not grant installation
authority and requires explicit interrupted-state handling before public use.

Preserve the legacy name-only interop record and capture a separate root-owned
registration/installation identity during normal Windows installation. Resolve
the name once for common setup, use the GUID, then reject changed correspondence.
Capture cannot overwrite a changed or malformed existing identity. This Linux
record is only one enrollment component; Windows file/owner binding must still
be enforced before mutation. Do not treat a copied root record as authority.

Only explicit installation enrollment persists the observed Linux installation,
WSL registration, Windows user and pinned VHDX identity. A mutation must require
that saved tuple and never bootstrap it from the target it is about to mutate.
Keep enrollment and interrupted-operation results separate; neither implicitly
acknowledges or overwrites the other. Reject changed/unknown bindings rather than
silently adopting a recreated distribution or disk. Controller request authority
and the packaged installer entry remain separate requirements.

For explicit worker handoff, prepare the existing pending record under the same
saved enrollment and native exclusion. Execute only its exact operation ID after
reacquiring and revalidating all native/installed identities. Reuse the synchronous
sequence and durable result, not a second state machine. A record ID is not
controller authority; never discover and replay an interrupted operation. Pending
launch failures retain evidence for explicit review rather than auto-recovery.

The internal worker launcher reuses native file pins for its own executable,
excludes executable write/delete sharing, and launches only a fixed self-worker
with detached/job-breakaway flags and cleared context. Dispatch is not completion;
the existing operation record remains the result authority. Read-only inspection
must not start WSL or create/acknowledge a record, and pending has unknown outcome.
The worker refuses an attached console before WSL access. Explicit Job breakaway
remains mandatory at launch, but a remaining outer Job is allowed: membership
alone does not grant WSL, enrollment or operation authority. An outer Job may
terminate the worker; retain the pending record and require explicit inspection.
This does not guarantee survival of arbitrary Windows process supervision.
Do not claim WSL shutdown survival from a successful process launch.

The launcher waits for a bounded private readiness frame and EOF after target
validation, before WSL stop. This separates startup refusal from dispatch success
without adding recovery states. Cancellation or timeout does not cancel the worker
or clear the durable intent; completion still comes from the recorded result.

Terminal failure review retains the original canonical result under its operation
GUID before allowing a new intent, under the existing continuation exclusion and
enrolled target checks. Do not equate acknowledgement with successful compaction,
delete evidence to unblock retry, or allow this path for pending work: a delayed
launcher could still execute that exact pending operation. Preserved failures stay
readable by ID. This adds an explicit acknowledgement boundary, not automatic replay
or a generic recovery state machine.

Allow discovery of the current persisted result only through the read-only status
path. Mutating continuation and review still require the exact operation ID.
A missing/malformed current result never causes enrollment, history fallback or
replay. This makes shutdown result inspection possible without adding operation
states or treating discovery as execution authority. Dedicated native worker
acceptance now covers stop/compact/same-registration resume; public all-layer
activation and interrupted-pending handling remain separate incomplete work.

The Linux-stage connection belongs to the existing controller management endpoint.
It accepts an exact installed WSL identity, never target paths or pool selection.
Compare the root-owned installer binding before selecting the configured pool and
again before outer-filesystem discard. This authorizes only those Linux stages;
a successful RPC report never substitutes for Windows enrollment/disk validation.
Keep per-stage results transient and explicit, including skipped work and cleanup
failure, rather than creating another recovery journal. The fixed internal client
bridge returns nonzero for failed reports and never retries mutations. Filesystem
statfs accounting, Incus backing-file allocation and Windows VHD allocation remain
distinct measurements. Guest Git/notification endpoints must not register this API.

The Windows worker now consumes the Linux report under the same enrolled-target
exclusion. Version-2 intents record Linux-started before the subprocess and its
bounded typed result before Windows shutdown. Missing/failed/unconfirmed Linux
completion prevents shutdown; a started pending record cannot replay that call.
Recheck installation identity before stop. Keep version-1 bytes and disk-only
meaning intact, including reviewed failures; reject unknown schemas rather than
discarding data. New preparation uses version 2 with no bulk migration. These
observations preserve operation evidence across WSL shutdown; they do not grant
authority or promise automatic resumption. Public activation remains separate.
