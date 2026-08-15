package ginadapter_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bluetape4k/bluetape-go/resilience"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/gin-gonic/gin"
)

func TestWrapResilienceRunsRouteAndReturnsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := 0
	router := gin.New()
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		called++
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusNoContent || called != 1 {
		t.Fatalf("response = (%d, calls=%d), want (204, 1)", recorder.Code, called)
	}
}

func TestWrapResilienceRetriesUncommittedRouteErrorAndClearsAttemptState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{
		MaxAttempts: 3,
		RetryIf:     func(error) bool { return true },
	})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}
	attempts := 0
	var finalErrors int
	router := gin.New()
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		attempts++
		if attempts < 3 {
			c.Set("attempt", attempts)
			c.Header("X-Attempt", "failed")
			c.Error(errors.New("transient route failure"))
			return
		}
		if _, exists := c.Get("attempt"); exists || c.Writer.Header().Get("X-Attempt") != "" || len(c.Errors) != 0 {
			c.Error(errors.New("attempt state leaked"))
			return
		}
		finalErrors = len(c.Errors)
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusNoContent || attempts != 3 || finalErrors != 0 {
		t.Fatalf("response = (%d, attempts=%d, finalErrors=%d), want (204, 3, 0)", recorder.Code, attempts, finalErrors)
	}
}

func TestWrapResilienceMarksCommittedResponseNonRetryable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{MaxAttempts: 3})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}
	attempts := 0
	var observed error
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Next()
		if last := c.Errors.Last(); last != nil {
			observed = last.Err
		}
	})
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		attempts++
		c.Header("X-Partial", "kept")
		c.String(http.StatusOK, "partial")
		c.Error(errors.New("late route failure"))
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "partial" || attempts != 1 {
		t.Fatalf("response = (%d, body=%q, attempts=%d), want (200, partial, 1)", recorder.Code, recorder.Body.String(), attempts)
	}
	if recorder.Header().Get("X-Partial") != "kept" {
		t.Fatalf("X-Partial = %q, want kept", recorder.Header().Get("X-Partial"))
	}
	if !resilience.IsNonRetryable(observed) {
		t.Fatalf("observed error = %v, want non-retryable marker", observed)
	}
}

func TestWrapResilienceDoesNotRetryNonReplayableBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{MaxAttempts: 3})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}
	attempts := 0
	var observed error
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Next()
		if last := c.Errors.Last(); last != nil {
			observed = last.Err
		}
	})
	router.POST("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		attempts++
		payload, _ := io.ReadAll(c.Request.Body)
		if string(payload) != "payload" {
			c.Error(errors.New("body was not available"))
			return
		}
		c.Error(errors.New("body request failed"))
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	req := httptest.NewRequest(http.MethodPost, "http://example.test/orders", io.NopCloser(strings.NewReader("payload")))
	req.ContentLength = int64(len("payload"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if attempts != 1 || !resilience.IsNonRetryable(observed) {
		t.Fatalf("attempts = %d, observed = %v, want one non-retryable attempt", attempts, observed)
	}
}

func TestWrapResilienceReplaysBodyWithGetBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{MaxAttempts: 2})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}
	attempts := 0
	var payloads []string
	var mu sync.Mutex
	router := gin.New()
	router.POST("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		attempts++
		payload, _ := io.ReadAll(c.Request.Body)
		mu.Lock()
		payloads = append(payloads, string(payload))
		mu.Unlock()
		if attempts == 1 {
			c.Error(errors.New("retry body"))
			return
		}
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	req := httptest.NewRequest(http.MethodPost, "http://example.test/orders", nil)
	req.Body = io.NopCloser(strings.NewReader("payload"))
	req.ContentLength = int64(len("payload"))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("payload")), nil }
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent || attempts != 2 || !bytes.Equal([]byte(strings.Join(payloads, ",")), []byte("payload,payload")) {
		t.Fatalf("response = (%d, attempts=%d, payloads=%v), want (204, 2, [payload payload])", recorder.Code, attempts, payloads)
	}
}

func TestWrapResilienceDefaultErrorIsSafeAndCustomHandlerAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{MaxAttempts: 1})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}
	const raw = "database password"
	router := gin.New()
	router.GET("/default", ginadapter.WrapResilience(func(c *gin.Context) {
		c.Error(errors.New(raw))
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))
	customCalled := false
	customAborted := false
	router.GET("/custom", ginadapter.WrapResilience(func(c *gin.Context) {
		c.Error(errors.New(raw))
	}, ginadapter.ResilienceOptions{
		Policies: []resilience.Policy[struct{}]{retry},
		ErrorHandler: func(c *gin.Context, err error) {
			customCalled = err != nil
			customAborted = c.IsAborted()
			c.Status(http.StatusTeapot)
		},
	}))

	defaultRecorder := httptest.NewRecorder()
	router.ServeHTTP(defaultRecorder, httptest.NewRequest(http.MethodGet, "http://example.test/default", nil))
	if defaultRecorder.Code != http.StatusServiceUnavailable || strings.Contains(defaultRecorder.Body.String(), raw) {
		t.Fatalf("default response = (%d, %q), want safe 503", defaultRecorder.Code, defaultRecorder.Body.String())
	}
	customRecorder := httptest.NewRecorder()
	router.ServeHTTP(customRecorder, httptest.NewRequest(http.MethodGet, "http://example.test/custom", nil))
	if customRecorder.Code != http.StatusTeapot || !customCalled || !customAborted {
		t.Fatalf("custom response = (%d, called=%t, aborted=%t), want (418, true, true)", customRecorder.Code, customCalled, customAborted)
	}
}

func TestWrapResiliencePropagatesCancellationAndNilNextIsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	var called bool
	router := gin.New()
	router.GET("/canceled", ginadapter.WrapResilience(func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{}))
	router.GET("/missing", ginadapter.WrapResilience(nil, ginadapter.ResilienceOptions{}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(canceled, http.MethodGet, "http://example.test/canceled", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusRequestTimeout || called {
		t.Fatalf("canceled response = (%d, called=%t), want (408, false)", recorder.Code, called)
	}
	notFound := httptest.NewRecorder()
	router.ServeHTTP(notFound, httptest.NewRequest(http.MethodGet, "http://example.test/missing", nil))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("nil next status = %d, want 404", notFound.Code)
	}
}

func TestWrapResilienceCopiesPoliciesAndSkipsTypedNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var typedNil *fakePolicy
	called := false
	policy := resilience.PolicyFunc[struct{}](func(operation resilience.Operation[struct{}]) resilience.Operation[struct{}] {
		return func(ctx context.Context) (struct{}, error) {
			called = true
			return operation(ctx)
		}
	})
	policies := []resilience.Policy[struct{}]{typedNil, policy}
	router := gin.New()
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) { c.Status(http.StatusNoContent) }, ginadapter.ResilienceOptions{Policies: policies}))
	policies[1] = nil
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("response = (%d, called=%t), want (204, true)", recorder.Code, called)
	}
}

type fakePolicy struct{}

func (*fakePolicy) Apply(operation resilience.Operation[struct{}]) resilience.Operation[struct{}] {
	return operation
}

var _ resilience.Policy[struct{}] = (*fakePolicy)(nil)
