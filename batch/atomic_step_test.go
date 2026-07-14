package batch

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type atomicTestReader[T any] struct {
	items             []T
	index             int
	openErr           error
	readErr           error
	readHook          func()
	restoreErr        error
	checkpointHook    func()
	checkpointErr     error
	checkpointMissing bool
	closeErr          error
	openCalls         int
	readCalls         int
	restoreCalls      int
	checkpointCalls   int
	closeCalls        int
	restored          []any
	closeContextErr   error
}

func (r *atomicTestReader[T]) Open(context.Context) error {
	r.openCalls++
	return r.openErr
}

func (r *atomicTestReader[T]) Read(context.Context) (T, bool, error) {
	r.readCalls++
	if r.readHook != nil {
		r.readHook()
	}
	if r.readErr != nil {
		var zero T
		return zero, false, r.readErr
	}
	if r.index >= len(r.items) {
		var zero T
		return zero, false, nil
	}
	item := r.items[r.index]
	r.index++
	return item, true, nil
}

func (r *atomicTestReader[T]) Restore(_ context.Context, checkpoint any) error {
	r.restoreCalls++
	r.restored = append(r.restored, checkpoint)
	if index, ok := checkpoint.(int); ok {
		r.index = index
	}
	return r.restoreErr
}

func (r *atomicTestReader[T]) Checkpoint(context.Context) (any, bool, error) {
	r.checkpointCalls++
	if r.checkpointHook != nil {
		r.checkpointHook()
	}
	if r.checkpointErr != nil {
		return nil, false, r.checkpointErr
	}
	if r.checkpointMissing {
		return nil, false, nil
	}
	return r.index, true, nil
}

func (r *atomicTestReader[T]) Close(ctx context.Context) error {
	r.closeCalls++
	r.closeContextErr = ctx.Err()
	return r.closeErr
}

type atomicCommitRecord[T any] struct {
	key        string
	expected   uint64
	items      []T
	checkpoint any
}

type atomicWriterRecorder[T any] struct {
	loadCheckpoint VersionedCheckpoint
	loadExists     bool
	loadErr        error
	commitVersions []uint64
	commitErrors   []error
	loadCalls      int
	loadKeys       []string
	commits        []atomicCommitRecord[T]
	retainItems    bool
	retainedItems  [][]T
	commitHook     func([]T)
	openCalls      int
	closeCalls     int
}

func (w *atomicWriterRecorder[T]) Load(_ context.Context, key string) (VersionedCheckpoint, bool, error) {
	w.loadCalls++
	w.loadKeys = append(w.loadKeys, key)
	return w.loadCheckpoint, w.loadExists, w.loadErr
}

func (w *atomicWriterRecorder[T]) Commit(_ context.Context, key string, expected uint64, items []T, checkpoint any) (uint64, error) {
	itemsCopy := append([]T(nil), items...)
	w.commits = append(w.commits, atomicCommitRecord[T]{
		key:        key,
		expected:   expected,
		items:      itemsCopy,
		checkpoint: checkpoint,
	})
	if w.retainItems {
		w.retainedItems = append(w.retainedItems, items)
	}
	if w.commitHook != nil {
		w.commitHook(items)
	}
	call := len(w.commits) - 1
	if call < len(w.commitErrors) && w.commitErrors[call] != nil {
		return 0, w.commitErrors[call]
	}
	if call < len(w.commitVersions) {
		return w.commitVersions[call], nil
	}
	return expected + 1, nil
}

// Open and Close are deliberately not part of AtomicCheckpointWriter. They
// detect accidental provider lifecycle calls by Step.
func (w *atomicWriterRecorder[T]) Open(context.Context) error {
	w.openCalls++
	return nil
}

func (w *atomicWriterRecorder[T]) Close(context.Context) error {
	w.closeCalls++
	return nil
}

type atomicLegacyWriter[T any] struct {
	openCalls  int
	writeCalls int
	closeCalls int
}

