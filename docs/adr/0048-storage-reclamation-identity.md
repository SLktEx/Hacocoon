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
pool permission. Windows measurement/compaction and the public all-layer entry
remain pending; Linux kernel trim counts are not Windows recovered allocation.
