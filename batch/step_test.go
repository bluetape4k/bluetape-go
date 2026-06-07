package batch

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestStepRunCompletesChunksAndFilters(t *testing.T) {
	reader := newSliceReader([]int{1, 2, 3, 4, 5})
	writer := &recordingWriter[int]{}
	step := mustStep(t, StepOptions[int, int]{
		Name:      "even-doubles",
		ChunkSize: 2,
		Reader:    reader,
		Processor: ProcessorFunc[int, int](func(ctx context.Context, item int) (int, bool, error) {
			if err := ctx.Err(); err != nil {
				return 0, false, err
			}
			return item * 2, item%2 == 0, nil
		}),
		Writer: writer,
	})

	report := step.Run(context.Background())

	if !report.IsSuccess() {
		t.Fatalf("expected completed report, got %+v", report)
	}
	if report.ReadCount != 5 || report.WriteCount != 2 || report.FilterCount != 3 {
		t.Fatalf("unexpected counts: %+v", report)
	}
	if got := writer.Chunks(); !reflect.DeepEqual(got, [][]int{{4, 8}}) {
		t.Fatalf("unexpected chunks: %#v", got)
	}
	if !reader.closed || !writer.closed {
		t.Fatalf("expected resources to be closed")
	}
}

func TestStepRunReportsPartialFailureCounts(t *testing.T) {
	expected := errors.New("writer failed")
	writer := &recordingWriter[int]{failOnWrite: 2, err: expected}
	step := mustStep(t, StepOptions[int, int]{
		Name:      "partial-write",
		ChunkSize: 2,
		Reader:    newSliceReader([]int{1, 2, 3, 4, 5}),
		Processor: passThroughProcessor[int](),
		Writer:    writer,
	})

	report := step.Run(context.Background())

	if report.Status != StatusFailed {
		t.Fatalf("expected failed status, got %+v", report)
	}
	if !errors.Is(report.Err, expected) {
		t.Fatalf("expected writer error, got %v", report.Err)
	}
	if report.ReadCount != 4 || report.WriteCount != 2 {
		t.Fatalf("expected partial counts read=4 write=2, got %+v", report)
	}
	if got := writer.Chunks(); !reflect.DeepEqual(got, [][]int{{1, 2}}) {
		t.Fatalf("expected first chunk only, got %#v", got)
	}
}

func TestStepRunHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancelAfterFirstReader{cancel: cancel}
	step := mustStep(t, StepOptions[int, int]{
		Name:      "cancelled",
		ChunkSize: 10,
		Reader:    reader,
		Processor: passThroughProcessor[int](),
		Writer:    &recordingWriter[int]{},
	})

	report := step.Run(ctx)

	if report.Status != StatusCancelled {
		t.Fatalf("expected cancelled status, got %+v", report)
	}
	if !errors.Is(report.Err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", report.Err)
	}
	if report.ReadCount != 1 || report.WriteCount != 0 {
		t.Fatalf("unexpected cancellation counts: %+v", report)
	}
}

func TestNewStepValidatesRequiredOptions(t *testing.T) {
	if _, err := NewStep(StepOptions[int, int]{}); err == nil {
		t.Fatal("expected missing name error")
	}
	if _, err := NewStep(StepOptions[int, int]{
		Name:      "bad-chunk",
		ChunkSize: -1,
		Reader:    newSliceReader([]int{1}),
		Processor: passThroughProcessor[int](),
		Writer:    &recordingWriter[int]{},
	}); err == nil {
		t.Fatal("expected negative chunk size error")
	}
	if _, err := NewStep(StepOptions[int, int]{
		Name:      "missing-reader",
		Processor: passThroughProcessor[int](),
		Writer:    &recordingWriter[int]{},
	}); err == nil {
		t.Fatal("expected missing reader error")
	}
}

