package batch

import (
	"context"
	"fmt"
)

// VersionedCheckpoint is a checkpoint value paired with its storage fencing version.
type VersionedCheckpoint struct {
	Value   any
	Version uint64
}

// AtomicCheckpointWriter atomically persists an output chunk and its next checkpoint.
// It has no Open or Close methods; the caller owns the provider and database lifecycle.
type AtomicCheckpointWriter[T any] interface {
	// Load returns the checkpoint for key and whether it exists. When no checkpoint
	// exists, the consumer contract starts with expected revision zero.
	Load(ctx context.Context, key string) (checkpoint VersionedCheckpoint, exists bool, err error)
	// Commit atomically persists items and checkpoint only when expectedVersion
	// matches, and returns the new revision. Implementations must preserve applicable
	// ErrCheckpointConflict, ErrCommitUnknown, ErrAtomicityUnknown, and
	// ErrCheckpointVersionExhausted identities via %w or equivalent errors.Is behavior.
	Commit(ctx context.Context, key string, expectedVersion uint64, items []T, checkpoint any) (newVersion uint64, err error)
}

// AtomicStepOptions configures a batch step whose output and checkpoint commit atomically.
type AtomicStepOptions[I any, O any] struct {
	Name          string
	ChunkSize     int
	Reader        Reader[I]
	Processor     Processor[I, O]
	AtomicWriter  AtomicCheckpointWriter[O]
	RetryPolicy   RetryPolicy
	SkipPolicy    SkipPolicy
	CheckpointKey string
}

// NewAtomicStep creates a batch step with atomic output and checkpoint commits.
func NewAtomicStep[I any, O any](options AtomicStepOptions[I, O]) (*Step[I, O], error) {
	if options.Name == "" {
		return nil, fmt.Errorf("step name must not be empty")
	}
	if options.ChunkSize == 0 {
		options.ChunkSize = DefaultChunkSize
	}
	if options.ChunkSize < 0 {
		return nil, fmt.Errorf("chunk size must be positive")
	}
	if options.Reader == nil {
		return nil, fmt.Errorf("reader must not be nil")
	}
	if options.Processor == nil {
		return nil, fmt.Errorf("processor must not be nil")
	}
	if options.AtomicWriter == nil {
		return nil, fmt.Errorf("atomic writer must not be nil")
	}
	if _, ok := options.Reader.(CheckpointReader); !ok {
		return nil, fmt.Errorf("reader does not support checkpoints")
	}
	retry, err := options.RetryPolicy.normalize()
	if err != nil {
		return nil, err
	}
	skip, err := options.SkipPolicy.normalize()
	if err != nil {
		return nil, err
	}
	if options.CheckpointKey == "" {
		options.CheckpointKey = options.Name
	}
	return &Step[I, O]{
		name:      options.Name,
		chunkSize: options.ChunkSize,
		reader:    options.Reader,
		processor: options.Processor,
		atomic:    options.AtomicWriter,
		retry:     retry,
		skip:      skip,
		key:       options.CheckpointKey,
	}, nil
}
