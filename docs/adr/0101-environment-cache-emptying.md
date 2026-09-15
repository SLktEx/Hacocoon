

# ADR 0101: Environment cache emptying

Status: accepted for the development candidate.

## Decision

Emptying disposable data retains the Env's ownership, Workspace, OCI, common
generations and saved copies. Pin the exact stopped Env through its common lifecycle
and persist child resource state `clearing` before provider mutation. Commit `ready`
only after the provider verifies empty contents and ownership. Failure retains
ownership and fences normal start/access/snapshot/copy/delete until explicit retry.
Inverse operations use the same lifecycle locks and durable catalog guards.

Linux deletes only children through the Incus volume file API without following
links. Other providers must satisfy the same contract.

## Rejected alternatives

Deleting/recreating the volume loses the attached ownership target on partial failure.
Guest rm executes mutable workload code as the implementation of a management task.
Guessing Host mount paths crosses the provider boundary. Clearing the maintenance
state after an ambiguous result resumes work with incomplete contents. No old-format
compatibility mechanism is introduced.

See the [owning contract](../design/cache-generations.md#empty-an-environments-cache).
