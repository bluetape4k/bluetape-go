package batch

import "errors"

var (
	// ErrCheckpointConflict indicates that the expected checkpoint revision is stale.
	ErrCheckpointConflict = errors.New("batch: checkpoint revision conflict")
	// ErrCommitUnknown indicates that an atomic commit may have succeeded.
	ErrCommitUnknown = errors.New("batch: commit outcome unknown")
	// ErrAtomicityUnknown indicates that output and checkpoint atomicity cannot be proven.
	ErrAtomicityUnknown = errors.New("batch: atomicity outcome unknown")
	// ErrCheckpointVersionExhausted indicates that the checkpoint version cannot advance.
	ErrCheckpointVersionExhausted = errors.New("batch: checkpoint version exhausted")

	// ErrUnsafeWriterSkipCheckpoint means a failed writer chunk matched SkipPolicy
	// while checkpointing was enabled. Writer does not report a committed item
	// boundary, so advancing the checkpoint could silently drop chunk items.
	ErrUnsafeWriterSkipCheckpoint = errors.New("batch: skipped writer chunk cannot advance checkpoint safely")
)
