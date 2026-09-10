# ADR 0053: Register imported Workspace data without Git population

Status: accepted for the initial single-Workspace import service.

## Decision

Native import uses the existing RepositoryService ownership/publication sequence:
plan a destination, reserve its fresh owner, import, record created, inspect the
owned native volume, then publish ready. Normal clone/copy still uses its existing
Git population callback. Imported data skips that callback entirely, preserving
commits, dirty files, untracked files and guest Git configuration as data.

The Incus adapter shares bounded archive preparation and native volume import with
OCI Stores. It supplies the Workspace domain's fresh config before native creation
and refuses every existing destination name. Source labels never grant ownership.
No new lifecycle state, catalog, Base copy, automatic backup or replay is introduced.

This initial service registers one Workspace with an explicit credential-free
GitHub remote. Source local-file URLs and missing routing are not automatically
adopted. Public aggregate import still needs offline/explicit rebinding policy and
multi-Workspace composition; this service is not a complete migration command.
Registration does not contact the remote or create Git approval/credentials.

Creation or verification failures preserve the exact incomplete ownership record,
as normal RepositoryService creation does. Ready data uses canonical Workspace
locking, lease exclusion and owned deletion. Aggregate failure cleanup and an
explicit cleanup route for incomplete imports remain required before public import
can be published; incomplete records must not be called recovered or discarded.

## Alternatives

Do not import by clone/checkout, populate a trusted Host checkout from guest data,
reuse a source owner, or treat the source remote as a destination Host capability.
Do not add import flags to normal user creation merely to expose this internal step.
