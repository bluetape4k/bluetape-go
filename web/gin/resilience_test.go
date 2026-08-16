package ginadapter_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
	router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil))
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
			_ = c.Error(errors.New("transient route failure"))
			return
		}
		if _, exists := c.Get("attempt"); exists || c.Writer.Header().Get("X-Attempt") != "" || len(c.Errors) != 0 {
			_ = c.Error(errors.New("attempt state leaked"))
			return
		}
		finalErrors = len(c.Errors)
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil))
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
	routeErr := errors.New("late route failure")
	var observed error
	var observedErrors []*gin.Error
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Next()
		observedErrors = append([]*gin.Error(nil), c.Errors...)
		if last := c.Errors.Last(); last != nil {
			observed = last.Err
		}
	})
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		attempts++
		c.Header("X-Partial", "kept")
		c.String(http.StatusOK, "partial")
		entry := c.Error(routeErr)
		_ = entry.SetMeta("private route metadata")
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "partial" || attempts != 1 {
		t.Fatalf("response = (%d, body=%q, attempts=%d), want (200, partial, 1)", recorder.Code, recorder.Body.String(), attempts)
	}
	if recorder.Header().Get("X-Partial") != "kept" {
		t.Fatalf("X-Partial = %q, want kept", recorder.Header().Get("X-Partial"))
	}
	if !resilience.IsNonRetryable(observed) {
		t.Fatalf("observed error = %v, want non-retryable marker", observed)
	}
	var nonRetryable resilience.NonRetryableError
	if !errors.As(observed, &nonRetryable) {
		t.Fatalf("observed error = %T, want preserved NonRetryableError cause", observed)
	}
	if len(observedErrors) != 2 {
		t.Fatalf("observed Gin errors = %d, want original route error plus final resilience error", len(observedErrors))
	}
	if observedErrors[0].Type != gin.ErrorTypePrivate || observedErrors[0].Meta != nil {
		t.Fatalf("sanitized route error = (type=%d, meta=%#v), want private type and nil meta", observedErrors[0].Type, observedErrors[0].Meta)
	}
	if !errors.Is(observedErrors[0].Err, routeErr) {
		t.Fatalf("sanitized route error = %v, want original cause", observedErrors[0].Err)
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
			_ = c.Error(errors.New("body was not available"))
			return
		}
		_ = c.Error(errors.New("body request failed"))
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", io.NopCloser(strings.NewReader("payload")))
	req.ContentLength = int64(len("payload"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if attempts != 1 || !resilience.IsNonRetryable(observed) {
		t.Fatalf("attempts = %d, observed = %v, want one non-retryable attempt", attempts, observed)
	}
}

func TestWrapResilienceTreatsUnknownLengthBodyAsNonReplayable(t *testing.T) {
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
		_, _ = io.ReadAll(c.Request.Body)
		_ = c.Error(errors.New("unknown-length body request failed"))
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", strings.NewReader("payload"))
	req.ContentLength = 0
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
			_ = c.Error(errors.New("retry body"))
			return
		}
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", nil)
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
		_ = c.Error(errors.New(raw))
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))
	customCalled := false
	customAborted := false
	router.GET("/custom", ginadapter.WrapResilience(func(c *gin.Context) {
		_ = c.Error(errors.New(raw))
	}, ginadapter.ResilienceOptions{
		Policies: []resilience.Policy[struct{}]{retry},
		ErrorHandler: func(c *gin.Context, err error) {
			customCalled = err != nil
			customAborted = c.IsAborted()
			c.Status(http.StatusTeapot)
		},
	}))

	defaultRecorder := httptest.NewRecorder()
	router.ServeHTTP(defaultRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/default", nil))
	if defaultRecorder.Code != http.StatusServiceUnavailable || strings.Contains(defaultRecorder.Body.String(), raw) {
		t.Fatalf("default response = (%d, %q), want safe 503", defaultRecorder.Code, defaultRecorder.Body.String())
	}
	customRecorder := httptest.NewRecorder()
	router.ServeHTTP(customRecorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/custom", nil))
	if customRecorder.Code != http.StatusTeapot || !customCalled || !customAborted {
		t.Fatalf("custom response = (%d, called=%t, aborted=%t), want (418, true, true)", customRecorder.Code, customCalled, customAborted)
	}
}

func TestWrapResilienceRedactsGinLoggerErrorDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const raw = "postgres://user:password@example.test/orders"
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{MaxAttempts: 1})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}
	var logs bytes.Buffer
	router := gin.New()
	router.Use(gin.LoggerWithWriter(&logs))
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		_ = c.Error(errors.New(raw))
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil))
	if strings.Contains(logs.String(), raw) || strings.Contains(logs.String(), "password") {
		t.Fatalf("Gin logger leaked raw error: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "resilience operation failed") {
		t.Fatalf("Gin logger did not record the safe observer: %q", logs.String())
	}
}

func TestWrapResilienceLeavesPanicToOuterGinRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/panic", ginadapter.WrapResilience(func(*gin.Context) {
		panic("route panic")
	}, ginadapter.ResilienceOptions{}))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/panic", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 from outer Recovery", recorder.Code)
	}
}

func TestWrapResilienceIsolatesConcurrentRetryState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	retry, err := resilience.NewRetry[struct{}](resilience.RetryOptions{MaxAttempts: 2, RetryIf: func(error) bool { return true }})
	if err != nil {
		t.Fatalf("NewRetry() error = %v", err)
	}
	var mu sync.Mutex
	attempts := make(map[string]int)
	router := gin.New()
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		requestID := c.GetHeader("X-Test-ID")
		mu.Lock()
		attempts[requestID]++
		attempt := attempts[requestID]
		mu.Unlock()
		if attempt == 1 {
			c.Set("attempt", requestID)
			c.Header("X-Attempt", requestID)
			_ = c.Error(errors.New("transient concurrent failure"))
			return
		}
		if _, exists := c.Get("attempt"); exists || c.Writer.Header().Get("X-Attempt") != "" {
			_ = c.Error(errors.New("retry state leaked"))
			return
		}
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retry}}))

	const requestCount = 32
	var wait sync.WaitGroup
	wait.Add(requestCount)
	statuses := make(chan int, requestCount)
	for index := 0; index < requestCount; index++ {
		index := index
		go func() {
			defer wait.Done()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
			req.Header.Set("X-Test-ID", fmt.Sprintf("request-%d", index))
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			statuses <- recorder.Code
		}()
	}
	wait.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusNoContent {
			t.Fatalf("concurrent response status = %d, want 204", status)
		}
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
	router.ServeHTTP(notFound, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/missing", nil))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("nil next status = %d, want 404", notFound.Code)
	}
}

func TestWrapResilienceMapsCancellationAfterHandlerReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestContext, cancel := context.WithCancel(context.Background())
	router := gin.New()
	router.GET("/canceled", ginadapter.WrapResilience(func(*gin.Context) {
		cancel()
	}, ginadapter.ResilienceOptions{}))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(requestContext, http.MethodGet, "http://example.test/canceled", nil)
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want 408 after handler-return cancellation", recorder.Code)
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
	router.ServeHTTP(recorder, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil))
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("response = (%d, called=%t), want (204, true)", recorder.Code, called)
	}
}

type fakePolicy struct{}

func (*fakePolicy) Apply(operation resilience.Operation[struct{}]) resilience.Operation[struct{}] {
	return operation
}

var _ resilience.Policy[struct{}] = (*fakePolicy)(nil)
