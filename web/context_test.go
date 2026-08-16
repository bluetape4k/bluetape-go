package web_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/web"
)

const validTraceParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func TestExtractRequestContext(t *testing.T) {
	t.Parallel()

	t.Run("trusted proxy preserves all supported values", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		req.Header.Set("X-Request-ID", " request-123 ")
		req.Header.Set("X-Correlation-ID", "correlation-456")
		req.Header.Set("X-Auth-Subject", "user-789")
		req.Header.Set("traceparent", validTraceParent)
		req.Header.Set("tracestate", "vendor=value")

		got, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			TrustedProxy: func(*http.Request) bool { return true },
			GenerateID:   func() (string, error) { return "unused", nil },
		})
		if err != nil {
			t.Fatalf("ExtractRequestContext() error = %v", err)
		}
		want := web.RequestContext{
			RequestID:     "request-123",
			CorrelationID: "correlation-456",
			AuthSubject:   "user-789",
			TraceParent:   validTraceParent,
			TraceState:    "vendor=value",
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("RequestContext = %#v, want %#v", got, want)
		}
	})

	t.Run("untrusted proxy ignores auth and trace", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		req.Header.Set("X-Request-ID", "request-123")
		req.Header.Set("X-Auth-Subject", "user-789")
		req.Header.Set("traceparent", validTraceParent)
		req.Header.Set("tracestate", "vendor=value")

		got, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			TrustedProxy: func(*http.Request) bool { return false },
			GenerateID:   func() (string, error) { return "unused", nil },
		})
		if err != nil {
			t.Fatalf("ExtractRequestContext() error = %v", err)
		}
		if got.RequestID != "request-123" || got.CorrelationID != "request-123" {
			t.Errorf("IDs = %#v, want request-123 fallback", got)
		}
		if got.AuthSubject != "" || got.TraceParent != "" || got.TraceState != "" {
			t.Errorf("untrusted values = %#v, want empty auth/trace values", got)
		}
	})

	t.Run("generates request ID once and reuses it for correlation", func(t *testing.T) {
		calls := 0
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		got, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			GenerateID: func() (string, error) {
				calls++
				return "generated-1", nil
			},
		})
		if err != nil {
			t.Fatalf("ExtractRequestContext() error = %v", err)
		}
		if calls != 1 {
			t.Errorf("GenerateID calls = %d, want 1", calls)
		}
		if got.RequestID != "generated-1" || got.CorrelationID != "generated-1" {
			t.Errorf("generated context = %#v, want generated-1 fallback", got)
		}
	})

	t.Run("returns generator errors", func(t *testing.T) {
		wantErr := errors.New("entropy unavailable")
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		_, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			GenerateID: func() (string, error) { return "", wantErr },
		})
		if !errors.Is(err, wantErr) {
			t.Errorf("error = %v, want %v", err, wantErr)
		}
	})

	t.Run("rejects empty generated ID", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		_, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			GenerateID: func() (string, error) { return " ", nil },
		})
		if !errors.Is(err, web.ErrInvalidRequestContext) {
			t.Errorf("error = %v, want ErrInvalidRequestContext", err)
		}
	})

	t.Run("supports custom header names", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		req.Header.Set("Request-ID", "custom-request")
		req.Header.Set("Correlation-ID", "custom-correlation")
		req.Header.Set("Subject", "custom-subject")
		req.Header.Set("Trace", validTraceParent)
		req.Header.Set("State", "vendor=value")

		got, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			TrustedProxy:        func(*http.Request) bool { return true },
			RequestIDHeader:     "Request-ID",
			CorrelationIDHeader: "Correlation-ID",
			AuthSubjectHeader:   "Subject",
			TraceParentHeader:   "Trace",
			TraceStateHeader:    "State",
		})
		if err != nil {
			t.Fatalf("ExtractRequestContext() error = %v", err)
		}
		if got.RequestID != "custom-request" || got.CorrelationID != "custom-correlation" ||
			got.AuthSubject != "custom-subject" || got.TraceParent != validTraceParent || got.TraceState != "vendor=value" {
			t.Errorf("custom context = %#v", got)
		}
	})

	t.Run("validates custom header names", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		_, err := web.ExtractRequestContext(req, web.RequestContextOptions{RequestIDHeader: "Bad Header"})
		if !errors.Is(err, web.ErrInvalidRequestContext) {
			t.Errorf("error = %v, want ErrInvalidRequestContext", err)
		}
	})

	t.Run("rejects invalid trusted values", func(t *testing.T) {
		tests := []struct {
			name   string
			header string
			value  string
		}{
			{name: "newline", header: "X-Request-ID", value: "request\n123"},
			{name: "control", header: "X-Correlation-ID", value: "correlation\x00"},
			{name: "too long", header: "X-Auth-Subject", value: strings.Repeat("a", 257)},
			{name: "invalid traceparent", header: "traceparent", value: "00-00000000000000000000000000000000-00f067aa0ba902b7-01"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
				req.Header.Set(tt.header, tt.value)
				_, err := web.ExtractRequestContext(req, web.RequestContextOptions{
					TrustedProxy: func(*http.Request) bool { return true },
				})
				if !errors.Is(err, web.ErrInvalidRequestContext) {
					t.Errorf("error = %v, want ErrInvalidRequestContext", err)
				}
			})
		}
	})

	t.Run("rejects duplicate trusted headers", func(t *testing.T) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		req.Header.Add("X-Auth-Subject", "first")
		req.Header.Add("X-Auth-Subject", "second")
		_, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			TrustedProxy: func(*http.Request) bool { return true },
		})
		if !errors.Is(err, web.ErrInvalidRequestContext) {
			t.Fatalf("duplicate trusted header error = %v, want ErrInvalidRequestContext", err)
		}
	})

	t.Run("evaluates trusted proxy once", func(t *testing.T) {
		calls := 0
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/orders", nil)
		_, err := web.ExtractRequestContext(req, web.RequestContextOptions{
			TrustedProxy: func(*http.Request) bool {
				calls++
				return true
			},
		})
		if err != nil {
			t.Fatalf("ExtractRequestContext() error = %v", err)
		}
		if calls != 1 {
			t.Errorf("TrustedProxy calls = %d, want 1", calls)
		}
	})
}

