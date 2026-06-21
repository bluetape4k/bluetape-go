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

func TestCurrencyByLocaleUsesGoroutineStressTester(t *testing.T) {
	const rounds = 256
	var operations atomic.Int64
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       max(32, runtime.GOMAXPROCS(0)*4),
		RoundsPerTask: rounds,
		Timeout:       10 * time.Second,
	})

	report, err := tester.Run(context.Background(),
		func(context.Context) error {
			got, err := CurrencyByLocale("ko-KR")
			if err != nil {
				return err
			}
			if got != KRW {
				return errors.New("ko-KR currency mismatch")
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			got, err := CurrencyByLocale("en-GB")
			if err != nil {
				return err
			}
			if got != MustParseCurrency("GBP") {
				return errors.New("en-GB currency mismatch")
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			if _, err := CurrencyByLocale("es-PA"); !errors.Is(err, ErrInvalidCurrency) {
				return err
			}
			operations.Add(1)
			return nil
		},
		func(context.Context) error {
			if _, err := CurrencyByLocale("en-u-cu-usd"); !errors.Is(err, ErrInvalidCurrency) {
				return err
			}
			operations.Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("stress failed: report=%+v err=%v", report, err)
	}
	if report.Completed != rounds*4 || report.Failures != 0 {
		t.Fatalf("unexpected stress report: %+v", report)
	}
	if operations.Load() != int64(rounds*4) {
		t.Fatalf("expected %d operations, got %d", rounds*4, operations.Load())
	}
}
