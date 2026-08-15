package ginadapter

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/gin-gonic/gin"
)

// WrapResilience는 하나의 Gin route handler를 공통 resilience policy로 감싼다.
//
// 재시도 사이에는 request, body, headers, keys, params, errors를 복원한다.
// 이미 응답이 기록된 오류는 NonRetryable로 표시해 writer를 덮어쓰거나
// side effect를 중복 실행하지 않는다.
func WrapResilience(next gin.HandlerFunc, options ResilienceOptions) gin.HandlerFunc {
	policies := make([]resilience.Policy[struct{}], 0, len(options.Policies))
	for _, policy := range options.Policies {
		if isNilInterface(policy) {
			continue
		}
		policies = append(policies, policy)
	}

	return func(c *gin.Context) {
		if c == nil {
			return
		}
		if next == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		original := c.Request
		defer func() { c.Request = original }()
		state := newResilienceAttemptState(c)
		ctx := context.Background()
		if original != nil {
			ctx = original.Context()
		}
		_, err := resilience.Run(ctx, func(policyCtx context.Context) (struct{}, error) {
			return runResilienceAttempt(c, original, state, policyCtx, next)
		}, policies...)
		if err == nil {
			return
		}

		recordResilienceError(c, err)
		if c.Writer.Written() {
			return
		}
		c.Abort()
		if options.ErrorHandler != nil {
			options.ErrorHandler(c, err)
			return
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			_ = AbortWithProblem(c, err)
			return
		}
		_ = AbortWithProblem(c, resilienceProblemError{})
	}
}

type resilienceAttemptState struct {
	request    *http.Request
	keys       map[any]any
	errors     []*gin.Error
	params     gin.Params
	header     http.Header
	writerCode int
}

func newResilienceAttemptState(c *gin.Context) resilienceAttemptState {
	state := resilienceAttemptState{
		request:    c.Request,
		errors:     append([]*gin.Error(nil), c.Errors...),
		params:     append(gin.Params(nil), c.Params...),
		header:     cloneHeader(c.Writer.Header()),
		writerCode: c.Writer.Status(),
	}
	if c.Keys != nil {
		state.keys = make(map[any]any, len(c.Keys))
		for key, value := range c.Keys {
			state.keys[key] = value
		}
	}
	return state
}

func runResilienceAttempt(
	c *gin.Context,
	original *http.Request,
	state resilienceAttemptState,
	policyCtx context.Context,
	next gin.HandlerFunc,
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
	c.Request = attemptRequest
	defer func() {
		c.Request = original
		if body != nil {
			_ = body.Close()
		}
	}()

	next(c)
	attemptErr := latestAttemptError(c, len(state.errors))
	if attemptErr == nil {
		attemptErr = policyCtx.Err()
	}
	if attemptErr == nil {
		return zero, nil
	}

	// Gin does not expose its mutable chain index. An aborted attempt therefore
	// cannot be rewound safely and is fail-closed as non-retryable.
	if c.Writer.Written() || c.IsAborted() || (original.Body != nil && original.GetBody == nil && requestMayHaveBody(original)) {
		return zero, resilience.NonRetryable(attemptErr)
	}
	return zero, attemptErr
}

func cloneResilienceRequest(ctx context.Context, original *http.Request) (*http.Request, io.ReadCloser, error) {
	request := original.WithContext(ctx)
	if original.Body == nil {
		return request, nil, nil
	}
	if original.GetBody != nil {
		body, err := original.GetBody()
		if err != nil {
			return nil, nil, err
		}
		request.Body = body
		return request, body, nil
	}
	if requestMayHaveBody(original) {
		request.Body = original.Body
	} else {
		request.Body = http.NoBody
	}
	return request, nil, nil
}

func requestMayHaveBody(request *http.Request) bool {
	return request != nil && request.Body != nil && request.ContentLength != 0
}

func latestAttemptError(c *gin.Context, baseline int) error {
	if len(c.Errors) <= baseline {
		return nil
	}
	last := c.Errors.Last()
	if last == nil {
		return nil
	}
	return last.Err
}

func restoreResilienceAttemptState(c *gin.Context, state resilienceAttemptState) {
	if !c.Writer.Written() {
		restoreHeader(c.Writer.Header(), state.header)
		c.Writer.WriteHeader(state.writerCode)
	}
	c.Request = state.request
	c.Params = append(c.Params[:0], state.params...)
	c.Errors = append(c.Errors[:0], state.errors...)
	if state.keys == nil {
		c.Keys = nil
		return
	}
	c.Keys = make(map[any]any, len(state.keys))
	for key, value := range state.keys {
		c.Keys[key] = value
	}
}

func recordResilienceError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	last := c.Errors.Last()
	if last != nil && errors.Is(last.Err, err) {
		return
	}
	c.Error(err)
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

type resilienceProblemError struct{}

func (resilienceProblemError) Error() string {
	return "resilience policy rejected request"
}

func (resilienceProblemError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusServiceUnavailable),
		Status: http.StatusServiceUnavailable,
		Detail: "resilience policy rejected request",
	}
}

type resilienceInvalidRequestError struct{}

func (resilienceInvalidRequestError) Error() string {
	return "invalid resilience request"
}

func (resilienceInvalidRequestError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Detail: "invalid resilience request",
	}
}

var (
	_ web.ProblemError = resilienceProblemError{}
	_ web.ProblemError = resilienceInvalidRequestError{}
)
