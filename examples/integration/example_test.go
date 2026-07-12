package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/batch"
	"github.com/bluetape4k/bluetape-go/cache"
	"github.com/bluetape4k/bluetape-go/id"
	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/leader"
	redisleader "github.com/bluetape4k/bluetape-go/leader/redis"
	redislock "github.com/bluetape4k/bluetape-go/lock/redis"
	btredis "github.com/bluetape4k/bluetape-go/redis"
	"github.com/bluetape4k/bluetape-go/resilience"
	redistestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/redis"
	"github.com/bluetape4k/bluetape-go/workflow"
	"github.com/bluetape4k/bluetape-go/workreport"
	"github.com/redis/go-redis/v9"
)

func Example_batchWorkflowJWTCacheAndResilienceRecipe() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Date(2026, 6, 24, 9, 0, 0, 0, time.UTC)
	requestIDs, err := id.NewUUIDV7Generator(
		id.WithUUIDTime(func() time.Time { return now }),
		id.WithUUIDReader(strings.NewReader("abcdefghijklmnopqrstuvwxyzabcdef")),
	)
	if err != nil {
		panic(err)
	}
	requestID, err := requestIDs.NextString()
	if err != nil {
		panic(err)
	}

	provider, err := jwt.NewFixedHMACProvider(
		jwt.HS256,
		[]byte("0123456789abcdef0123456789abcdef"),
		jwt.WithClock(func() time.Time { return now }),
		jwt.WithKeyIDGenerator(func() (string, error) { return "integration-kid", nil }),
	)
	if err != nil {
		panic(err)
	}
	token, err := provider.Compose(
		jwt.WithSubject("account-42"),
		jwt.WithAudience("integration-api"),
		jwt.WithExpiresAfter(time.Hour),
		jwt.WithJWTID(requestID),
	)
	if err != nil {
		panic(err)
	}
	reader, err := provider.Parse(
		token,
		jwt.WithExpectedSubject("account-42"),
		jwt.WithExpectedAudience("integration-api"),
		jwt.WithExpirationRequired(),
		jwt.WithParseClock(func() time.Time { return now.Add(time.Minute) }),
	)
	if err != nil {
		panic(err)
	}

	profiles := cache.NewMemory[string, string]()
	var profile string
	var importReport batch.Report

	flow := workflow.Sequential(
		"signup-import",
		workreport.StopOnFailure,
		func(ctx context.Context) workreport.Report {
			loaded, err := loadProfileWithRetry(ctx, profiles, reader.Subject())
			if err != nil {
				return workreport.Failed("profile-cache", err)
			}
			profile = loaded
			return workreport.Completed("profile-cache")
		},
		func(ctx context.Context) workreport.Report {
			report, err := runOrderImport(ctx)
			importReport = report
			if err != nil {
				return workreport.Failed("order-import", err)
			}
			return workreport.Completed("order-import")
		},
	)

	flowReport := flow.Run(ctx)
	fmt.Println(flowReport.Status)
	fmt.Println(reader.Subject(), reader.Kid())
	fmt.Println(profile)
	fmt.Println(importReport.Status, importReport.ReadCount, importReport.WriteCount, importReport.SkipCount, importReport.RetryCount)

	// Output:
	// completed
	// account-42 integration-kid
	// profile:account-42
	// completed 3 2 1 1
}

func TestConcurrentIDAndJWTRecipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	provider, err := jwt.NewFixedHMACProvider(
		jwt.HS256,
		[]byte("0123456789abcdef0123456789abcdef"),
		jwt.WithKeyIDGenerator(func() (string, error) { return "concurrent-kid", nil }),
	)
	if err != nil {
		t.Fatalf("NewFixedHMACProvider() error = %v", err)
	}

	const workers = 16
	var seen sync.Map
	errs := make(chan error, workers)
	var wg sync.WaitGroup

	for worker := range workers {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			requestID, err := id.NewUUIDV7()
			if err != nil {
				errs <- fmt.Errorf("worker %d request id: %w", worker, err)
				return
			}
			if _, loaded := seen.LoadOrStore(requestID, struct{}{}); loaded {
				errs <- fmt.Errorf("worker %d duplicated request id %q", worker, requestID)
				return
			}

			subject := fmt.Sprintf("account-%02d", worker)
			token, err := provider.Compose(
				jwt.WithSubject(subject),
				jwt.WithJWTID(requestID),
				jwt.WithExpiresAfter(time.Minute),
			)
			if err != nil {
				errs <- fmt.Errorf("worker %d compose token: %w", worker, err)
				return
			}

			if err := ctx.Err(); err != nil {
				errs <- fmt.Errorf("worker %d context: %w", worker, err)
				return
			}
			reader, err := provider.Parse(token, jwt.WithExpectedSubject(subject))
			if err != nil {
				errs <- fmt.Errorf("worker %d parse token: %w", worker, err)
				return
			}
			if got := reader.Subject(); got != subject {
				errs <- fmt.Errorf("worker %d subject = %q, want %q", worker, got, subject)
			}
		}(worker)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRedisCoordinationRecipeSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_INTEGRATION_RECIPE_SMOKE") != "1" {
		t.Skip("set BLUETAPE_INTEGRATION_RECIPE_SMOKE=1 to run the Docker-backed Redis integration recipe")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	addr := redistestcontainer.Start(ctx, t)
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		_ = client.Close()
	})

	namespace := fmt.Sprintf("bluetape:integration:%d", time.Now().UnixNano())
	mutex, err := redislock.New(client, redislock.Options{
		Key:   namespace + ":lock",
		TTL:   10 * time.Second,
		Token: "worker-1",
	})
	if err != nil {
		t.Fatalf("redis lock: %v", err)
	}
	lease, err := mutex.TryLock(ctx)
	if errors.Is(err, btredis.ErrCommitUnknown) && lease != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, cleanupErr := lease.Unlock(cleanupCtx)
		cleanupCancel()
		t.Fatalf("try lock commit unknown: %v", errors.Join(err, cleanupErr))
	}
	if err != nil {
		t.Fatalf("try lock: %v", err)
	}
	t.Cleanup(func() {
		unlocked, err := lease.Unlock(context.Background())
		if err != nil {
			t.Fatalf("unlock redis lease: %v", err)
		}
		if !unlocked {
			t.Fatalf("redis lease was not owned during cleanup")
		}
	})

	elector, err := redisleader.New(client, leader.Options{
		Group:         namespace + ":group",
		MemberID:      "worker-1",
		Lease:         5 * time.Second,
		RenewInterval: time.Second,
		KeyPrefix:     namespace + ":leader",
	})
	if err != nil {
		t.Fatalf("redis leader elector: %v", err)
	}
	if err := elector.Campaign(ctx); err != nil {
		t.Fatalf("campaign leadership: %v", err)
	}
	t.Cleanup(func() {
		if err := elector.Resign(context.Background()); err != nil {
			t.Fatalf("resign leadership: %v", err)
		}
	})
	if !elector.IsLeader() {
		t.Fatalf("elector did not retain leadership")
	}

	report, err := runOrderImport(ctx)
	if err != nil {
		t.Fatalf("run batch under leader lock: %v", err)
	}
	if report.WriteCount != 2 || report.SkipCount != 1 || report.RetryCount != 1 {
		t.Fatalf("batch report = writes %d skips %d retries %d, want 2/1/1", report.WriteCount, report.SkipCount, report.RetryCount)
	}

	resultKey := namespace + ":result"
	if err := client.Set(ctx, resultKey, report.WriteCount, time.Minute).Err(); err != nil {
		t.Fatalf("write redis result: %v", err)
	}
	got, err := client.Get(ctx, resultKey).Int()
	if err != nil {
		t.Fatalf("read redis result: %v", err)
	}
	if got != 2 {
		t.Fatalf("redis result = %d, want 2", got)
	}
}

var (
	errInvalidOrder   = errors.New("invalid order")
	errTemporaryWrite = errors.New("temporary writer failure")
)

type order struct {
	ID    string
	Valid bool
}

type receipt struct {
	OrderID string
}

