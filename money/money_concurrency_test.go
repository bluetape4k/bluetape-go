package money

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestMoneyOperationsUseGoroutineStressTester(t *testing.T) {
	const rounds = 512
	var operations atomic.Int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       max(32, runtime.GOMAXPROCS(0)*4),
		RoundsPerTask: rounds,
		Timeout:       10 * time.Second,
	})

	report, err := tester.Run(context.Background(),
		func(context.Context) error {
			value, err := Parse("USD 12.34")
			if err != nil {
				return err
			}
			text, err := value.MarshalText()
			if err != nil {
				return err
			}
			var parsed Money
			if err := parsed.UnmarshalText(text); err != nil {
				return err
			}
			if !parsed.Equal(value) {
				return errors.New("text round trip mismatch")
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			left, _ := New("10.00", USD)
			right, _ := New("2.50", USD)
			sum, err := left.Add(right)
			if err != nil {
				return err
			}
			if sum.String() != "USD 12.50" {
				return errors.New("unexpected sum")
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			usd, _ := New("1.00", USD)
			krw, _ := New("1", KRW)
			if _, err := usd.Add(krw); !errors.Is(err, ErrCurrencyMismatch) {
				return err
			}
			operations.Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stress failed: report=%+v err=%v", report, err)
	}
	if report.Completed != rounds*3 || report.Failures != 0 {
		t.Fatalf("unexpected stress report: %+v", report)
	}
	if operations.Load() != int64(rounds*3) {
		t.Fatalf("expected %d operations, got %d", rounds*3, operations.Load())
	}
}
