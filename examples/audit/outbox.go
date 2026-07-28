package auditexample

import (
	"context"
	"sync"

	"github.com/bluetape4k/bluetape-go/audit"
)

// MemoryOutbox는 example package에서 사용하는 in-memory EntrySink다.
type MemoryOutbox struct {
	mu      sync.Mutex
	entries []audit.Entry
}

// NewMemoryOutbox는 비어 있는 in-memory outbox fixture를 생성한다.
func NewMemoryOutbox() *MemoryOutbox {
	return &MemoryOutbox{}
}

// Enqueue는 entries의 defensive copy를 저장한다.
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

// Entries는 queue에 쌓인 entry의 defensive copy를 반환한다.
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