func loadProfileWithRetry(ctx context.Context, profiles *cache.Memory[string, string], subject string) (string, error) {
	var attempts int
	retryPolicy, err := resilience.NewRetry[string](resilience.RetryOptions{
		Name:        "profile-loader",
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		RetryIf: func(err error) bool {
			return errors.Is(err, errTemporaryWrite)
		},
	})
	if err != nil {
		return "", err
	}
	timeoutPolicy, err := resilience.NewTimeout[string](resilience.TimeoutOptions{
		Name:    "profile-timeout",
		Timeout: time.Second,
	})
	if err != nil {
		return "", err
	}

	return profiles.GetOrLoad(ctx, subject, time.Minute, func(ctx context.Context, key string) (string, error) {
		return resilience.Run(ctx, func(ctx context.Context) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			attempts++
			if attempts == 1 {
				return "", errTemporaryWrite
			}
			return "profile:" + key, nil
		}, retryPolicy, timeoutPolicy)
	})
}

func runOrderImport(ctx context.Context) (batch.Report, error) {
	retryPolicy, err := batch.RetryErrors(2, func(err error) bool {
		return errors.Is(err, errTemporaryWrite)
	})
	if err != nil {
		return batch.Report{}, err
	}
	skipPolicy, err := batch.SkipErrors(1, func(err error) bool {
		return errors.Is(err, errInvalidOrder)
	})
	if err != nil {
		return batch.Report{}, err
	}

	reader := &sliceCheckpointReader[order]{
		items: []order{
			{ID: "order-1", Valid: true},
			{ID: "order-2", Valid: false},
			{ID: "order-3", Valid: true},
		},
	}
	writer := &failingOnceWriter[receipt]{failOnce: true}
	step, err := batch.NewStep(batch.StepOptions[order, receipt]{
		Name:      "orders",
		ChunkSize: 2,
		Reader:    reader,
		Processor: batch.ProcessorFunc[order, receipt](func(ctx context.Context, item order) (receipt, bool, error) {
			if err := ctx.Err(); err != nil {
				return receipt{}, false, err
			}
			if !item.Valid {
				return receipt{}, false, errInvalidOrder
			}
			return receipt{OrderID: item.ID}, true, nil
		}),
		Writer:          writer,
		RetryPolicy:     retryPolicy,
		SkipPolicy:      skipPolicy,
		CheckpointStore: batch.NewMemoryCheckpointStore(),
		CheckpointKey:   "orders",
	})
	if err != nil {
		return batch.Report{}, err
	}

	report := step.Run(ctx)
	if report.IsFailure() {
		if report.Err != nil {
			return report, report.Err
		}
		return report, fmt.Errorf("batch report status %s", report.Status)
	}
	return report, nil
}

type sliceCheckpointReader[T any] struct {
	items []T
	index int
}

func (r *sliceCheckpointReader[T]) Open(ctx context.Context) error {
	return ctx.Err()
}

func (r *sliceCheckpointReader[T]) Read(ctx context.Context) (T, bool, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	if r.index >= len(r.items) {
		return zero, false, nil
	}
	item := r.items[r.index]
	r.index++
	return item, true, nil
}

func (r *sliceCheckpointReader[T]) Close(context.Context) error {
	return nil
}

func (r *sliceCheckpointReader[T]) Restore(ctx context.Context, checkpoint any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	index, ok := checkpoint.(int)
	if !ok {
		return fmt.Errorf("checkpoint has type %T, want int", checkpoint)
	}
	if index < 0 || index > len(r.items) {
		return fmt.Errorf("checkpoint index %d out of range", index)
	}
	r.index = index
	return nil
}

func (r *sliceCheckpointReader[T]) Checkpoint(ctx context.Context) (any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	return r.index, true, nil
}

type failingOnceWriter[T any] struct {
	failOnce bool
	failed   bool
	values   []T
}

func (w *failingOnceWriter[T]) Open(ctx context.Context) error {
	return ctx.Err()
}

func (w *failingOnceWriter[T]) Write(ctx context.Context, values []T) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.failOnce && !w.failed {
		w.failed = true
		return errTemporaryWrite
	}
	w.values = append(w.values, values...)
	return nil
}

func (w *failingOnceWriter[T]) Close(context.Context) error {
	return nil
}
