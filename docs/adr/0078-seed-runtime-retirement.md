# Remove implicit Seed selection from ordinary Environments

Status: accepted; implementation candidate. [日本語](0078-seed-runtime-retirement.ja.md)

## Decision

A new Environment uses the explicitly selected Base and pinned revision. An OCI
plugin setting must not read a current-Seed catalog, substitute a different image,
or enable nested-container authority because of that substituted image. Remove
the resolver option and Seed-specific materialization argument rather than leave
an inactive branch that a future caller could accidentally enable.

Composition no longer constructs a Seed store/service or wraps ordinary commands
with the Seed-harvest adapter. Remove the legacy Seed command family, including
build, current, pin, recommendation, GC and recovery. Keep ordinary Base builds,
managed persistent OCI data and their existing scoped attachment checks.

Retirement is not migration or deletion. Do not rewrite saved Env Base references,
delete existing images/catalogs, recreate an installed Env, or infer permission to
clean old Seed data. Existing resume and data-lifetime paths remain authoritative.
The harvest adapter is removed; its shared managed-kind markers belong with the
Environment identity contract and cannot replace pinned ownership checks. Legacy builder, maintenance and Seed catalog code is also removed after
confirming that only the retired Seed path calls it. Its private-registry fixture
and manual job are retired with that path; historical results remain evidence for
that path only, not acceptance of the current persistent OCI Store. Existing
`seeds.json`, native images and Host OCI content remain untouched and must be
inventoried/captured through the reviewed evacuation workflow. OCI-plugin sampling,
recommendation and the shared deletion-state schema remain a separate retirement
slice so existing image deletion and re-enabling are preserved.

## Rejected alternatives

Keeping a disabled resolver option retains an unsafe alternate Base-selection
path. Treating an OCI plugin toggle as permission for image substitution mixes
tool integration with Environment identity and authority. Automatically pruning old
Seed images/catalogs would confuse retirement with data deletion and could destroy
the only recoverable copy of old work. Restoring `switch-base` is not part of this
transition; use the current retained-data recreation workflow.

See [Seed retirement](../design/oci-seed-and-cow.md) and
[data lifetime](../guides/data-lifetime.md).
