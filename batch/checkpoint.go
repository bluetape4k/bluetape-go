package batch

import (
	"context"
	"fmt"
	"sync"
)

// CheckpointReader restores and captures durable reader progress.
type CheckpointReader interface {
	Restore(context.Context, any) error
	Checkpoint(context.Context) (any, bool, error)
}

// CheckpointStore persists restart checkpoints by key.
type CheckpointStore interface {
	Load(context.Context, string) (any, bool, error)
	Save(context.Context, string, any) error
}

// MemoryCheckpointStore is an in-memory CheckpointStore for tests and local jobs.
type MemoryCheckpointStore struct {
	mu     sync.RWMutex
	values map[string]any
}

// NewMemoryCheckpointStore creates an empty memory checkpoint store.
func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{values: make(map[string]any)}
}

// Load returns the checkpoint for key when present.
func (s *MemoryCheckpointStore) Load(ctx context.Context, key string) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if s == nil {
		return nil, false, fmt.Errorf("checkpoint store must not be nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.values[key]
	return value, ok, nil
}

// Save stores checkpoint for key.
func (s *MemoryCheckpointStore) Save(ctx context.Context, key string, checkpoint any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("checkpoint store must not be nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = checkpoint
	return nil
}
