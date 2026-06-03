package resilience_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/resilience"
)

func TestRoundTripperRetriesRetryableStatusAndClosesBody(t *testing.T) {
	retry, err := resilience.NewRetry[*http.Response](resilience.RetryOptions{ //nolint:bodyclose
		Name:        "http",
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		Sleeper:     &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	firstBody := &closeRecorder{Reader: strings.NewReader("unavailable")}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		if firstBody.closed {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Status:     "503 Service Unavailable",
			Body:       firstBody,
		}, nil
	})
	client := http.Client{
		Transport: resilience.NewRoundTripper(resilience.RoundTripperOptions{
			Transport:       transport,
			Policies:        []resilience.Policy[*http.Response]{retry},
			RetryableStatus: resilience.RetryableServerError,
		}),
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/catalog", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext failed: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Do failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if !firstBody.closed {
		t.Fatal("retryable response body was not closed before retry")
	}
}

func TestRoundTripperRequiresReplayableRequestBody(t *testing.T) {
	retry, err := resilience.NewRetry[*http.Response](resilience.RetryOptions{ //nolint:bodyclose
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		Sleeper:     &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	request.GetBody = nil

	calls := 0
	transport := resilience.NewRoundTripper(resilience.RoundTripperOptions{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("temporary")
		}),
		Policies: []resilience.Policy[*http.Response]{retry},
	})
	response, err := transport.RoundTrip(request)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err == nil {
		t.Fatal("expected non-replayable request error")
	}
	if calls != 1 {
		t.Fatalf("transport calls = %d, want only the first non-replayable attempt", calls)
	}
}

func TestRoundTripperReplaysRequestBodyAcrossRetries(t *testing.T) {
	retry, err := resilience.NewRetry[*http.Response](resilience.RetryOptions{ //nolint:bodyclose
		MaxAttempts: 2,
		Backoff:     resilience.NoBackoff(),
		Sleeper:     &fakeSleeper{},
	})
	if err != nil {
		t.Fatalf("NewRetry failed: %v", err)
	}

	var bodies []string
	transport := resilience.NewRoundTripper(resilience.RoundTripperOptions{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			payload, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			bodies = append(bodies, string(payload))
			if len(bodies) == 1 {
				return nil, errors.New("temporary")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}),
		Policies: []resilience.Policy[*http.Response]{retry},
	})

	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", bytes.NewBufferString("payload"))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}

	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if len(bodies) != 2 || bodies[0] != "payload" || bodies[1] != "payload" {
		t.Fatalf("bodies = %#v, want payload replayed twice", bodies)
	}
}

func TestHandlerAppliesPolicyErrorHandler(t *testing.T) {
	deny := resilience.PolicyFunc[struct{}](func(resilience.Operation[struct{}]) resilience.Operation[struct{}] {
		return func(context.Context) (struct{}, error) {
			return struct{}{}, errors.New("bulkhead full")
		}
	})
	handler := resilience.NewHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("rejected handler must not run")
	}), resilience.HandlerOptions{
		Policies: []resilience.Policy[struct{}]{deny},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
		},
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "bulkhead full") {
		t.Fatalf("body = %q, want error text", recorder.Body.String())
	}
}

func TestHandlerAppliesTimeoutPolicy(t *testing.T) {
	timeout, err := resilience.NewTimeout[struct{}](resilience.TimeoutOptions{
		Timeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewTimeout failed: %v", err)
	}
	handler := resilience.NewHandler(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		<-req.Context().Done()
	}), resilience.HandlerOptions{
		Policies: []resilience.Policy[struct{}]{timeout},
	})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type closeRecorder struct {
	io.Reader
	closed bool
}

func (r *closeRecorder) Close() error {
	r.closed = true
	return nil
}
