package echoadapter_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bluetape4k/bluetape-go/resilience"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestResilienceRetriesReplayableBody(t *testing.T) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	ctx, recorder := newEchoContextFromRequest(request)
	attempts := 0
	policy := retryPolicy(3)
	err = echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		if value := c.Get("attempt"); value != nil {
			t.Fatalf("store value leaked into attempt %d: %v", attempts, value)
		}
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return err
		}
		if string(body) != "payload" {
			return errors.New("request body was not replayed")
		}
		if attempts == 1 {
			return errors.New("transient route failure")
		}
		return c.NoContent(http.StatusNoContent)
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{policy}})(ctx)
	if err != nil || recorder.Code != http.StatusNoContent || attempts != 2 {
		t.Fatalf("err=%v status=%d attempts=%d, want nil/204/2", err, recorder.Code, attempts)
	}
}

func TestResilienceDoesNotRetryStoreMutationAfterRouteError(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	attempts := 0
	err := echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		c.Set("private", "attempt-local")
		return errors.New("store mutation failed")
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(3)}})(ctx)
	if err != nil || recorder.Code != http.StatusServiceUnavailable || attempts != 1 || ctx.Get("private") != nil {
		t.Fatalf("err=%v status=%d attempts=%d private=%v, want nil/503/1/nil", err, recorder.Code, attempts, ctx.Get("private"))
	}
}

func TestResilienceSuccessfulStoreMutationPersists(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	err := echoadapter.WrapResilience(func(c echo.Context) error {
		c.Set("request-scoped", "value")
		return c.NoContent(http.StatusNoContent)
	}, echoadapter.ResilienceOptions{})(ctx)
	if err != nil || recorder.Code != http.StatusNoContent || ctx.Get("request-scoped") != "value" {
		t.Fatalf("err=%v status=%d value=%v, want nil/204/value", err, recorder.Code, ctx.Get("request-scoped"))
	}
}

func TestResilienceDoesNotRetryCommittedResponse(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	attempts := 0
	err := echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		if writeErr := c.String(http.StatusAccepted, "committed"); writeErr != nil {
			return writeErr
		}
		return errors.New("post-commit failure")
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(3)}})(ctx)
	if err != nil || recorder.Code != http.StatusAccepted || recorder.Body.String() != "committed" || attempts != 1 {
		t.Fatalf("err=%v status=%d body=%q attempts=%d, want nil/202/committed/1", err, recorder.Code, recorder.Body.String(), attempts)
	}
}

func TestResilienceDoesNotRetryRawWriterResponse(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	attempts := 0
	err := echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		_, _ = c.Response().Writer.Write([]byte("raw response"))
		return errors.New("post-raw-write failure")
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(3)}})(ctx)
	if err != nil || recorder.Code != http.StatusOK || recorder.Body.String() != "raw response" || attempts != 1 || !ctx.Response().Committed {
		t.Fatalf("err=%v status=%d body=%q attempts=%d committed=%t, want nil/200/raw response/1/true", err, recorder.Code, recorder.Body.String(), attempts, ctx.Response().Committed)
	}
}

func TestResilienceProblemResponseIsCommittedBeforeRouteError(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	attempts := 0
	err := echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		if writeErr := echoadapter.AbortWithProblem(c, errors.New("private route detail")); writeErr != nil {
			return writeErr
		}
		return errors.New("route failed after response")
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(3)}})(ctx)
	if err != nil || recorder.Code != http.StatusInternalServerError || attempts != 1 || !ctx.Response().Committed {
		t.Fatalf("err=%v status=%d attempts=%d committed=%t, want nil/500/1/true", err, recorder.Code, attempts, ctx.Response().Committed)
	}
}

func TestResilienceRunsNonReplayableBodyOnce(t *testing.T) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.GetBody = nil
	ctx, recorder := newEchoContextFromRequest(request)
	attempts := 0
	err = echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		body, readErr := io.ReadAll(c.Request().Body)
		if readErr != nil || string(body) != "payload" {
			return errors.New("request body was not delivered")
		}
		return c.NoContent(http.StatusNoContent)
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(3)}})(ctx)
	if err != nil || recorder.Code != http.StatusNoContent || attempts != 1 {
		t.Fatalf("err=%v status=%d attempts=%d, want nil/204/1", err, recorder.Code, attempts)
	}
}

func TestResilienceDoesNotRetryNonReplayableBodyAfterRouteError(t *testing.T) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/orders", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.GetBody = nil
	ctx, recorder := newEchoContextFromRequest(request)
	attempts := 0
	err = echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		_, _ = io.ReadAll(c.Request().Body)
		return errors.New("non-replayable route failure")
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(3)}})(ctx)
	if err != nil || recorder.Code != http.StatusServiceUnavailable || attempts != 1 {
		t.Fatalf("err=%v status=%d attempts=%d, want nil/503/1", err, recorder.Code, attempts)
	}
}

