# ADR 0103: Release attached virtual-disk observation handles before waiting

[日本語](0103-virtual-disk-observation-lifetime.ja.md) | English

Status: accepted for implementation; actual installed reclamation is separate.

## Context

The Windows worker can open a disk before WSL releases its attachment. Waiting
for that same virtual handle to report detached can prevent the state change we
are waiting for. Microsoft's [DetachVirtualDisk contract](https://learn.microsoft.com/en-us/windows/win32/api/virtdisk/nf-virtdisk-detachvirtualdisk)
requires other virtual-disk handles to close before detach, and non-permanent
attachments remain until their last open handle closes.

## Decision

Keep the managed registration, saved operation, exclusion, native file and parent
pins throughout. Those pins fix the file and its volume-relative path. An attached
virtual-disk observation handle has a different lifetime: close it before waiting
and reopen the same pinned path within the existing 90-second budget. Retry only
native open sharing violations or a confirmed attached observation after successful
close. An unknown observation or failed close stops immediately. Cancellation
closes any failed observation before returning.

A successfully detached handle transfers to the caller for one compaction. No
open/inspection retry occurs after mutation starts. Never release file/parent
pins, force-detach the disk, stop another distribution, change WSL idle settings,
or reinterpret timeout as completion. Same-target resume and durable failure
records retain their existing ownership.

The refusal is still necessary: a live WSL filesystem must not be compacted as
an offline disk. The observer reads the documented
[GET_VIRTUAL_DISK_INFO.IsLoaded](https://learn.microsoft.com/en-us/windows/win32/api/virtdisk/ns-virtdisk-get_virtual_disk_info)
field. Component tests cover close-before-reopen, bounded cancellation and failed
close. An independently created empty VHD checks the native handle lifetime when
the Windows account has attachment privilege. That fixture is separate from
installed WSL acceptance and cannot erase previous `compact_attached` failures.

## Superseded assumption

The earlier same-virtual-handle polling assumption is superseded. The ownership
pins and no-mutation-replay rules in [ADR 0048](0048-storage-reclamation-identity.md)
remain. Increasing the timeout alone would not address an observer that prevents
detach. A WSL restart by another client is a separate concurrent-use condition.