func (w *atomicLegacyWriter[T]) Open(context.Context) error {
	w.openCalls++
	return nil
}

func (w *atomicLegacyWriter[T]) Write(context.Context, []T) error {
	w.writeCalls++
	return nil
}

func (w *atomicLegacyWriter[T]) Close(context.Context) error {
	w.closeCalls++
	return nil
}

func newAtomicIntStep(
	t *testing.T,
	reader *atomicTestReader[int],
	writer *atomicWriterRecorder[int],
	chunkSize int,
	processor Processor[int, int],
	retry RetryPolicy,
	skip SkipPolicy,
) *Step[int, int] {
	t.Helper()
	step, err := NewAtomicStep(AtomicStepOptions[int, int]{
		Name:          "atomic-test",
		ChunkSize:     chunkSize,
		Reader:        reader,
		Processor:     processor,
		AtomicWriter:  writer,
		RetryPolicy:   retry,
		SkipPolicy:    skip,
		CheckpointKey: "checkpoint-key",
	})
	if err != nil {
		t.Fatalf("NewAtomicStep() error = %v", err)
	}
	return step
}

func TestAtomicRestoreMissingStartsAtVersionZero(t *testing.T) {
	reader := &atomicTestReader[int]{items: []int{7}}
	writer := &atomicWriterRecorder[int]{}
	step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

	report := step.Run(context.Background())

	if report.Status != StatusCompleted || report.Err != nil {
		t.Fatalf("Run() report = %+v", report)
	}
	if writer.loadCalls != 1 || reader.restoreCalls != 0 {
		t.Fatalf("load calls = %d, restore calls = %d", writer.loadCalls, reader.restoreCalls)
	}
	if len(writer.commits) != 1 || writer.commits[0].expected != 0 {
		t.Fatalf("commits = %#v", writer.commits)
	}
}