func TestResiliencePreCanceledRequestDoesNotInvokeRoute(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
	requestContext, cancel := context.WithCancel(request.Context())
	cancel()
	request = request.WithContext(requestContext)
	ctx, recorder := newEchoContextFromRequest(request)
	attempts := 0
	err := echoadapter.WrapResilience(func(echo.Context) error {
		attempts++
		return nil
	}, echoadapter.ResilienceOptions{})(ctx)
	if err != nil || recorder.Code != http.StatusRequestTimeout || attempts != 0 {
		t.Fatalf("err=%v status=%d attempts=%d, want nil/408/0", err, recorder.Code, attempts)
	}
}

func TestResilienceRestoresPathAndParamsBetweenAttempts(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders/42", nil)
	ctx.SetPath("/orders/:id")
	ctx.SetParamNames("id")
	ctx.SetParamValues("42")
	attempts := 0
	err := echoadapter.WrapResilience(func(c echo.Context) error {
		attempts++
		if attempts > 1 && (c.Path() != "/orders/:id" || len(c.ParamNames()) != 1 || c.ParamNames()[0] != "id" || c.ParamValues()[0] != "42") {
			return errors.New("path or params leaked between attempts")
		}
		c.SetPath("/mutated")
		c.SetParamNames("other")
		c.SetParamValues("value")
		if attempts == 1 {
			return errors.New("transient route failure")
		}
		return c.NoContent(http.StatusNoContent)
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(2)}})(ctx)
	if err != nil || recorder.Code != http.StatusNoContent || attempts != 2 {
		t.Fatalf("err=%v status=%d attempts=%d, want nil/204/2", err, recorder.Code, attempts)
	}
}

func TestResilienceConcurrentAttemptIsolation(t *testing.T) {
	const requests = 32
	wrapped := echoadapter.WrapResilience(func(c echo.Context) error {
		tracker, _ := c.Request().Context().Value(concurrentAttemptKey{}).(*atomic.Int32)
		if tracker == nil {
			return errors.New("missing request tracker")
		}
		if tracker.Add(1) == 1 {
			return errors.New("transient route failure")
		}
		return c.NoContent(http.StatusNoContent)
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(2)}})

	var group sync.WaitGroup
	errorsCh := make(chan error, requests)
	for i := 0; i < requests; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			tracker := &atomic.Int32{}
			request := httptest.NewRequestWithContext(
				context.WithValue(context.Background(), concurrentAttemptKey{}, tracker),
				http.MethodGet,
				"http://example.test/orders",
				nil,
			)
			ctx, recorder := newEchoContextFromRequest(request)
			if err := wrapped(ctx); err != nil {
				errorsCh <- err
				return
			}
			if recorder.Code != http.StatusNoContent || tracker.Load() != 2 {
				errorsCh <- errors.New("request attempt state was not isolated")
			}
		}()
	}
	group.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatal(err)
	}
}

type concurrentAttemptKey struct{}

func retryPolicy(maxAttempts int) resilience.Policy[struct{}] {
	return resilience.PolicyFunc[struct{}](func(operation resilience.Operation[struct{}]) resilience.Operation[struct{}] {
		return func(ctx context.Context) (struct{}, error) {
			var (
				result struct{}
				err    error
			)
			for attempt := 0; attempt < maxAttempts; attempt++ {
				result, err = operation(ctx)
				if err == nil || resilience.IsNonRetryable(err) {
					return result, err
				}
			}
			return result, err
		}
	})
}

func TestResilienceOuterErrorHandlerGetsRedactedCause(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	var observed error
	cause := errors.New("private route detail")
	err := echoadapter.WrapResilience(func(echo.Context) error {
		return cause
	}, echoadapter.ResilienceOptions{
		ErrorHandler: func(_ echo.Context, err error) { observed = err },
	})(ctx)
	if err != nil || recorder.Code != http.StatusOK || observed == nil || observed.Error() != "resilience operation failed" || !errors.Is(observed, cause) {
		t.Fatalf("err=%v status=%d observed=%v, want nil/200/redacted cause", err, recorder.Code, observed)
	}
}

func TestResilienceRecordsRedactedObserverByDefault(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	cause := errors.New("private route detail")
	err := echoadapter.WrapResilience(func(echo.Context) error {
		return cause
	}, echoadapter.ResilienceOptions{})(ctx)
	observed, ok := echoadapter.ResilienceError(ctx)
	if err != nil || recorder.Code != http.StatusServiceUnavailable || !ok || observed == nil || observed.Error() != "resilience operation failed" || strings.Contains(observed.Error(), "private route detail") || !errors.Is(observed, cause) {
		t.Fatalf("err=%v status=%d observed=%v ok=%t, want nil/503/redacted observer", err, recorder.Code, observed, ok)
	}
}

func TestResilienceCancellationMapsToProblem(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
	ctx, recorder := newEchoContextFromRequest(request)
	attempts := 0
	err := echoadapter.WrapResilience(func(echo.Context) error {
		attempts++
		return context.Canceled
	}, echoadapter.ResilienceOptions{Policies: []resilience.Policy[struct{}]{retryPolicy(3)}})(ctx)
	if err != nil || recorder.Code != http.StatusRequestTimeout || attempts != 1 {
		t.Fatalf("err=%v status=%d attempts=%d, want nil/408/1", err, recorder.Code, attempts)
	}
}
