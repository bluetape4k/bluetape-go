package echoadapter

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/labstack/echo/v4"
)

// NewRateLimit bridges the common ratelimit HTTP middleware to Echo.
// 공통 ratelimit HTTP middleware를 Echo middleware로 연결한다.
func NewRateLimit(options RateLimitOptions) (echo.MiddlewareFunc, error) {
	if isNilInterface(options.Limiter) {
		return nil, fmt.Errorf("limiter must not be nil")
	}
	if options.Tokens < 0 {
		return nil, fmt.Errorf("tokens must be positive")
	}

	core, err := ratelimit.NewHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		state, _ := request.Context().Value(echoContextKey{}).(*echoRequestState)
		if state == nil || isNilInterface(state.context) {
			http.NotFoundHandler().ServeHTTP(writer, request)
			return
		}
		if state.next == nil {
			return
		}
		state.nextErr = state.next(state.context)
	}), ratelimit.HandlerOptions{
		Limiter: options.Limiter,
		Tokens:  options.Tokens,
		KeyFunc: func(request *http.Request) string {
			state, _ := request.Context().Value(echoContextKey{}).(*echoRequestState)
			if state == nil || isNilInterface(state.context) || options.KeyFunc == nil {
				return ratelimit.RemoteIPKey(request)
			}
			return options.KeyFunc(state.context)
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, result ratelimit.Result, requestErr error) {
			state, _ := request.Context().Value(echoContextKey{}).(*echoRequestState)
			if state == nil {
				return
			}
			handleRateLimitError(state.context, writer, request, result, requestErr, options.ErrorHandler)
			state.handled = true
		},
	})
	if err != nil {
		return nil, err
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if isNilInterface(c) {
				return nil
			}
			if next == nil {
				return c.NoContent(http.StatusNotFound)
			}
			original := c.Request()
			if original == nil {
				return AbortWithProblem(c, rateLimitInvalidRequestError{})
			}
			defer c.SetRequest(original)
			state := &echoRequestState{context: c, next: next}
			request := original.WithContext(context.WithValue(original.Context(), echoContextKey{}, state))
			c.SetRequest(request)
			core.ServeHTTP(c.Response(), request)
			if state.nextErr != nil && !c.Response().Committed && !state.handled {
				return state.nextErr
			}
			return nil
		}
	}, nil
}

type echoContextKey struct{}

type echoRequestState struct {
	context echo.Context
	next    echo.HandlerFunc
	nextErr error
	handled bool
}

func handleRateLimitError(
	c echo.Context,
	writer http.ResponseWriter,
	request *http.Request,
	result ratelimit.Result,
	requestErr error,
	custom func(echo.Context, ratelimit.Result, error),
) {
	if custom != nil && !isNilInterface(c) {
		custom(c, result, requestErr)
		return
	}

	if requestErr != nil {
		if errors.Is(requestErr, context.Canceled) || errors.Is(requestErr, context.DeadlineExceeded) {
			_ = web.WriteProblem(writer, request, requestErr)
			return
		}
		_ = web.WriteProblem(writer, request, rateLimitBackendProblemError{})
		return
	}

	if result.RetryAfter > 0 {
		writer.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds(result.RetryAfter), 10))
	}
	writer.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
	_ = web.WriteProblem(writer, request, rateLimitRejectedError{})
}

func retryAfterSeconds(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	seconds := int64(math.Ceil(duration.Seconds()))
	if seconds <= 0 {
		return 1
	}
	return seconds
}

type rateLimitRejectedError struct{}

func (rateLimitRejectedError) Error() string { return "rate limit exceeded" }

func (rateLimitRejectedError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: http.StatusText(http.StatusTooManyRequests), Status: http.StatusTooManyRequests, Detail: "rate limit exceeded"}
}

type rateLimitBackendProblemError struct{}

func (rateLimitBackendProblemError) Error() string { return "rate limit backend unavailable" }

func (rateLimitBackendProblemError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: http.StatusText(http.StatusServiceUnavailable), Status: http.StatusServiceUnavailable, Detail: "rate limit backend unavailable"}
}

type rateLimitInvalidRequestError struct{}

func (rateLimitInvalidRequestError) Error() string { return "invalid rate limit request" }

func (rateLimitInvalidRequestError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: http.StatusText(http.StatusInternalServerError), Status: http.StatusInternalServerError, Detail: "invalid rate limit request"}
}

var (
	_ web.ProblemError = rateLimitRejectedError{}
	_ web.ProblemError = rateLimitBackendProblemError{}
	_ web.ProblemError = rateLimitInvalidRequestError{}
)