func TestAtomicRestoreExistingValueAndVersion(t *testing.T) {
	reader := &atomicTestReader[int]{items: []int{10, 20, 30}}
	writer := &atomicWriterRecorder[int]{
		loadExists:     true,
		loadCheckpoint: VersionedCheckpoint{Value: 1, Version: 7},
	}
	step := newAtomicIntStep(t, reader, writer, 2, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

	report := step.Run(context.Background())

	if report.Status != StatusCompleted || report.ReadCount != 2 || report.WriteCount != 2 {
		t.Fatalf("Run() report = %+v", report)
	}
	if !reflect.DeepEqual(reader.restored, []any{1}) {
		t.Fatalf("restored = %#v", reader.restored)
	}
	if len(writer.commits) != 1 || writer.commits[0].expected != 7 || !reflect.DeepEqual(writer.commits[0].items, []int{20, 30}) {
		t.Fatalf("commits = %#v", writer.commits)
	}
}

func TestAtomicRestoreLoadAndRestoreFailures(t *testing.T) {
	loadFailure := errors.New("load failed")
	restoreFailure := errors.New("restore failed")
	tests := []struct {
		name      string
		reader    *atomicTestReader[int]
		writer    *atomicWriterRecorder[int]
		wantError error
	}{
		{
			name:      "load",
			reader:    &atomicTestReader[int]{items: []int{1}},
			writer:    &atomicWriterRecorder[int]{loadErr: loadFailure},
			wantError: loadFailure,
		},
		{
			name:      "restore",
			reader:    &atomicTestReader[int]{items: []int{1}, restoreErr: restoreFailure},
			writer:    &atomicWriterRecorder[int]{loadExists: true, loadCheckpoint: VersionedCheckpoint{Value: 0, Version: 1}},
			wantError: restoreFailure,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := newAtomicIntStep(t, tt.reader, tt.writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

			report := step.Run(context.Background())

			if report.Status != StatusFailed || !errors.Is(report.Err, tt.wantError) {
				t.Fatalf("Run() report = %+v", report)
			}
			if tt.reader.closeCalls != 1 || len(tt.writer.commits) != 0 {
				t.Fatalf("close calls = %d, commits = %d", tt.reader.closeCalls, len(tt.writer.commits))
			}
		})
	}
}

func TestAtomicBoundaryUsesConsumedInputForCommits(t *testing.T) {
	processorFailure := errors.New("processor failed")
	tests := []struct {
		name            string
		items           []int
		chunkSize       int
		processor       Processor[int, int]
		skip            SkipPolicy
		wantItems       [][]int
		wantCheckpoints []any
		wantRead        int
		wantWrite       int
		wantFilter      int
		wantSkip        int
	}{
		{
			name:      "kept then filtered",
			items:     []int{1, 2},
			chunkSize: 2,
			processor: ProcessorFunc[int, int](func(_ context.Context, item int) (int, bool, error) {
				return item * 10, item != 2, nil
			}),
			wantItems:       [][]int{{10}},
			wantCheckpoints: []any{2},
			wantRead:        2,
			wantWrite:       1,
			wantFilter:      1,
		},
		{
			name:      "kept then processor skipped",
			items:     []int{1, 2},
			chunkSize: 2,
			processor: ProcessorFunc[int, int](func(_ context.Context, item int) (int, bool, error) {
				if item == 2 {
					return 0, false, processorFailure
				}
				return item * 10, true, nil
			}),
			skip:            SkipPolicy{MaxSkips: 10, SkipIf: func(err error) bool { return errors.Is(err, processorFailure) }},
			wantItems:       [][]int{{10}},
			wantCheckpoints: []any{2},
			wantRead:        2,
			wantWrite:       1,
			wantSkip:        1,
		},
		{
			name:      "all filtered",
			items:     []int{1, 2, 3},
			chunkSize: 2,
			processor: ProcessorFunc[int, int](func(_ context.Context, item int) (int, bool, error) {
				return item, false, nil
			}),
			wantItems:       [][]int{nil, nil},
			wantCheckpoints: []any{2, 3},
			wantRead:        3,
			wantFilter:      3,
		},
		{
			name:      "all processor skipped",
			items:     []int{1, 2, 3},
			chunkSize: 2,
			processor: ProcessorFunc[int, int](func(context.Context, int) (int, bool, error) {
				return 0, false, processorFailure
			}),
			skip:            SkipPolicy{MaxSkips: 10, SkipIf: func(err error) bool { return errors.Is(err, processorFailure) }},
			wantItems:       [][]int{nil, nil},
			wantCheckpoints: []any{2, 3},
			wantRead:        3,
			wantSkip:        3,
		},
		{
			name:      "mixed",
			items:     []int{1, 2, 3, 4},
			chunkSize: 3,
			processor: ProcessorFunc[int, int](func(_ context.Context, item int) (int, bool, error) {
				switch item {
				case 2:
					return 0, false, nil
				case 3:
					return 0, false, processorFailure
				default:
					return item * 10, true, nil
				}
			}),
			skip:            SkipPolicy{MaxSkips: 10, SkipIf: func(err error) bool { return errors.Is(err, processorFailure) }},
			wantItems:       [][]int{{10}, {40}},
			wantCheckpoints: []any{3, 4},
			wantRead:        4,
			wantWrite:       2,
			wantFilter:      1,
			wantSkip:        1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &atomicTestReader[int]{items: tt.items}
			writer := &atomicWriterRecorder[int]{}
			step := newAtomicIntStep(t, reader, writer, tt.chunkSize, tt.processor, RetryPolicy{}, tt.skip)

			report := step.Run(context.Background())

			if report.Status != StatusCompleted || report.Err != nil {
				t.Fatalf("Run() report = %+v", report)
			}
			if report.ReadCount != tt.wantRead || report.WriteCount != tt.wantWrite || report.FilterCount != tt.wantFilter || report.SkipCount != tt.wantSkip {
				t.Fatalf("Run() counts = read:%d write:%d filter:%d skip:%d", report.ReadCount, report.WriteCount, report.FilterCount, report.SkipCount)
			}
			gotItems := make([][]int, len(writer.commits))
			gotCheckpoints := make([]any, len(writer.commits))
			for i, commit := range writer.commits {
				gotItems[i] = commit.items
				gotCheckpoints[i] = commit.checkpoint
			}
			if !reflect.DeepEqual(gotItems, tt.wantItems) || !reflect.DeepEqual(gotCheckpoints, tt.wantCheckpoints) {
				t.Fatalf("commit items/checkpoints = %#v / %#v, want %#v / %#v", gotItems, gotCheckpoints, tt.wantItems, tt.wantCheckpoints)
			}
		})
	}
}

