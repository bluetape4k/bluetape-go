package webtest_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/webtest"
)

func TestRunCapturesResponseAndNextRequest(t *testing.T) {
	var nextRequest *http.Request

	webtest.Run(t, webtest.Scenario{
		Name: "captures response and request",
		Adapter: func(next http.Handler) http.Handler {
			return next
		},
		NewRequest: func(ctx context.Context) *http.Request {
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/catalog", nil)
			req.Header.Set("X-Request-ID", "request-1")
			return req
		},
		Next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			nextRequest = req
			w.Header().Set("X-Test", "observed")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("accepted"))
		}),
		Assert: func(t *testing.T, got webtest.Observation) {
			if got.StatusCode != http.StatusAccepted {
				t.Fatalf("status = %d, want %d", got.StatusCode, http.StatusAccepted)
			}
			if got.Header.Get("X-Test") != "observed" {
				t.Fatalf("X-Test = %q, want observed", got.Header.Get("X-Test"))
			}
			if string(got.Body) != "accepted" {
				t.Fatalf("body = %q, want accepted", got.Body)
			}
			if got.NextCalls != 1 || got.NextRequest == nil {
				t.Fatalf("next observation = calls:%d request:%v, want one request", got.NextCalls, got.NextRequest)
			}
			if got.NextRequest.Header.Get("X-Request-ID") != "request-1" {
				t.Fatalf("request ID = %q, want request-1", got.NextRequest.Header.Get("X-Request-ID"))
			}
		},
	})

	if nextRequest == nil {
		t.Fatal("next request was not captured")
	}
}

func TestRunSupportsInFlightCancellation(t *testing.T) {
	started := make(chan struct{})
	var startOnce sync.Once
	var cancel context.CancelFunc
	cancelDone := make(chan struct{})
	go func() {
		<-started
		cancel()
		close(cancelDone)
	}()

	webtest.Run(t, webtest.Scenario{
		Name: "cancels in flight",
		Adapter: func(next http.Handler) http.Handler {
			return next
		},
		NewRequest: func(parent context.Context) *http.Request {
			ctx, cancelRequest := context.WithCancel(parent)
			cancel = cancelRequest
			return httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/slow", nil)
		},
		Next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			startOnce.Do(func() { close(started) })
			<-req.Context().Done()
			w.WriteHeader(http.StatusNoContent)
		}),
		Timeout: 2 * time.Second,
		Assert: func(t *testing.T, got webtest.Observation) {
			if got.StatusCode != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", got.StatusCode, http.StatusNoContent)
			}
			if got.NextCalls != 1 {
				t.Fatalf("next calls = %d, want 1", got.NextCalls)
			}
		},
	})

	select {
	case <-cancelDone:
	case <-time.After(time.Second):
		t.Fatal("next handler did not start")
	}
}

func TestRunPreservesPreCancelledRequest(t *testing.T) {
	webtest.Run(t, webtest.Scenario{
		Name:    "preserves pre-cancelled request",
		Adapter: identityAdapter,
		NewRequest: func(parent context.Context) *http.Request {
			ctx, cancel := context.WithCancel(parent)
			cancel()
			return httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/cancelled", nil)
		},
		Next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !errors.Is(req.Context().Err(), context.Canceled) {
				t.Fatalf("request context error = %v, want context.Canceled", req.Context().Err())
			}
			w.WriteHeader(http.StatusNoContent)
		}),
		Assert: func(t *testing.T, got webtest.Observation) {
			if got.StatusCode != http.StatusNoContent || got.NextCalls != 1 {
				t.Fatalf("observation = %#v, want 204 and one next call", got)
			}
		},
	})
}

