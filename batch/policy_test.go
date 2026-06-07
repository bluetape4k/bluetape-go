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

func TestStepRunComposesRetryAndSkipPolicies(t *testing.T) {
	transientErr := errors.New("transient")
	permanentErr := errors.New("permanent")
	processorAttempts := make(map[int]int)
	writer := &recordingWriter[int]{failOnWrite: 1, err: transientErr}
	retry := mustRetryPolicy(t, 2, func(err error) bool {
		return errors.Is(err, transientErr)
	})
	skip := mustSkipPolicy(t, 1, func(err error) bool {
		return errors.Is(err, permanentErr)
	})
	step := mustStep(t, StepOptions[int, int]{
		Name:      "retry-skip",
		ChunkSize: 2,
		Reader:    newSliceReader([]int{1, 2, 3, 4}),
		Processor: ProcessorFunc[int, int](func(_ context.Context, item int) (int, bool, error) {
			processorAttempts[item]++
			if item == 2 && processorAttempts[item] == 1 {
				return 0, false, transientErr
			}
			if item == 3 {
				return 0, false, permanentErr
			}
			return item, true, nil
		}),
		Writer:      writer,
		RetryPolicy: retry,
		SkipPolicy:  skip,
	})

	report := step.Run(context.Background())

	if !report.IsSuccess() {
		t.Fatalf("expected completed report, got %+v", report)
	}
	if report.ReadCount != 4 || report.WriteCount != 3 || report.SkipCount != 1 || report.RetryCount != 2 {
		t.Fatalf("unexpected counts: %+v", report)
	}
	if got := writer.Chunks(); !reflect.DeepEqual(got, [][]int{{1, 2}, {4}}) {
		t.Fatalf("unexpected chunks: %#v", got)
	}
}

func TestStepRunRestartsFromCheckpoint(t *testing.T) {
	store := NewMemoryCheckpointStore()
	expected := errors.New("write failed")
	firstWriter := &recordingWriter[int]{failOnWrite: 2, err: expected}
	first := mustStep(t, StepOptions[int, int]{
		Name:            "restart",
		ChunkSize:       2,
		Reader:          newCheckpointReader([]int{1, 2, 3, 4, 5}),
		Processor:       passThroughProcessor[int](),
		Writer:          firstWriter,
		CheckpointStore: store,
		CheckpointKey:   "restart-key",
	})

	firstReport := first.Run(context.Background())
	if firstReport.Status != StatusFailed {
		t.Fatalf("expected first run to fail, got %+v", firstReport)
	}
	if !errors.Is(firstReport.Err, expected) {
		t.Fatalf("expected writer error, got %v", firstReport.Err)
	}
	if firstReport.ReadCount != 4 || firstReport.WriteCount != 2 {
		t.Fatalf("unexpected first run counts: %+v", firstReport)
	}
	checkpoint, ok, err := store.Load(context.Background(), "restart-key")
	if err != nil || !ok || checkpoint != 2 {
		t.Fatalf("expected checkpoint 2, got checkpoint=%v ok=%v err=%v", checkpoint, ok, err)
	}

	secondWriter := &recordingWriter[int]{}
	second := mustStep(t, StepOptions[int, int]{
		Name:            "restart",
		ChunkSize:       2,
		Reader:          newCheckpointReader([]int{1, 2, 3, 4, 5}),
		Processor:       passThroughProcessor[int](),
		Writer:          secondWriter,
		CheckpointStore: store,
		CheckpointKey:   "restart-key",
	})

	secondReport := second.Run(context.Background())
	if !secondReport.IsSuccess() {
		t.Fatalf("expected restart to complete, got %+v", secondReport)
	}
	if secondReport.ReadCount != 3 || secondReport.WriteCount != 3 {
		t.Fatalf("unexpected restart counts: %+v", secondReport)
	}
	if got := secondWriter.Chunks(); !reflect.DeepEqual(got, [][]int{{3, 4}, {5}}) {
		t.Fatalf("unexpected restarted chunks: %#v", got)
	}
	checkpoint, ok, err = store.Load(context.Background(), "restart-key")
	if err != nil || !ok || checkpoint != 5 {
		t.Fatalf("expected final checkpoint 5, got checkpoint=%v ok=%v err=%v", checkpoint, ok, err)
	}
}

func TestRetrySkipPoliciesDoNotHandleContextCancellation(t *testing.T) {
	retry := mustRetryPolicy(t, 3, func(error) bool { return true })
	skip := mustSkipPolicy(t, 10, func(error) bool { return true })
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers: 1,
		Timeout: 25 * time.Millisecond,
	})

	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		step := mustStep(t, StepOptions[int, int]{
			Name:        "cancel-policy",
			ChunkSize:   1,
			Reader:      blockingReader{},
			Processor:   passThroughProcessor[int](),
			Writer:      &recordingWriter[int]{},
			RetryPolicy: retry,
			SkipPolicy:  skip,
		})
		stepReport := step.Run(ctx)
		if stepReport.Status != StatusCancelled {
			return errors.New("expected cancelled report")
		}
		if stepReport.RetryCount != 0 || stepReport.SkipCount != 0 {
			return errors.New("context cancellation was retried or skipped")
		}
		return stepReport.Err
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got report=%+v err=%v", report, err)
	}
}

func TestMemoryCheckpointStoreWithGoroutineStressTester(t *testing.T) {
	store := NewMemoryCheckpointStore()
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 16,
		Timeout:       time.Second,
	})

	report, err := tester.Run(context.Background(), func(ctx context.Context) error {
		if err := store.Save(ctx, "shared", time.Now().UnixNano()); err != nil {
			return err
		}
		_, _, err := store.Load(ctx, "shared")
		return err
	})
	if err != nil {
		t.Fatalf("stress run failed: %v", err)
	}
	if report.Completed != 16 {
		t.Fatalf("expected 16 completed stress runs, got %+v", report)
	}
}

func mustRetryPolicy(tb testing.TB, maxAttempts int, retryIf ErrorPredicate) RetryPolicy {
	tb.Helper()
	policy, err := RetryErrors(maxAttempts, retryIf)
	if err != nil {
		tb.Fatalf("RetryErrors failed: %v", err)
	}
	return policy
}

func mustSkipPolicy(tb testing.TB, maxSkips int, skipIf ErrorPredicate) SkipPolicy {
	tb.Helper()
	policy, err := SkipErrors(maxSkips, skipIf)
	if err != nil {
		tb.Fatalf("SkipErrors failed: %v", err)
	}
	return policy
}

type checkpointReader struct {
	values []int
	index  int
	mu     sync.Mutex
}

func newCheckpointReader(values []int) *checkpointReader {
	return &checkpointReader{values: append([]int(nil), values...)}
}

func (r *checkpointReader) Open(context.Context) error { return nil }

func (r *checkpointReader) Read(ctx context.Context) (int, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.index >= len(r.values) {
		return 0, false, nil
	}
	value := r.values[r.index]
	r.index++
	return value, true, nil
}

func (r *checkpointReader) Restore(ctx context.Context, checkpoint any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	index, ok := checkpoint.(int)
	if !ok {
		return errors.New("checkpoint must be an int")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.index = index
	return nil
}

func (r *checkpointReader) Checkpoint(ctx context.Context) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.index, true, nil
}

func (r *checkpointReader) Close(context.Context) error { return nil }
