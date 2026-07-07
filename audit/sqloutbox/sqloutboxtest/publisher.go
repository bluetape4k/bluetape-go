package sqloutboxtest

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/sqloutbox"
)

var (
	// ErrNilPublisherFunc reports a nil PublisherFunc.
	ErrNilPublisherFunc = errors.New("nil sqloutbox publisher function")
	// ErrNilRecordingPublisher reports a nil *RecordingPublisher receiver.
	ErrNilRecordingPublisher = errors.New("nil sqloutbox recording publisher")
	// ErrInjectedFailure is the default error returned by WithFailures when no
	// explicit error is supplied.
	ErrInjectedFailure = errors.New("injected sqloutbox publisher failure")
)

// PublisherFunc adapts a function into a sqloutbox.Publisher.
type PublisherFunc func(context.Context, sqloutbox.Record) error

// Publish publishes one claimed sqloutbox record.
func (fn PublisherFunc) Publish(ctx context.Context, record sqloutbox.Record) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if fn == nil {
		return ErrNilPublisherFunc
	}
	return fn(ctx, record)
}

// DiscardPublisher accepts records without retaining or transporting them.
type DiscardPublisher struct{}

// Publish accepts one claimed sqloutbox record.
func (DiscardPublisher) Publish(ctx context.Context, _ sqloutbox.Record) error {
	return contextError(ctx)
}

// RecordingOption configures a RecordingPublisher.
type RecordingOption func(*RecordingPublisher)

// WithFailures configures deterministic per-event publish failures.
func WithFailures(failures map[audit.EventID]int, err error) RecordingOption {
	return func(p *RecordingPublisher) {
		p.mu.Lock()
		defer p.mu.Unlock()

		if err == nil {
			err = ErrInjectedFailure
		}
		p.failureErr = err
		p.failures = make(map[audit.EventID]int, len(failures))
		for eventID, count := range failures {
			if count > 0 {
				p.failures[eventID] = count
			}
		}
	}
}

// RecordingPublisher records every publish attempt and can inject deterministic
// failures. Its zero value is ready to use and is safe for concurrent Publish
// calls.
type RecordingPublisher struct {
	mu         sync.Mutex
	records    []sqloutbox.Record
	failures   map[audit.EventID]int
	failureErr error
}

// NewRecordingPublisher creates a recording publisher.
func NewRecordingPublisher(options ...RecordingOption) *RecordingPublisher {
	publisher := &RecordingPublisher{}
	for _, option := range options {
		if option != nil {
			option(publisher)
		}
	}
	return publisher
}

// Publish records one claimed sqloutbox record.
func (p *RecordingPublisher) Publish(ctx context.Context, record sqloutbox.Record) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if p == nil {
		return ErrNilRecordingPublisher
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.records = append(p.records, record)
	if p.failures != nil {
		if remaining := p.failures[record.EventID]; remaining > 0 {
			p.failures[record.EventID] = remaining - 1
			if p.failureErr == nil {
				return ErrInjectedFailure
			}
			return p.failureErr
		}
	}
	return nil
}

// Records returns a snapshot of all publish attempts.
func (p *RecordingPublisher) Records() []sqloutbox.Record {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	records := make([]sqloutbox.Record, len(p.records))
	copy(records, p.records)
	return records
}

// EventIDs returns a snapshot of published event IDs in attempt order.
func (p *RecordingPublisher) EventIDs() []audit.EventID {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ids := make([]audit.EventID, len(p.records))
	for index, record := range p.records {
		ids[index] = record.EventID
	}
	return ids
}

// Count returns the number of publish attempts recorded.
func (p *RecordingPublisher) Count() int {
	if p == nil {
		return 0
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.records)
}

// Reset clears recorded attempts and failure counters.
func (p *RecordingPublisher) Reset() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.records = nil
	p.failures = nil
	p.failureErr = nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("nil context")
	}
	return ctx.Err()
}
