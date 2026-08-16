package echoadapter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/labstack/echo/v4"
)

// WrapResilience wraps one Echo route handler with common resilience policies.
// 하나의 Echo route handler를 공통 resilience policy로 감싼다.
func WrapResilience(next echo.HandlerFunc, options ResilienceOptions) echo.HandlerFunc {
	policies := make([]resilience.Policy[struct{}], 0, len(options.Policies))
	for _, policy := range options.Policies {
		if isNilInterface(policy) {
			continue
		}
		policies = append(policies, policy)
	}

	return func(c echo.Context) error {
		if isNilInterface(c) {
			return nil
		}
		if next == nil {
			return c.NoContent(http.StatusNotFound)
		}

		original := c.Request()
		ctx := context.Background()
		if original != nil && original.Context() != nil {
			ctx = original.Context()
		}
		state := newResilienceAttemptState(c)
		_, err := resilience.Run(ctx, func(policyCtx context.Context) (struct{}, error) {
			return runResilienceAttempt(policyCtx, c, original, state, next)
		}, policies...)
		if err == nil {
			return nil
		}

		observerErr := newResilienceObserverError(err)
		if options.ErrorHandler != nil {
			options.ErrorHandler(c, observerErr)
			return nil
		}
		if c.Response() != nil && c.Response().Committed {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return AbortWithProblem(c, err)
		}
		return AbortWithProblem(c, resilienceProblemError{})
	}
}

type resilienceAttemptState struct {
	request     *http.Request
	path        string
	paramNames  []string
	paramValues []string
	header      http.Header
}

func newResilienceAttemptState(c echo.Context) resilienceAttemptState {
	return resilienceAttemptState{
		request:     c.Request(),
		path:        c.Path(),
		paramNames:  append([]string(nil), c.ParamNames()...),
		paramValues: append([]string(nil), c.ParamValues()...),
		header:      cloneHeader(c.Response().Header()),
	}
}

func runResilienceAttempt(
	policyCtx context.Context,
	c echo.Context,
	original *http.Request,
	state resilienceAttemptState,
	next echo.HandlerFunc,
) (struct{}, error) {
	var zero struct{}
	if err := policyCtx.Err(); err != nil {
		return zero, err
	}
	if original == nil {
		return zero, resilienceInvalidRequestError{}
	}

	restoreResilienceAttemptState(c, state)
	attemptRequest, body, err := cloneResilienceRequest(policyCtx, original)
	if err != nil {
		return zero, resilience.NonRetryable(err)
	}
	attempt := &attemptContext{Context: c, previous: make(map[string]any)}
	c.SetRequest(attemptRequest)
	defer func() {
		c.SetRequest(original)
		attempt.restore()
		if body != nil {
			_ = body.Close()
		}
	}()

	attemptErr := next(attempt)
	if attemptErr == nil {
		if err := policyCtx.Err(); err != nil {
			return zero, err
		}
		return zero, nil
	}
	if c.Response().Committed || (original.Body != nil && original.GetBody == nil && requestMayHaveBody(original)) {
		return zero, resilience.NonRetryable(attemptErr)
	}
	return zero, attemptErr
}

func cloneResilienceRequest(ctx context.Context, original *http.Request) (*http.Request, io.ReadCloser, error) {
	request := original.WithContext(ctx)
	if original.Body == nil || original.Body == http.NoBody {
		request.Body = http.NoBody
		return request, nil, nil
	}
	if original.GetBody == nil {
		return nil, nil, fmt.Errorf("request body is not replayable")
	}
	body, err := original.GetBody()
	if err != nil {
		return nil, nil, err
	}
	request.Body = body
	return request, body, nil
}

func requestMayHaveBody(request *http.Request) bool {
	return request != nil && request.Body != nil && request.Body != http.NoBody
}

func restoreResilienceAttemptState(c echo.Context, state resilienceAttemptState) {
	c.SetRequest(state.request)
	c.SetPath(state.path)
	c.SetParamNames(state.paramNames...)
	c.SetParamValues(state.paramValues...)
	if c.Response() != nil && !c.Response().Committed {
		restoreHeader(c.Response().Header(), state.header)
	}
}

type attemptContext struct {
	echo.Context
	previous map[string]any
}

func (c *attemptContext) Set(key string, value any) {
	if _, seen := c.previous[key]; !seen {
		c.previous[key] = c.Get(key)
	}
	c.Context.Set(key, value)
}

func (c *attemptContext) restore() {
	for key, value := range c.previous {
		c.Context.Set(key, value)
	}
}

func cloneHeader(header http.Header) http.Header {
	if header == nil {
		return nil
	}
	clone := make(http.Header, len(header))
	for key, values := range header {
		clone[key] = append([]string(nil), values...)
	}
	return clone
}

func restoreHeader(header, snapshot http.Header) {
	for key := range header {
		delete(header, key)
	}
	for key, values := range snapshot {
		header[key] = append([]string(nil), values...)
	}
}

type resilienceObserverError struct {
	cause          error
	nonRetryable   bool
	retryExhausted bool
	timeout        bool
	circuitOpen    bool
	bulkhead       bool
	canceled       bool
	deadline       bool
}

func newResilienceObserverError(err error) resilienceObserverError {
	return resilienceObserverError{
		cause:          err,
		nonRetryable:   errors.Is(err, resilience.ErrNonRetryable),
		retryExhausted: errors.Is(err, resilience.ErrRetryExhausted),
		timeout:        errors.Is(err, resilience.ErrTimeout),
		circuitOpen:    errors.Is(err, resilience.ErrCircuitOpen),
		bulkhead:       errors.Is(err, resilience.ErrBulkheadRejected),
		canceled:       errors.Is(err, context.Canceled),
		deadline:       errors.Is(err, context.DeadlineExceeded),
	}
}

func (resilienceObserverError) Error() string { return "resilience operation failed" }

func (e resilienceObserverError) Unwrap() error { return e.cause }

func (e resilienceObserverError) Is(target error) bool {
	switch target {
	case resilience.ErrNonRetryable:
		return e.nonRetryable
	case resilience.ErrRetryExhausted:
		return e.retryExhausted
	case resilience.ErrTimeout:
		return e.timeout
	case resilience.ErrCircuitOpen:
		return e.circuitOpen
	case resilience.ErrBulkheadRejected:
		return e.bulkhead
	case context.Canceled:
		return e.canceled
	case context.DeadlineExceeded:
		return e.deadline
	default:
		return false
	}
}

type resilienceProblemError struct{}

func (resilienceProblemError) Error() string { return "resilience policy rejected request" }

func (resilienceProblemError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: http.StatusText(http.StatusServiceUnavailable), Status: http.StatusServiceUnavailable, Detail: "resilience policy rejected request"}
}

type resilienceInvalidRequestError struct{}

func (resilienceInvalidRequestError) Error() string { return "invalid resilience request" }

func (resilienceInvalidRequestError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: http.StatusText(http.StatusInternalServerError), Status: http.StatusInternalServerError, Detail: "invalid resilience request"}
}

var (
	_ web.ProblemError = resilienceProblemError{}
	_ web.ProblemError = resilienceInvalidRequestError{}
)
