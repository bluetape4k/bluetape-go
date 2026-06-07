package batch

import "errors"

var (
	// ErrUnsafeWriterSkipCheckpoint means a failed writer chunk matched SkipPolicy
	// while checkpointing was enabled. Writer does not report a committed item
	// boundary, so advancing the checkpoint could silently drop chunk items.
	ErrUnsafeWriterSkipCheckpoint = errors.New("batch: skipped writer chunk cannot advance checkpoint safely")
)
