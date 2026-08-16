package echoadapter_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/resilience"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestResilienceRetriesReplayableBodyAndRestoresStore(t *testing.T) {
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
		c.Set("attempt", attempts)
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

func TestResilienceCancellationMapsToProblem(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
	ctx, recorder := newEchoContextFromRequest(request)
	err := echoadapter.WrapResilience(func(echo.Context) error {
		return context.Canceled
	}, echoadapter.ResilienceOptions{})(ctx)
	if err != nil || recorder.Code != http.StatusRequestTimeout {
		t.Fatalf("err=%v status=%d, want nil/408", err, recorder.Code)
	}
}
