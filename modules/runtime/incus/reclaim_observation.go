package incus

// ReclaimAllocation measures a backing file, not filesystem free space.
type ReclaimAllocation struct{ LogicalBytes, AllocatedBytes uint64 }

// PoolTrimObservation keeps kernel discard counts distinct from file allocation.
type PoolTrimObservation struct {
	FilesystemBefore, FilesystemAfter *ReclaimFilesystemUsage
	Before, After                     ReclaimAllocation
	Attempted, KernelReportKnown      bool
	KernelTrimmedBytes                uint64
}

// OuterTrimObservation reports ext4 discard, never Windows recovered bytes.
type OuterTrimObservation struct {
	FilesystemBefore, FilesystemAfter *ReclaimFilesystemUsage
	Attempted, KernelReportKnown      bool
	KernelTrimmedBytes                uint64
}

// ReclaimFilesystemUsage is statfs capacity/use, not backing-file allocation.
type ReclaimFilesystemUsage struct{ CapacityBytes, UsedBytes uint64 }
