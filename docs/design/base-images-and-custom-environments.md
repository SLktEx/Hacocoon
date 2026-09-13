# Base images and custom Environments

Status: implemented for the representative definition-build workflow. Native
Incus/Btrfs and Windows-to-WSL SSH acceptance passed at `a2fcb72`.

## Daily use

`haco base list` and `haco base inspect <name>` show starting points. Create with
`haco env create --base <name> --workspace managed:<workspace> <environment>`.
Existing Environments retain their original immutable revision when a name moves.
No switch-base step is required.

Hacocoon official names such as `haco/ubuntu-26.04` resolve, after Host setup, to
an immutable private image produced by the same canonical Base Builder lifecycle
used for explicit definitions. The upstream configured Ubuntu image is only the
trusted parent used to rebuild that official revision. Official publication does
not introduce a second image builder or publish directly from the installer.
Official Ubuntu revisions include OpenSSH server, so ordinary SSH preparation does
not need to install packages in a newly created Environment.

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
haco env create --base my-tools --workspace managed:my-project dev
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

The command prints a human-readable result. Pass `--json` to read the Base
name/revision, state and any retained builder name as JSON. Build output is not
copied into controller logs or error messages.
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

Official Base bootstrap reuses this exact lifecycle. Its definition and package
hosts are fixed by trusted product code, its temporary package-network grant is
bound only to the exact Builder Environment and removed after the build, and the
result is still published by the normal Incus Base publication operation with the
same ownership and immutable-revision checks. A trusted `ParentBaseOnly` creation
flag is used only for that temporary Builder so a rebuild starts from the upstream
parent instead of recursively using the previous derived official revision.

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
For official SSH-capable Bases this means the OpenSSH package is retained while
host keys are removed; each created Environment generates its own host identity
when SSH access is prepared.

Script/stop failure triggers bounded cleanup of the exact temporary Workspace
owner through canonical Env deletion. Publication uncertainty retains the builder
name and any native image/build alias evidence. A successfully published image
survives builder cleanup failure. Inspect `haco env list` and the reported native
image/alias before explicit cleanup; retry starts a fresh builder. No automatic
rollback, old-image deletion or crash replay is attempted.

## Existing Base and saved data

Official names include `haco/ubuntu-26.04` and `haco/ubuntu-24.04`. Operator mappings
in `HACO_INCUS_BASES_JSON` remain supported; ordinary `haco base build` cannot
replace these configured names, the reserved `haco/` namespace, or the internal
official-build namespace. Host setup may invoke only the fixed trusted official
recipes; those recipes still use the same Base Builder service and publication
machinery rather than bypassing namespace protection with direct Incus writes.
Native built images are scoped to the Hacocoon Incus project. Image alias movement
never rewrites saved `BaseRef` values. Official Builder revisions are protected
from ordinary built-image deletion while they back a reserved official name.

No catalog schema change or manual saved-data migration is required. Old Base
assets and snapshot Base components retain their existing ownership/explicit
cleanup. New snapshot rootfs remains independent and never gains a Base image
retention dependency. See [snapshots](environment-snapshots.md).

## Acceptance and remaining scope

Definition build, alias revision independence and installed Windows SSH passed
on `a2fcb72`. Official-Base acceptance additionally requires a fresh Environment
under default-deny Policy to contain `sshd` before SSH setup and to complete
ordinary `haco ssh setup` plus Windows native OpenSSH without package-mirror
permission. Earlier machine-ID/stdio failures and a local rebuild timeout remain
recorded in [acceptance evidence](../status/acceptance-evidence.md#storage).
Archive import, concurrent builds, history UI and automatic retries are deferred.

## Explicit built-image cleanup

Status: implemented; scoped native and Windows acceptance is recorded in [acceptance evidence](../status/acceptance-evidence.md#storage).

`haco base list --all [--json]` shows retained built-image revisions, their exact
fingerprints and build owners, current aliases, Environment/native users and
independent snapshot provenance. The ordinary Base list still shows selectable
starting points. `haco base delete <name>` selects the current built revision;
a fingerprint or an unambiguous hexadecimal prefix of at least eight characters
selects an older revision. Deletion previews the exact selection and asks for
confirmation (`--yes` for explicit automation). It does not delete every revision
sharing a name or select configured/upstream images as owned built images.

Incus owns image removal and its aliases. `internal/basemanage` adds current
catalog references and the reviewed identity; `modules/runtime/incus` verifies
native ownership and references and serializes removal against native create and
publication. The controller transports the exact fingerprint and build owner,
so alias movement cannot silently change the deletion target. Used images and
images with protected external aliases are refused. A failed or ambiguous delete
is reported without deleting any catalog records or launching rollback.

Workspace, OCI Stores, independent snapshot rootfs and existing Base assets are
not deleted. Snapshot BaseRef values remain provenance: image deletion must not
make snapshot restore depend on an image cache or original Base. No catalog schema
change, image retention object, automatic GC or migration is introduced. Source
repository/OCI-image cleanup and capacity reclamation use separate operations. See
[ADR 0043](../adr/0043-explicit-built-image-deletion.md).

## Resolution rules

Core uses `BaseName`, `BaseRevision` and `BaseRef{Name, Revision}`; provider-specific
aliases/remotes are adapter details. Explicit `--base` takes precedence over the
configured Hacocoon default. Project/user precedence layers are future intent,
not implemented selectors.

Incus resolves a configured alias once with image inspection, validates the full
fingerprint and initializes from that pinned revision. Reserved official logical
names first resolve to their verified Base Builder publication when available;
the trusted official Builder alone resolves their upstream configured source as a
parent. `haco/` remains reserved. `HACO_INCUS_BASES_JSON` rejects invalid names,
control characters, option-like sources, reserved overrides and oversized/malformed
configuration. A Base supplies guest contents; Policy/Capability supplies authority.
Project-specific setup remains separate from reusable Base tooling.
