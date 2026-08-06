package batch

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

var (
	_ AtomicCheckpointWriter[int] = (*constructorAtomicWriter[int])(nil)
	_                             = AtomicStepOptions[int, int]{
		"approved-shape", 1, nil, nil, nil,
		RetryPolicy{}, SkipPolicy{}, "",
	}
)

func TestAtomicSentinels(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{name: "checkpoint conflict", err: ErrCheckpointConflict, message: "batch: checkpoint revision conflict"},
		{name: "commit unknown", err: ErrCommitUnknown, message: "batch: commit outcome unknown"},
		{name: "atomicity unknown", err: ErrAtomicityUnknown, message: "batch: atomicity outcome unknown"},
		{name: "version exhausted", err: ErrCheckpointVersionExhausted, message: "batch: checkpoint version exhausted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil || tt.err.Error() != tt.message {
				t.Fatalf("sentinel = %v, want %q", tt.err, tt.message)
			}
			if !errors.Is(fmt.Errorf("wrapped: %w", tt.err), tt.err) {
				t.Fatalf("wrapped sentinel did not match %v", tt.err)
			}
		})
	}
}

func TestAtomicCheckpointContractCarriesValueAndVersion(t *testing.T) {
	checkpoint := VersionedCheckpoint{Value: "offset-42", Version: 7}
	if checkpoint.Value != "offset-42" || checkpoint.Version != 7 {
		t.Fatalf("unexpected checkpoint: %+v", checkpoint)
	}
}

func TestNewAtomicStepValidatesOptionsWithoutSideEffects(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AtomicStepOptions[int, int], *constructorEffects)
		wantErr string
	}{
		{
			name: "empty name",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.Name = ""
			},
			wantErr: "step name must not be empty",
		},
		{
			name: "negative chunk size",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.ChunkSize = -1
			},
			wantErr: "chunk size must be positive",
		},
		{
			name: "nil reader",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.Reader = nil
			},
			wantErr: "reader must not be nil",
		},
		{
			name: "nil processor",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.Processor = nil
			},
			wantErr: "processor must not be nil",
		},
		{
			name: "nil atomic writer",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.AtomicWriter = nil
			},
			wantErr: "atomic writer must not be nil",
		},
		{
			name: "reader without checkpoint support",
			mutate: func(options *AtomicStepOptions[int, int], effects *constructorEffects) {
				options.Reader = &constructorPlainReader{effects: effects}
			},
			wantErr: "reader does not support checkpoints",
		},
		{
			name: "negative retry policy",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.RetryPolicy = RetryPolicy{MaxAttempts: -1}
			},
			wantErr: "max attempts must be positive",
		},
		{
			name: "negative skip policy",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.SkipPolicy = SkipPolicy{MaxSkips: -1}
			},
			wantErr: "max skips must not be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effects := &constructorEffects{}
			options := validAtomicStepOptions(effects)
			tt.mutate(&options, effects)

			step, err := NewAtomicStep(options)

			if step != nil {
				t.Fatalf("step = %#v, want nil", step)
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if effects.calls != 0 {
				t.Fatalf("constructor performed %d reader/provider calls", effects.calls)
			}
		})
	}
}

func TestNewAtomicStepValidationOrder(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*AtomicStepOptions[int, int], *constructorEffects)
		wantErr string
	}{
		{
			name: "empty name precedes every other validation",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				*options = AtomicStepOptions[int, int]{
					ChunkSize:   -1,
					RetryPolicy: RetryPolicy{MaxAttempts: -1},
					SkipPolicy:  SkipPolicy{MaxSkips: -1},
				}
			},
			wantErr: "step name must not be empty",
		},
		{
			name: "negative chunk precedes nil dependencies",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.ChunkSize = -1
				options.Reader = nil
				options.Processor = nil
				options.AtomicWriter = nil
			},
			wantErr: "chunk size must be positive",
		},
		{
			name: "nil reader precedes nil processor and writer",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.Reader = nil
				options.Processor = nil
				options.AtomicWriter = nil
			},
			wantErr: "reader must not be nil",
		},
		{
			name: "nil processor precedes nil writer",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.Processor = nil
				options.AtomicWriter = nil
			},
			wantErr: "processor must not be nil",
		},
		{
			name: "nil writer precedes checkpoint support and policies",
			mutate: func(options *AtomicStepOptions[int, int], effects *constructorEffects) {
				options.Reader = &constructorPlainReader{effects: effects}
				options.AtomicWriter = nil
				options.RetryPolicy = RetryPolicy{MaxAttempts: -1}
				options.SkipPolicy = SkipPolicy{MaxSkips: -1}
			},
			wantErr: "atomic writer must not be nil",
		},
		{
			name: "checkpoint support precedes policies",
			mutate: func(options *AtomicStepOptions[int, int], effects *constructorEffects) {
				options.Reader = &constructorPlainReader{effects: effects}
				options.RetryPolicy = RetryPolicy{MaxAttempts: -1}
				options.SkipPolicy = SkipPolicy{MaxSkips: -1}
			},
			wantErr: "reader does not support checkpoints",
		},
		{
			name: "retry policy precedes skip policy",
			mutate: func(options *AtomicStepOptions[int, int], _ *constructorEffects) {
				options.RetryPolicy = RetryPolicy{MaxAttempts: -1}
				options.SkipPolicy = SkipPolicy{MaxSkips: -1}
			},
			wantErr: "max attempts must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effects := &constructorEffects{}
			options := validAtomicStepOptions(effects)
			tt.mutate(&options, effects)

			step, err := NewAtomicStep(options)

			if step != nil {
				t.Fatalf("step = %#v, want nil", step)
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if effects.calls != 0 {
				t.Fatalf("constructor performed %d reader/provider calls", effects.calls)
			}
		})
	}
}

