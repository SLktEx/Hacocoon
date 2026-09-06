# ADR 0011: Switch Base through canonical Environment lifecycle operations

Status: accepted  
Date: 2026-09-06

The PoC product client offers `haco env switch-base --base <base> <environment>`.
It requires a managed Workspace, resolves the destination Base before mutation,
gracefully stops the Environment, and calls canonical controller delete/create
operations with the same Workspace, access mode and resource budget. Delete
releases the lease only after the runtime is positively absent. Repository
volumes and their independent Git state are not deleted or copied again.

The Environment root filesystem and connections are disposable. The caller
reconnects Git and SSH after switching. An error stops the sequence; uncertain
deletion cannot authorize creation. Failed creation retains Workspace data and
reports a recreate command. The operations are not an atomic transaction;
interruption and automatic recovery remain deferred. Concurrent acquisition
can make recreation fail, but cannot create overlapping Workspace leases.

Rejected: editing Base metadata in place, swapping live mounts, reconstructing
low-level state/lease mutations in the client, or re-cloning from upstream and
losing local work. No second controller or provider-specific Core API is added.
