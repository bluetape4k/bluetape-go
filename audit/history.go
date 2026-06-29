package audit

import "sort"

// History is a validated audit history for one aggregate.
type History struct {
	aggregate AggregateID
	entries   []Entry
	head      Revision
}

// NewHistory creates a validated, revision-ordered audit history.
func NewHistory(entries []Entry) (History, error) {
	if len(entries) == 0 {
		return History{}, validationError(ErrInvalidEntry, "entries", len(entries))
	}

	ordered := make([]Entry, len(entries))
	for i, entry := range entries {
		ordered[i] = entry.Clone()
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Revision < ordered[j].Revision
	})

	aggregate := ordered[0].Aggregate
	if err := aggregate.Validate(); err != nil {
		return History{}, err
	}
	seenEventIDs := make(map[EventID]struct{}, len(ordered))
	seenIdempotencyKeys := make(map[string]EventID, len(ordered))

	for i, entry := range ordered {
		if err := entry.Validate(); err != nil {
			return History{}, err
		}
		if entry.Aggregate != aggregate {
			return History{}, validationError(ErrMixedAggregate, "aggregate", entry.Aggregate)
		}
		wantRevision := Revision(i + 1)
		if entry.Revision != wantRevision || entry.Revision != entry.Event.Revision {
			return History{}, validationError(ErrRevisionConflict, "revision", entry.Revision)
		}
		if _, ok := seenEventIDs[entry.Event.EventID]; ok {
			return History{}, validationError(ErrRevisionConflict, "event_id", entry.Event.EventID)
		}
		seenEventIDs[entry.Event.EventID] = struct{}{}
		if existingEventID, ok := seenIdempotencyKeys[entry.Event.IdempotencyKey]; ok && existingEventID != entry.Event.EventID {
			return History{}, validationError(ErrRevisionConflict, "idempotency_key", entry.Event.IdempotencyKey)
		}
		seenIdempotencyKeys[entry.Event.IdempotencyKey] = entry.Event.EventID
	}

	return History{
		aggregate: aggregate,
		entries:   ordered,
		head:      ordered[len(ordered)-1].Revision,
	}, nil
}

// AggregateID returns the aggregate identity shared by all history entries.
func (h History) AggregateID() AggregateID {
	return h.aggregate
}

// HeadRevision returns the latest revision in the history.
func (h History) HeadRevision() Revision {
	return h.head
}

// Entries returns a defensive copy of ordered audit entries.
func (h History) Entries() []Entry {
	if len(h.entries) == 0 {
		return nil
	}
	entries := make([]Entry, len(h.entries))
	for i, entry := range h.entries {
		entries[i] = entry.Clone()
	}
	return entries
}
