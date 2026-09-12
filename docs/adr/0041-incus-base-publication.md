# ADR 0041: Publish Base images through Incus

Status: accepted.

Use an ordinary temporary Environment for definition execution and Incus image
publication for the resulting stopped rootfs. Base build is not a snapshot of a
user Workspace or an OCI Seed build. Current lifecycle locks and temporary
Workspace identity prevent accidental publication/deletion after name reuse.

Keep exact build identity in the image properties written atomically by Incus,
then verify the image before moving its logical alias. Unique build aliases make
uncertain publication discoverable without parsing human progress output. Keep
old revisions and ambiguous new images. Do not add a Hacocoon image catalog,
retained Base instance, generic importer or automatic replay/rollback framework.

Publishing changes native image metadata, so it belongs in the Incus adapter.
Application orchestration owns no Incus management credentials; definition code
runs inside the normal untrusted guest. The source lease remains held throughout
publication. A failed publication keeps the builder and native evidence for
explicit inspection. Existing data and permissions retain their normal lifetime.

An updated logical alias is a pointer for future creation, not mutation of an
existing Environment or revision. Ordinary create resolves and pins a full
fingerprint before Incus init. Never route explicitly built images through the
historical Seed substitution mechanism.