func TestNewAtomicStepAppliesDefaultsWithoutSideEffects(t *testing.T) {
	effects := &constructorEffects{}
	options := validAtomicStepOptions(effects)
	options.ChunkSize = 0
	options.CheckpointKey = ""

	step, err := NewAtomicStep(options)

	if err != nil {
		t.Fatalf("NewAtomicStep failed: %v", err)
	}
	if step.chunkSize != DefaultChunkSize {
		t.Fatalf("chunk size = %d, want %d", step.chunkSize, DefaultChunkSize)
	}
	if step.key != options.Name {
		t.Fatalf("checkpoint key = %q, want %q", step.key, options.Name)
	}
	if step.reader != options.Reader || step.processor != options.Processor || step.atomic != options.AtomicWriter {
		t.Fatal("constructor did not retain the configured atomic components")
	}
	if step.writer != nil || step.store != nil {
		t.Fatal("atomic step unexpectedly configured legacy writer or checkpoint store")
	}
	if step.retry.MaxAttempts != 1 || step.skip.MaxSkips != 0 {
		t.Fatalf("unexpected normalized policies: retry=%+v skip=%+v", step.retry, step.skip)
	}
	if effects.calls != 0 {
		t.Fatalf("constructor performed %d reader/provider calls", effects.calls)
	}
}

func TestNewAtomicStepPreservesExplicitChunkSizeAndCheckpointKey(t *testing.T) {
	effects := &constructorEffects{}
	options := validAtomicStepOptions(effects)
	options.ChunkSize = 17
	options.CheckpointKey = "tenant-42"

	step, err := NewAtomicStep(options)

	if err != nil {
		t.Fatalf("NewAtomicStep failed: %v", err)
	}
	if step.chunkSize != 17 || step.key != "tenant-42" {
		t.Fatalf("unexpected explicit configuration: chunk=%d key=%q", step.chunkSize, step.key)
	}
	if effects.calls != 0 {
		t.Fatalf("constructor performed %d reader/provider calls", effects.calls)
	}
}

func validAtomicStepOptions(effects *constructorEffects) AtomicStepOptions[int, int] {
	return AtomicStepOptions[int, int]{
		Name:         "atomic-step",
		ChunkSize:    3,
		Reader:       &constructorCheckpointReader{effects: effects},
		Processor:    &constructorProcessor{effects: effects},
		AtomicWriter: &constructorAtomicWriter[int]{effects: effects},
	}
}

type constructorEffects struct {
	calls int
}

func (e *constructorEffects) record() {
	e.calls++
}

type constructorCheckpointReader struct {
	effects *constructorEffects
}

func (r *constructorCheckpointReader) Open(context.Context) error {
	r.effects.record()
	return nil
}

func (r *constructorCheckpointReader) Read(context.Context) (int, bool, error) {
	r.effects.record()
	return 0, false, nil
}

func (r *constructorCheckpointReader) Close(context.Context) error {
	r.effects.record()
	return nil
}

func (r *constructorCheckpointReader) Restore(context.Context, any) error {
	r.effects.record()
	return nil
}

func (r *constructorCheckpointReader) Checkpoint(context.Context) (any, bool, error) {
	r.effects.record()
	return nil, false, nil
}

type constructorPlainReader struct {
	effects *constructorEffects
}

func (r *constructorPlainReader) Open(context.Context) error {
	r.effects.record()
	return nil
}

func (r *constructorPlainReader) Read(context.Context) (int, bool, error) {
	r.effects.record()
	return 0, false, nil
}

func (r *constructorPlainReader) Close(context.Context) error {
	r.effects.record()
	return nil
}

type constructorProcessor struct {
	effects *constructorEffects
}

func (p *constructorProcessor) Process(context.Context, int) (int, bool, error) {
	p.effects.record()
	return 0, false, nil
}

type constructorAtomicWriter[T any] struct {
	effects *constructorEffects
}

func (w *constructorAtomicWriter[T]) Load(context.Context, string) (VersionedCheckpoint, bool, error) {
	w.effects.record()
	return VersionedCheckpoint{}, false, nil
}

func (w *constructorAtomicWriter[T]) Commit(context.Context, string, uint64, []T, any) (uint64, error) {
	w.effects.record()
	return 0, nil
}