func TestAtomicBoundaryEmptyAndExactMultipleEOFDoNotExtraCommit(t *testing.T) {
	tests := []struct {
		name        string
		items       []int
		wantCommits int
	}{
		{name: "empty", items: nil, wantCommits: 0},
		{name: "exact multiple", items: []int{1, 2, 3, 4}, wantCommits: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := &atomicTestReader[int]{items: tt.items}
			writer := &atomicWriterRecorder[int]{}
			step := newAtomicIntStep(t, reader, writer, 2, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

			report := step.Run(context.Background())

			if report.Status != StatusCompleted || len(writer.commits) != tt.wantCommits {
				t.Fatalf("Run() report = %+v, commits = %#v", report, writer.commits)
			}
		})
	}
}

func TestAtomicBoundaryKeptEmptyItemIsCommitted(t *testing.T) {
	reader := &atomicTestReader[string]{items: []string{"input"}}
	writer := &atomicWriterRecorder[string]{}
	step, err := NewAtomicStep(AtomicStepOptions[string, string]{
		Name:         "empty-item",
		ChunkSize:    1,
		Reader:       reader,
		Processor:    ProcessorFunc[string, string](func(context.Context, string) (string, bool, error) { return "", true, nil }),
		AtomicWriter: writer,
	})
	if err != nil {
		t.Fatalf("NewAtomicStep() error = %v", err)
	}

	report := step.Run(context.Background())

	if report.Status != StatusCompleted || report.WriteCount != 1 || len(writer.commits) != 1 || !reflect.DeepEqual(writer.commits[0].items, []string{""}) {
		t.Fatalf("Run() report = %+v, commits = %#v", report, writer.commits)
	}
}

func TestAtomicBoundaryAdvancesExpectedVersionFromCommitResult(t *testing.T) {
	reader := &atomicTestReader[int]{items: []int{1, 2}}
	writer := &atomicWriterRecorder[int]{commitVersions: []uint64{5, 9}}
	step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

	report := step.Run(context.Background())

	if report.Status != StatusCompleted || report.WriteCount != 2 || len(writer.commits) != 2 {
		t.Fatalf("Run() report = %+v, commits = %#v", report, writer.commits)
	}
	if writer.commits[0].expected != 0 || writer.commits[1].expected != 5 {
		t.Fatalf("expected versions = %d, %d", writer.commits[0].expected, writer.commits[1].expected)
	}
}

func TestAtomicBoundaryProcessorRetryUsesExistingPolicy(t *testing.T) {
	processorFailure := errors.New("processor failed")
	attempts := 0
	reader := &atomicTestReader[int]{items: []int{1}}
	writer := &atomicWriterRecorder[int]{}
	processor := ProcessorFunc[int, int](func(_ context.Context, item int) (int, bool, error) {
		attempts++
		if attempts == 1 {
			return 0, false, processorFailure
		}
		return item * 10, true, nil
	})
	step := newAtomicIntStep(t, reader, writer, 1, processor, RetryPolicy{
		MaxAttempts: 2,
		RetryIf: func(err error) bool {
			return errors.Is(err, processorFailure)
		},
	}, SkipPolicy{})

	report := step.Run(context.Background())

	if report.Status != StatusCompleted || report.RetryCount != 1 || report.WriteCount != 1 {
		t.Fatalf("Run() report = %+v", report)
	}
	if attempts != 2 || len(writer.commits) != 1 || !reflect.DeepEqual(writer.commits[0].items, []int{10}) {
		t.Fatalf("attempts = %d, commits = %#v", attempts, writer.commits)
	}
}

