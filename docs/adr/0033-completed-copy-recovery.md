# ADR 0033: Recover positively completed copies

Status: accepted; implemented; real provider/service recovery verified
Date: 2026-09-08

## Decision

The canonical persistent-resource lifecycle records `copy_completed` only after
the provider positively reports copy completion and verifies the exact owned
destination. A staged backend invokes this durable transition before restoring
source writers. The receipt retains `creating`, the exact `copy_source` and both
reservations. Publication requires the receipt and successful source restoration,
then atomically clears the receipt and reservation while marking the copy ready.

A restarted controller may finish a receipt-bearing copy without copying again.
The Incus adapter shares the Host operation lock, rechecks source/destination
ownership and attachment, and validates the exact versioned Host journal and
restart guard. Frozen or stopped owned Host instances can be started only with
that positive completion proof. Already running instances need no extra start.
Restoration clears the journal and restores the previous local/inherited autostart
setting; ambiguous responses retain recovery state. If only catalog publication
was interrupted after restoration, the verified ready result can be committed.

Ordinary Host setup/entry runs completed-copy recovery before reading or starting
the Host. Retrying Environment creation also recovers its existing completed
Workspace copy. No new mandatory command or argument is added. These routes use
the canonical service and atomic state transitions, not reconstructed metadata
or lease mutations.

## Unconfirmed operations

Object existence, an empty operation list or a lost operation ID is not proof of
copy completion. A missing receipt, legacy journal, foreign ownership or changed
guard fails closed. A crash before the receipt is durably written remains
recovery-required, including the gap after provider success but before receipt
persistence. This slice does not provide cancellation or recovery of an unknown
asynchronous Incus operation. Source/target reservations and writer guards remain
intact; do not clear them manually or recreate the destination by force.

The new receipt is an additive controller-owned field. Older controllers do not
implement its recovery protocol; switching controller versions with a pending
copy is not a supported recovery procedure.

## Rejected alternatives

- Publishing first would permit target attachment before source restoration.
- Recording completion before the provider reports success would resume writers
  while a copy could still be active.
- Inferring success from a destination volume or missing operation would adopt
  partial or uncertain work.
- Recopying on retry can overwrite independent guest data or create duplicates.
- Clearing ownership after an attempted resume loses the recovery authority.

## Validation

State/service regressions reopen the catalog, keep both reservations on failed
recovery, reject publication without a receipt and finish without another copy.
Provider regressions cover malformed/legacy journals, foreign owners/consumers,
changed guards, unknown states, failed resume/clear and inherited autostart.
The real owned-area extension injects a resume failure only after real copy and
receipt persistence, reopens state, recovers through the ordinary OCI setup
service and checks image identity/execution, independent data and exact cleanup.
This is provider/service acceptance; fresh installed CLI recovery remains a
separate acceptance scope.
