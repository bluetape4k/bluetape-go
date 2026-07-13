package ratelimittest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunMemoryHarness(t *testing.T) { Run(t, MemoryHarness()) }

func TestConformanceNoRefillCasesTolerateAdapterLatency(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, Harness, Config, string) error
	}{
		{name: "rejection-result", run: runRejection},
		{name: "cancel-after-linearize", run: runCancelAfter},
		{name: "lost-response", run: runLostResponse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := MemoryHarness()
			newAllow := harness.New
			harness.New = func(tb testing.TB, config Config) (AllowFunc, error) {
				allow, err := newAllow(tb, config)
				if err != nil {
					return nil, err
				}
				return func(ctx context.Context, key string, tokens int64) (Result, error) {
					result, err := allow(ctx, key, tokens)
					time.Sleep(20 * time.Millisecond)
					return result, err
				}, nil
			}

			config := Config{RatePerSecond: 100, Burst: 5, IdleTTL: time.Second}
			if err := tt.run(t, harness, config, "latency-"+tt.name); err != nil {
				t.Fatalf("latency-sensitive conformance case failed: %v", err)
			}
		})
	}
}

func TestRunRefillWaitsForEventuallyAllowedResult(t *testing.T) {
	var calls int
	harness := Harness{
		New: func(testing.TB, Config) (AllowFunc, error) {
			return func(context.Context, string, int64) (Result, error) {
				calls++
				switch calls {
				case 1:
					return Result{Allowed: true, Requested: 5}, nil
				case 2:
					return Result{Requested: 1, RetryAfter: time.Millisecond}, nil
				default:
					return Result{Allowed: true, Requested: 1}, nil
				}
			}, nil
		},
	}
	config := Config{RatePerSecond: 100, Burst: 5, IdleTTL: time.Second}

	if err := runRefill(t, harness, config, "eventual-refill"); err != nil {
		t.Fatalf("eventual refill failed: %v", err)
	}
	if calls != 3 {
		t.Fatalf("refill attempts = %d, want 3", calls)
	}
}

const diagnosticMarker = "forbidden-ratelimittest-diagnostic-marker"

func TestRunRedactsAdapterDiagnostics(t *testing.T) {
	if mode := os.Getenv("RATELIMITTEST_DIAGNOSTIC_MODE"); mode != "" {
		h := MemoryHarness()
		switch mode {
		case "factory":
			h.New = func(testing.TB, Config) (AllowFunc, error) {
				return nil, errors.New(diagnosticMarker)
			}
		case "control":
			h.Control = diagnosticControl{Control: h.Control}
		case "provider":
			h.New = func(testing.TB, Config) (AllowFunc, error) {
				return func(context.Context, string, int64) (Result, error) {
					return Result{}, &diagnosticProviderError{}
				}, nil
			}
			h.IsProviderError = func(err error) bool {
				var target *diagnosticProviderError
				return errors.As(err, &target)
			}
		case "blocking":
			h.New = func(testing.TB, Config) (AllowFunc, error) {
				return func(context.Context, string, int64) (Result, error) { select {} }, nil
			}
		}
		Run(t, h)
		return
	}

	for _, mode := range []string{"factory", "control", "provider", "blocking"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunRedactsAdapterDiagnostics$")
			cmd.Env = append(os.Environ(), "RATELIMITTEST_DIAGNOSTIC_MODE="+mode)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatal("broken adapter unexpectedly passed conformance")
			}
			if strings.Contains(string(output), diagnosticMarker) {
				t.Fatal("conformance diagnostics exposed an adapter marker")
			}
		})
	}
}

type diagnosticControl struct{ Control }

func (c diagnosticControl) FailNext(context.Context, string, error) error {
	return errors.New(diagnosticMarker)
}

type diagnosticProviderError struct{}

func (*diagnosticProviderError) Error() string { return diagnosticMarker }