func TestAtomicBoundaryCheckpointUnavailableOrFailed(t *testing.T) {
	checkpointFailure := errors.New("checkpoint failed")
	tests := []struct {
		name      string
		reader    *atomicTestReader[int]
		wantError error
	}{
		{name: "unavailable", reader: &atomicTestReader[int]{items: []int{1}, checkpointMissing: true}},
		{name: "failed", reader: &atomicTestReader[int]{items: []int{1}, checkpointErr: checkpointFailure}, wantError: checkpointFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &atomicWriterRecorder[int]{}
			step := newAtomicIntStep(t, tt.reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

			report := step.Run(context.Background())

			if report.Status != StatusFailed || report.WriteCount != 0 || len(writer.commits) != 0 {
				t.Fatalf("Run() report = %+v, commits = %#v", report, writer.commits)
			}
			if tt.wantError != nil && !errors.Is(report.Err, tt.wantError) {
				t.Fatalf("Run() error = %v, want %v", report.Err, tt.wantError)
			}
			if tt.wantError == nil && (report.Err == nil || report.Err.Error() != "reader checkpoint is unavailable") {
				t.Fatalf("Run() error = %v", report.Err)
			}
		})
	}
}

func TestAtomicStatusCommitErrorsAreTerminalAndPreservePendingItems(t *testing.T) {
	tests := []struct {
		name      string
		commitErr error
		wantError error
	}{
		{name: "conflict", commitErr: fmt.Errorf("commit: %w", ErrCheckpointConflict), wantError: ErrCheckpointConflict},
		{name: "version exhausted", commitErr: fmt.Errorf("commit: %w", ErrCheckpointVersionExhausted), wantError: ErrCheckpointVersionExhausted},
		{name: "commit unknown", commitErr: fmt.Errorf("commit: %w", ErrCommitUnknown), wantError: ErrCommitUnknown},
		{name: "atomicity unknown", commitErr: fmt.Errorf("commit: %w", ErrAtomicityUnknown), wantError: ErrAtomicityUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryPredicateCalls := 0
			skipPredicateCalls := 0
			reader := &atomicTestReader[int]{items: []int{1, 2}}
			writer := &atomicWriterRecorder[int]{commitErrors: []error{tt.commitErr}}
			step := newAtomicIntStep(t, reader, writer, 2, IdentityProcessor[int](), RetryPolicy{
				MaxAttempts: 3,
				RetryIf: func(error) bool {
					retryPredicateCalls++
					return true
				},
			}, SkipPolicy{
				MaxSkips: 3,
				SkipIf: func(error) bool {
					skipPredicateCalls++
					return true
				},
			})

			report := step.Run(context.Background())

			if report.Status != StatusFailed || !errors.Is(report.Err, tt.wantError) {
				t.Fatalf("Run() report = %+v", report)
			}
			if report.WriteCount != 0 || report.RetryCount != 0 || report.SkipCount != 0 || retryPredicateCalls != 0 || skipPredicateCalls != 0 {
				t.Fatalf("commit failure counts = write:%d retry:%d skip:%d predicates:%d/%d", report.WriteCount, report.RetryCount, report.SkipCount, retryPredicateCalls, skipPredicateCalls)
			}
			if len(writer.commits) != 1 || !reflect.DeepEqual(writer.commits[0].items, []int{1, 2}) {
				t.Fatalf("commit snapshot = %#v", writer.commits)
			}
		})
	}
}