func TestRequestContextRoundTrip(t *testing.T) {
	t.Parallel()

	value := web.RequestContext{RequestID: "request-1", CorrelationID: "correlation-1"}
	ctx := web.WithRequestContext(context.Background(), value)
	got, ok := web.RequestContextFromContext(ctx)
	if !ok || !reflect.DeepEqual(got, value) {
		t.Fatalf("RequestContextFromContext() = %#v, %t; want %#v, true", got, ok, value)
	}

	var emptyContext context.Context
	if got, ok := web.RequestContextFromContext(emptyContext); ok || got != (web.RequestContext{}) {
		t.Errorf("nil context result = %#v, %t; want zero, false", got, ok)
	}
	if got, ok := web.RequestContextFromContext(context.Background()); ok || got != (web.RequestContext{}) {
		t.Errorf("missing context result = %#v, %t; want zero, false", got, ok)
	}

	nilContext := web.WithRequestContext(emptyContext, value)
	if got, ok := web.RequestContextFromContext(nilContext); !ok || !reflect.DeepEqual(got, value) {
		t.Errorf("nil context fallback = %#v, %t; want %#v, true", got, ok, value)
	}
}

func TestWithRequestContextOnRequest(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequestWithContext(parent, http.MethodGet, "https://example.test/orders", nil)
	gotRequest, gotValue, err := web.WithRequestContextOnRequest(request, web.RequestContextOptions{
		GenerateID: func() (string, error) { return "request-1", nil },
	})
	if err != nil {
		t.Fatalf("WithRequestContextOnRequest() error = %v", err)
	}
	if gotRequest == request {
		t.Error("returned request aliases original request")
	}
	if gotValue.RequestID != "request-1" || gotValue.CorrelationID != "request-1" {
		t.Errorf("value = %#v, want generated request context", gotValue)
	}
	stored, ok := web.RequestContextFromContext(gotRequest.Context())
	if !ok || !reflect.DeepEqual(stored, gotValue) {
		t.Errorf("stored context = %#v, %t; want %#v, true", stored, ok, gotValue)
	}
	cancel()
	select {
	case <-gotRequest.Context().Done():
	default:
		t.Error("returned request did not preserve cancellation")
	}

	if _, _, err := web.WithRequestContextOnRequest(nil, web.RequestContextOptions{}); !errors.Is(err, web.ErrInvalidRequestContext) {
		t.Errorf("nil request error = %v, want ErrInvalidRequestContext", err)
	}
}
