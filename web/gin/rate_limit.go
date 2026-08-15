package ginadapter

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
	"github.com/gin-gonic/gin"
)

// NewRateLimit bridges the common ratelimit HTTP middleware to Gin middleware.
// 공통 ratelimit HTTP middleware를 Gin middleware로 연결한다.
//
// 허용된 요청만 현재 Gin chain의 다음 handler로 진행하며, 거부와 backend
// 오류는 요청별 context를 사용해 서로 다른 Gin 요청에 섞이지 않도록 한다.
func NewRateLimit(options RateLimitOptions) (gin.HandlerFunc, error) {
	if isNilInterface(options.Limiter) {
		return nil, fmt.Errorf("limiter must not be nil")
	}
	if options.Tokens < 0 {
		return nil, fmt.Errorf("tokens must be positive")
	}

	core, err := ratelimit.NewHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		contextValue, _ := request.Context().Value(ginContextKey{}).(*gin.Context)
		if contextValue == nil {
			http.NotFoundHandler().ServeHTTP(writer, request)
			return
		}
		contextValue.Next()
	}), ratelimit.HandlerOptions{
		Limiter: options.Limiter,
		Tokens:  options.Tokens,
		KeyFunc: func(request *http.Request) string {
			contextValue, _ := request.Context().Value(ginContextKey{}).(*gin.Context)
			if contextValue == nil || options.KeyFunc == nil {
				return ratelimit.RemoteIPKey(request)
			}
			return options.KeyFunc(contextValue)
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, result ratelimit.Result, requestErr error) {
			contextValue, _ := request.Context().Value(ginContextKey{}).(*gin.Context)
			handleRateLimitError(contextValue, writer, request, result, requestErr, options.ErrorHandler)
		},
	})
	if err != nil {
		return nil, err
	}

	return func(c *gin.Context) {
		if c == nil {
			return
		}
		original := c.Request
		defer func() { c.Request = original }()
		if original == nil {
			_ = AbortWithProblem(c, rateLimitInvalidRequestError{})
			return
		}

		request := original.WithContext(context.WithValue(original.Context(), ginContextKey{}, c))
		c.Request = request
		core.ServeHTTP(c.Writer, request)
	}, nil
}

type ginContextKey struct{}

func handleRateLimitError(
	c *gin.Context,
	writer http.ResponseWriter,
	request *http.Request,
	result ratelimit.Result,
	requestErr error,
	custom func(*gin.Context, ratelimit.Result, error),
) {
	if c != nil {
		c.Abort()
	}
	if custom != nil && c != nil {
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

func (rateLimitRejectedError) Error() string {
	return "rate limit exceeded"
}

func (rateLimitRejectedError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusTooManyRequests),
		Status: http.StatusTooManyRequests,
		Detail: "rate limit exceeded",
	}
}

type rateLimitBackendProblemError struct{}

func (rateLimitBackendProblemError) Error() string {
	return "rate limit backend unavailable"
}

func (rateLimitBackendProblemError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusServiceUnavailable),
		Status: http.StatusServiceUnavailable,
		Detail: "rate limit backend unavailable",
	}
}

type rateLimitInvalidRequestError struct{}

func (rateLimitInvalidRequestError) Error() string {
	return "invalid rate limit request"
}

func (rateLimitInvalidRequestError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Detail: "invalid rate limit request",
	}
}

var (
	_ web.ProblemError = rateLimitRejectedError{}
	_ web.ProblemError = rateLimitBackendProblemError{}
	_ web.ProblemError = rateLimitInvalidRequestError{}
)
