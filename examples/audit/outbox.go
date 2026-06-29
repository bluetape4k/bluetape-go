package auditexample

import (
	"context"
	"sync"

	"github.com/bluetape4k/bluetape-go/audit"
)

// MemoryOutbox is an in-memory EntrySink for the example package.
type MemoryOutbox struct {
	mu      sync.Mutex
	entries []audit.Entry
}

// NewMemoryOutbox creates an empty in-memory outbox fixture.
func NewMemoryOutbox() *MemoryOutbox {
	return &MemoryOutbox{}
}

// Enqueue stores defensive copies of entries.
func (o *MemoryOutbox) Enqueue(ctx context.Context, entries ...audit.Entry) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	clones := make([]audit.Entry, len(entries))
	for i, entry := range entries {
		clone := entry.Clone()
		if err := clone.Validate(); err != nil {
			return err
		}
		clones[i] = clone
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.entries = append(o.entries, clones...)
	return nil
}

// Entries returns defensive copies of queued entries.
func (o *MemoryOutbox) Entries() []audit.Entry {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.entries) == 0 {
		return nil
	}
	entries := make([]audit.Entry, len(o.entries))
	for i, entry := range o.entries {
		entries[i] = entry.Clone()
	}
	return entries
}