func TestAtomicStatusUnknownWinsOverContextCancellation(t *testing.T) {
	for _, unknown := range []error{ErrCommitUnknown, ErrAtomicityUnknown} {
		t.Run(unknown.Error(), func(t *testing.T) {
			commitErr := errors.Join(context.Canceled, unknown)
			reader := &atomicTestReader[int]{items: []int{1}}
			writer := &atomicWriterRecorder[int]{commitErrors: []error{commitErr}}
			step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

			report := step.Run(context.Background())

			if report.Status != StatusFailed || !errors.Is(report.Err, unknown) || !errors.Is(report.Err, context.Canceled) {
				t.Fatalf("Run() report = %+v", report)
			}
		})
	}
}

func TestAtomicSliceRetentionAcrossSuccessfulCommits(t *testing.T) {
	reader := &atomicTestReader[int]{items: []int{1, 2}}
	writer := &atomicWriterRecorder[int]{retainItems: true}
	step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

	report := step.Run(context.Background())

	if report.Status != StatusCompleted || report.WriteCount != 2 {
		t.Fatalf("Run() report = %+v", report)
	}
	if len(writer.retainedItems) != 2 {
		t.Fatalf("retained item slices = %#v", writer.retainedItems)
	}
	if !reflect.DeepEqual(writer.retainedItems[0], []int{1}) || !reflect.DeepEqual(writer.retainedItems[1], []int{2}) {
		t.Fatalf("retained item slices = %#v", writer.retainedItems)
	}
}

func TestAtomicSliceRetentionProviderCannotMutatePendingOnFailure(t *testing.T) {
	commitFailure := errors.New("commit failed")
	t.Run("mutation during commit", func(t *testing.T) {
		reader := &atomicTestReader[int]{}
		writer := &atomicWriterRecorder[int]{
			commitErrors: []error{commitFailure},
			commitHook: func(items []int) {
				items[0] = 99
			},
		}
		step := &Step[int, int]{atomic: writer, key: "checkpoint-key"}
		report := newReport("atomic-test")
		pending := []int{1, 2}

		_, err := step.commitAtomic(context.Background(), &report, reader, 0, pending)

		if !errors.Is(err, commitFailure) || report.WriteCount != 0 {
			t.Fatalf("commitAtomic() error = %v, report = %+v", err, report)
		}
		if !reflect.DeepEqual(pending, []int{1, 2}) {
			t.Fatalf("Step pending items mutated during Commit: %#v", pending)
		}
	})

	t.Run("mutation after commit returns", func(t *testing.T) {
		reader := &atomicTestReader[int]{}
		writer := &atomicWriterRecorder[int]{commitErrors: []error{commitFailure}, retainItems: true}
		step := &Step[int, int]{atomic: writer, key: "checkpoint-key"}
		report := newReport("atomic-test")
		pending := []int{1, 2}

		_, err := step.commitAtomic(context.Background(), &report, reader, 0, pending)
		if !errors.Is(err, commitFailure) || len(writer.retainedItems) != 1 {
			t.Fatalf("commitAtomic() error = %v, retained = %#v", err, writer.retainedItems)
		}
		writer.retainedItems[0][1] = 88
		if !reflect.DeepEqual(pending, []int{1, 2}) {
			t.Fatalf("Step pending items mutated after Commit returned: %#v", pending)
		}
	})
}

func TestAtomicCancellationDispatchBeforeCheckpointCapture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &atomicTestReader[int]{items: []int{1}}
	writer := &atomicWriterRecorder[int]{}
	processor := ProcessorFunc[int, int](func(_ context.Context, item int) (int, bool, error) {
		cancel()
		return item, true, nil
	})
	step := newAtomicIntStep(t, reader, writer, 1, processor, RetryPolicy{}, SkipPolicy{})

	report := step.Run(ctx)

	if report.Status != StatusCancelled || !errors.Is(report.Err, context.Canceled) || report.WriteCount != 0 {
		t.Fatalf("Run() report = %+v", report)
	}
	if reader.checkpointCalls != 0 || len(writer.commits) != 0 {
		t.Fatalf("dispatch calls = checkpoint:%d commit:%d", reader.checkpointCalls, len(writer.commits))
	}
}

