# Base images and custom Environments

Status: partial. Logical Base selection is implemented. Definition-driven build
is implemented in this change; real acceptance results are recorded separately.

## Daily use

`haco base list` and `haco base inspect <name>` show starting points. Create with
`haco create --base <name> --workspace managed:<workspace> <environment>`.
Existing Environments retain their original immutable revision when a name moves.
No switch-base step is required.

To add a reusable tool, save this definition as `base.json`:

```json
{
  "name": "my-tools",
  "from": "haco/ubuntu-26.04",
  "run": "printf '#!/bin/sh\\necho hello-from-my-base\\n' > /usr/local/bin/my-tool\nchmod 0755 /usr/local/bin/my-tool\n"
}
```

```bash
haco base build base.json
haco base inspect my-tools
haco create --base my-tools --workspace managed:my-project dev
haco ssh setup dev
ssh haco-dev my-tool
```

`from` may be omitted for the normal default Base. The definition is a strict
JSON object with `name`, optional `from`, and `run`. Names are lowercase letters,
digits, dots and hyphens, start with a letter/digit and contain at most 63
characters. Script text is bounded to 64 KiB. Shell steps execute as guest root
inside an ordinary isolated Environment, never as Host commands. Network access
uses the ordinary permission path; package downloads are not implicitly allowed.
No private Workspace, OCI Store or Host credentials are supplied to the builder.

The command returns JSON containing the Base name/revision, state and any retained
builder name. Build output is not copied into controller logs or error messages.
A failure is nonzero. The destination name must differ from an explicitly selected parent. Concurrent
builds, caching, import and history UI are not
promised by this first build path.

## Incus owns images

Incus creates the builder instance and publishes its stopped rootfs as a private
image. It stores build ownership properties at image creation. The adapter
verifies the full fingerprint and ownership before updating an Incus alias for
the logical name. Older images remain; moving the alias only affects later creates.
There is no second Hacocoon Base catalog or filesystem retention object.
Publication produces a compressed image and is not claimed to be a cheap COW
operation. See [Incus image creation](https://linuxcontainers.org/incus/docs/main/howto/images_create/).

`modules/runtime/incus` owns native image/alias operations and pinned resolution.
`internal/basebuild` composes canonical temporary Env creation, guest execution,
stop, publication and bounded cleanup. `internal/workspace` holds existing
lifecycle locks and checks the unique temporary Workspace and creation lease.
`internal/environment` only routes native references; CLI/control transport
transfers the bounded definition. No generic builder backend or replay state
machine is introduced. See [ADR 0041](../adr/0041-incus-base-publication.md).

## Security and failure

Base contents are untrusted. They do not grant Incus management, Host mounts,
privileged mode, direct egress, credentials or prior approvals. Every create
applies current normal configuration and a new generation. Built-image creation
renews managed guest SSH identity before publication, using the same helper as
snapshot restore. Explicit built Bases
do not pass through the historical Seed replacement resolver.

Before publication, the builder removes instance SSH keys, machine IDs and
its scratch Workspace; Incus metadata templates are cleared. This cannot identify
arbitrary secrets deliberately written by a definition: do not bake credentials
into a Base. Hacocoon never injects reusable Host credentials into the builder.

Script/stop failure triggers bounded cleanup of the exact temporary Workspace
owner through canonical Env deletion. Publication uncertainty retains the builder
name and any native image/build alias evidence. A successfully published image
survives builder cleanup failure. Inspect `haco env list` and the reported native
image/alias before explicit cleanup; retry starts a fresh builder. No automatic
rollback, old-image deletion or crash replay is attempted.

## Existing Base and saved data

Official names include `haco/ubuntu-26.04` and `haco/ubuntu-24.04`. Operator mappings
in `HACO_INCUS_BASES_JSON` remain supported; build cannot replace these configured
names or the reserved official namespace. Native built images are scoped to the
Hacocoon Incus project. Image alias movement never rewrites saved `BaseRef` values.

No catalog schema change or manual saved-data migration is required. Old Base
assets and snapshot Base components retain their existing ownership/explicit
cleanup. New snapshot rootfs remains independent and never gains a Base image
retention dependency. See [snapshots](environment-snapshots.md).

## Remaining acceptance

Verify a representative WSL build, registered Base selection, ordinary create and
an added tool invoked through SSH. Keep repository tests, native Incus behavior and
Windows client acceptance distinct. Explicit retained-image cleanup and broader
migration/reclamation remain later roadmap work.
