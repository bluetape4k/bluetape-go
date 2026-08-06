package locktest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRunMemoryHarness(t *testing.T) {
	Run(t, MemoryHarness())
}

const diagnosticMarker = "forbidden-locktest-diagnostic-marker"

func TestRunRedactsAdapterDiagnostics(t *testing.T) {
	if mode := os.Getenv("LOCKTEST_DIAGNOSTIC_MODE"); mode != "" {
		h := MemoryHarness()
		switch mode {
		case "factory":
			h.New = func(testing.TB, Config) (AcquireFunc, error) {
				return nil, errors.New(diagnosticMarker)
			}
		case "owner":
			h.Control = diagnosticControl{Control: h.Control}
		case "provider":
			h.New = func(testing.TB, Config) (AcquireFunc, error) {
				return func(context.Context) (ReleaseFunc, error) {
					return nil, &diagnosticProviderError{}
				}, nil
			}
			h.IsProviderError = func(err error) bool {
				var target *diagnosticProviderError
				return errors.As(err, &target)
			}
		case "blocking":
			h.New = func(testing.TB, Config) (AcquireFunc, error) {
				return func(context.Context) (ReleaseFunc, error) { select {} }, nil
			}
		}
		Run(t, h)
		return
	}

	for _, mode := range []string{"factory", "owner", "provider", "blocking"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunRedactsAdapterDiagnostics$")
			cmd.Env = append(os.Environ(), "LOCKTEST_DIAGNOSTIC_MODE="+mode)
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

func (c diagnosticControl) Owner(context.Context, Config) (string, error) {
	return diagnosticMarker, errors.New(diagnosticMarker)
}

type diagnosticProviderError struct{}

func (*diagnosticProviderError) Error() string { return diagnosticMarker }