func TestAtomicCancellationDispatchAfterCheckpointCapture(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &atomicTestReader[int]{items: []int{1}, checkpointHook: cancel}
	writer := &atomicWriterRecorder[int]{}
	step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

	report := step.Run(ctx)

	if report.Status != StatusCancelled || !errors.Is(report.Err, context.Canceled) || report.WriteCount != 0 {
		t.Fatalf("Run() report = %+v", report)
	}
	if reader.checkpointCalls != 1 || len(writer.commits) != 0 {
		t.Fatalf("dispatch calls = checkpoint:%d commit:%d", reader.checkpointCalls, len(writer.commits))
	}
}

func TestAtomicStepPreCancelledDoesNotOpenOrLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := &atomicTestReader[int]{items: []int{1}}
	writer := &atomicWriterRecorder[int]{}
	step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

	report := step.Run(ctx)

	if report.Status != StatusCancelled || !errors.Is(report.Err, context.Canceled) {
		t.Fatalf("Run() report = %+v", report)
	}
	if reader.openCalls != 0 || reader.closeCalls != 0 || writer.loadCalls != 0 {
		t.Fatalf("lifecycle calls = open:%d close:%d load:%d", reader.openCalls, reader.closeCalls, writer.loadCalls)
	}
}

func TestAtomicStepClosesReaderWithoutCancellationAndJoinsErrors(t *testing.T) {
	closeFailure := errors.New("close failed")
	t.Run("completed becomes failed", func(t *testing.T) {
		reader := &atomicTestReader[int]{closeErr: closeFailure}
		writer := &atomicWriterRecorder[int]{}
		step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

		report := step.Run(context.Background())

		if report.Status != StatusFailed || !errors.Is(report.Err, closeFailure) || reader.closeCalls != 1 {
			t.Fatalf("Run() report = %+v, close calls = %d", report, reader.closeCalls)
		}
	})

	t.Run("prior cancellation is preserved", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reader := &atomicTestReader[int]{readErr: context.Canceled, closeErr: closeFailure}
		reader.readHook = cancel
		writer := &atomicWriterRecorder[int]{}
		step := newAtomicIntStep(t, reader, writer, 1, IdentityProcessor[int](), RetryPolicy{}, SkipPolicy{})

		report := step.Run(ctx)

		if report.Status != StatusCancelled || !errors.Is(report.Err, context.Canceled) || !errors.Is(report.Err, closeFailure) {
			t.Fatalf("Run() report = %+v", report)
		}
		if reader.closeContextErr != nil {
			t.Fatalf("Close() context error = %v", reader.closeContextErr)
		}
	})
}

func TestAtomicStepDoesNotUseProviderOrLegacyWriterLifecycle(t *testing.T) {
	reader := &atomicTestReader[int]{items: []int{1}}
	atomicWriter := &atomicWriterRecorder[int]{}
	legacyWriter := &atomicLegacyWriter[int]{}
	step := &Step[int, int]{
		name:      "atomic-lifecycle",
		chunkSize: 1,
		reader:    reader,
		processor: IdentityProcessor[int](),
		writer:    legacyWriter,
		atomic:    atomicWriter,
		key:       "checkpoint-key",
	}

	report := step.Run(context.Background())

	if report.Status != StatusCompleted || report.WriteCount != 1 {
		t.Fatalf("Run() report = %+v", report)
	}
	if atomicWriter.openCalls != 0 || atomicWriter.closeCalls != 0 {
		t.Fatalf("provider lifecycle calls = open:%d close:%d", atomicWriter.openCalls, atomicWriter.closeCalls)
	}
	if legacyWriter.openCalls != 0 || legacyWriter.writeCalls != 0 || legacyWriter.closeCalls != 0 {
		t.Fatalf("legacy writer calls = open:%d write:%d close:%d", legacyWriter.openCalls, legacyWriter.writeCalls, legacyWriter.closeCalls)
	}
}
