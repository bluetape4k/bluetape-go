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
		}
		Run(t, h)
		return
	}

	for _, mode := range []string{"factory", "control", "provider"} {
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