func TestRunRejectsScenarioInput(t *testing.T) {
	if os.Getenv("WEBTEST_EXPECT_FAILURE") == "1" {
		webtest.Run(t, invalidScenario(os.Getenv("WEBTEST_FAILURE_CASE")))
		return
	}

	cases := []struct {
		name string
	}{
		{name: "missing name"},
		{name: "missing adapter"},
		{name: "missing request"},
		{name: "missing next"},
		{name: "missing assertion"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunRejectsScenarioInput$", "-test.v")
			cmd.Env = append(os.Environ(), "WEBTEST_EXPECT_FAILURE=1", "WEBTEST_FAILURE_CASE="+tc.name)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("invalid scenario %q was accepted; output=%s", tc.name, output)
			}
			if !strings.Contains(string(output), "webtest:") {
				t.Fatalf("invalid scenario %q failed without contract error: %s", tc.name, output)
			}
		})
	}
}

func TestRunRejectsTimeoutEvenAfterCancellationCleanup(t *testing.T) {
	if os.Getenv("WEBTEST_EXPECT_TIMEOUT_FAILURE") == "1" {
		webtest.Run(t, webtest.Scenario{
			Name:       "timeout",
			Adapter:    identityAdapter,
			NewRequest: requestFactory,
			Next:       http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) { <-req.Context().Done() }),
			Timeout:    10 * time.Millisecond,
			Assert:     noOpAssertion,
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunRejectsTimeoutEvenAfterCancellationCleanup$", "-test.v")
	cmd.Env = append(os.Environ(), "WEBTEST_EXPECT_TIMEOUT_FAILURE=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("timeout scenario was accepted; output=%s", output)
	}
	if !strings.Contains(string(output), "exceeded timeout") {
		t.Fatalf("timeout failed without the expected contract error: %s", output)
	}
}

func TestCloseTrackerRecordsOwnedClose(t *testing.T) {
	tracker := webtest.NewCloseTracker(strings.NewReader("payload"))
	got, err := io.ReadAll(tracker)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("payload = %q, want payload", got)
	}
	if tracker.Closed() || tracker.CloseCount() != 0 {
		t.Fatalf("tracker before close = closed:%t count:%d", tracker.Closed(), tracker.CloseCount())
	}
	if err := tracker.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := tracker.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
	if !tracker.Closed() || tracker.CloseCount() != 2 {
		t.Fatalf("tracker after close = closed:%t count:%d, want true/2", tracker.Closed(), tracker.CloseCount())
	}
}

func TestRunDoesNotShareObservationState(t *testing.T) {
	var mu sync.Mutex
	var observations []string

	webtest.Run(t,
		webtest.Scenario{
			Name:       "first",
			Adapter:    identityAdapter,
			NewRequest: requestFactory,
			Next:       statusHandler(http.StatusCreated, "first"),
			Assert:     appendObservation(&mu, &observations),
		},
		webtest.Scenario{
			Name:       "second",
			Adapter:    identityAdapter,
			NewRequest: requestFactory,
			Next:       statusHandler(http.StatusNoContent, "second"),
			Assert:     appendObservation(&mu, &observations),
		},
	)

	mu.Lock()
	defer mu.Unlock()
	if len(observations) != 2 || observations[0] != "first" || observations[1] != "second" {
		t.Fatalf("observations = %#v, want [first second]", observations)
	}
}

func identityAdapter(next http.Handler) http.Handler { return next }

func requestFactory(ctx context.Context) *http.Request {
	return httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/", nil)
}

func noOpAssertion(*testing.T, webtest.Observation) {}

func statusHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

func appendObservation(mu *sync.Mutex, observations *[]string) func(*testing.T, webtest.Observation) {
	return func(_ *testing.T, got webtest.Observation) {
		mu.Lock()
		defer mu.Unlock()
		*observations = append(*observations, string(got.Body))
	}
}

func invalidScenario(name string) webtest.Scenario {
	base := webtest.Scenario{
		Name:       name,
		Adapter:    identityAdapter,
		NewRequest: requestFactory,
		Next:       http.NotFoundHandler(),
		Assert:     noOpAssertion,
	}
	switch name {
	case "missing name":
		base.Name = ""
	case "missing adapter":
		base.Adapter = nil
	case "missing request":
		base.NewRequest = nil
	case "missing next":
		base.Next = nil
	case "missing assertion":
		base.Assert = nil
	default:
		panic(fmt.Sprintf("unknown invalid scenario %q", name))
	}
	return base
}
