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
wait; compaction is never retried. Immediate-stop acceptance still failed at the
open deadline, so automatic readiness remains unresolved. Preserve that failure
and resume the exact distribution; do not stop unrelated WSL distributions.
