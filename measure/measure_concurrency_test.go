package measure_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/measure"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestMeasureParsingWithGoroutineStressTester(t *testing.T) {
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 128,
		Timeout:       2 * time.Second,
	})

	var mu sync.Mutex
	seen := make(map[string]struct{})
	report, err := tester.Run(context.Background(), func(context.Context) error {
		value, err := measure.ParseLength("1 km")
		if err != nil {
			return err
		}
		meters, err := value.In(measure.LengthMeter)
		if err != nil {
			return err
		}
		if meters != 1000 {
			return errors.New("unexpected conversion")
		}
		sum, err := value.Add(measure.Must(500, measure.LengthMeter))
		if err != nil {
			return err
		}
		text, err := sum.Format(measure.LengthKilometer)
		if err != nil {
			return err
		}
		if text != "1.5 km" {
			return errors.New("unexpected formatting")
		}
		speed, err := measure.VelocityFromLengthTime(value, measure.Must(100, measure.TimeSecond))
		if err != nil {
			return err
		}
		if got, err := speed.In(measure.VelocityMeterPerSecond); err != nil || got != 10 {
			return errors.New("unexpected velocity")
		}
		mu.Lock()
		seen[value.String()] = struct{}{}
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("stress failed: %v", err)
	}
	if report.Completed != 128 {
		t.Fatalf("expected 128 completed runs, got %+v", report)
	}
	if len(seen) != 1 {
		t.Fatalf("expected stable formatting, got %v", seen)
	}
}

func TestMeasureAsyncJobTesterRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{Workers: 1})
	report, err := tester.Run(ctx, func(context.Context) error {
		t.Fatal("job should not run after caller cancellation")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got report=%+v err=%v", report, err)
	}
	if report.Started != 0 {
		t.Fatalf("cancelled context should prevent local work, got %+v", report)
	}
}