func TestIdentityProcessorForwardsItemsAndCancellation(t *testing.T) {
	processor := IdentityProcessor[int]()
	value, keep, err := processor.Process(context.Background(), 42)
	if err != nil || !keep || value != 42 {
		t.Fatalf("unexpected identity result value=%d keep=%v err=%v", value, keep, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	value, keep, err = processor.Process(ctx, 42)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got value=%d keep=%v err=%v", value, keep, err)
	}
	if keep {
		t.Fatal("expected cancelled identity processor not to keep item")
	}
}

func TestJobRunStopsAtFailedStep(t *testing.T) {
	first := mustStep(t, StepOptions[int, int]{
		Name:      "first",
		ChunkSize: 2,
		Reader:    newSliceReader([]int{1, 2}),
		Processor: passThroughProcessor[int](),
		Writer:    &recordingWriter[int]{},
	})
	secondErr := errors.New("second failed")
	second := mustStep(t, StepOptions[int, int]{
		Name:      "second",
		ChunkSize: 2,
		Reader:    newSliceReader([]int{3}),
		Processor: ProcessorFunc[int, int](func(context.Context, int) (int, bool, error) {
			return 0, false, secondErr
		}),
		Writer: &recordingWriter[int]{},
	})
	job, err := NewJob("two-step", first, second)
	if err != nil {
		t.Fatalf("NewJob failed: %v", err)
	}

	report := job.Run(context.Background())

	if report.Status != StatusPartial {
		t.Fatalf("expected partial job status, got %+v", report)
	}
	if !errors.Is(report.Err, secondErr) {
		t.Fatalf("expected second step error, got %v", report.Err)
	}
	if len(report.Children) != 2 {
		t.Fatalf("expected two child reports, got %+v", report)
	}
	if report.ReadCount != 3 || report.WriteCount != 2 {
		t.Fatalf("unexpected aggregate counts: %+v", report)
	}
}

func TestStepRunWithGoroutineStressTester(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 8,
		Timeout:       time.Second,
	})

	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		writer := &recordingWriter[int]{}
		step := mustStep(t, StepOptions[int, int]{
			Name:      "stress",
			ChunkSize: 3,
			Reader:    newSliceReader([]int{1, 2, 3, 4, 5, 6}),
			Processor: passThroughProcessor[int](),
			Writer:    writer,
		})
		stepReport := step.Run(ctx)
		if !stepReport.IsSuccess() {
			return stepReport.Err
		}
		if stepReport.WriteCount != 6 {
			return errors.New("unexpected write count")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("stress run failed: %v", err)
	}
	if report.Completed != 8 {
		t.Fatalf("expected 8 completed stress runs, got %+v", report)
	}
}

func TestStepRunWithAsyncJobTesterCancellation(t *testing.T) {
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: 25 * time.Millisecond,
	})

	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		step := mustStep(t, StepOptions[int, int]{
			Name:      "async-cancel",
			ChunkSize: 1,
			Reader:    blockingReader{},
			Processor: passThroughProcessor[int](),
			Writer:    &recordingWriter[int]{},
		})
		stepReport := step.Run(ctx)
		if stepReport.Status != StatusCancelled {
			return errors.New("expected cancelled batch report")
		}
		return stepReport.Err
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got report=%+v err=%v", report, err)
	}
	if report.Failures != 1 {
		t.Fatalf("expected one async failure, got %+v", report)
	}
}

func mustStep[I any, O any](tb testing.TB, options StepOptions[I, O]) *Step[I, O] {
	tb.Helper()
	step, err := NewStep(options)
	if err != nil {
		tb.Fatalf("NewStep failed: %v", err)
	}
	return step
}

func passThroughProcessor[T any]() Processor[T, T] {
	return ProcessorFunc[T, T](func(ctx context.Context, item T) (T, bool, error) {
		if err := ctx.Err(); err != nil {
			var zero T
			return zero, false, err
		}
		return item, true, nil
	})
}

type sliceReader[T any] struct {
	values []T
	index  int
	closed bool
}

func newSliceReader[T any](values []T) *sliceReader[T] {
	return &sliceReader[T]{values: append([]T(nil), values...)}
}

func (r *sliceReader[T]) Open(context.Context) error { return nil }

func (r *sliceReader[T]) Read(ctx context.Context) (T, bool, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	if r.index >= len(r.values) {
		return zero, false, nil
	}
	value := r.values[r.index]
	r.index++
	return value, true, nil
}

func (r *sliceReader[T]) Close(context.Context) error {
	r.closed = true
	return nil
}

type recordingWriter[T any] struct {
	mu          sync.Mutex
	chunks      [][]T
	writeCalls  int
	failOnWrite int
	err         error
	closed      bool
}

func (w *recordingWriter[T]) Open(context.Context) error { return nil }

func (w *recordingWriter[T]) Write(ctx context.Context, items []T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writeCalls++
	if w.failOnWrite > 0 && w.writeCalls == w.failOnWrite {
		return w.err
	}
	copied := append([]T(nil), items...)
	w.chunks = append(w.chunks, copied)
	return nil
}

func (w *recordingWriter[T]) Close(context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *recordingWriter[T]) Chunks() [][]T {
	w.mu.Lock()
	defer w.mu.Unlock()
	copied := make([][]T, len(w.chunks))
	for i, chunk := range w.chunks {
		copied[i] = append([]T(nil), chunk...)
	}
	return copied
}

type cancelAfterFirstReader struct {
	cancel func()
	read   bool
}

func (r *cancelAfterFirstReader) Open(context.Context) error { return nil }

func (r *cancelAfterFirstReader) Read(ctx context.Context) (int, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if r.read {
		return 0, false, nil
	}
	r.read = true
	r.cancel()
	return 1, true, nil
}

func (r *cancelAfterFirstReader) Close(context.Context) error { return nil }

type blockingReader struct{}

func (blockingReader) Open(context.Context) error { return nil }

func (blockingReader) Read(ctx context.Context) (int, bool, error) {
	<-ctx.Done()
	return 0, false, ctx.Err()
}

func (blockingReader) Close(context.Context) error { return nil }
