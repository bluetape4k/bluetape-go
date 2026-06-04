package ratelimit

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// KeyFunc 는 HTTP request에서 rate limit key를 만든다.
type KeyFunc func(*http.Request) string

// HandlerErrorHandler 는 middleware 거부 또는 backend 오류를 쓴다.
type HandlerErrorHandler func(http.ResponseWriter, *http.Request, Result, error)

// HandlerOptions 는 HTTP middleware 설정이다.
type HandlerOptions struct {
	Limiter      Limiter
	KeyFunc      KeyFunc
	Tokens       int64
	ErrorHandler HandlerErrorHandler
}

// Handler 는 HTTP server rate limit middleware다.
type Handler struct {
	next         http.Handler
	limiter      Limiter
	keyFunc      KeyFunc
	tokens       int64
	errorHandler HandlerErrorHandler
}

// NewHandler 는 HTTP rate limit middleware를 만든다.
func NewHandler(next http.Handler, options HandlerOptions) (*Handler, error) {
	if options.Limiter == nil {
		return nil, fmt.Errorf("limiter must not be nil")
	}
	tokens := options.Tokens
	if tokens == 0 {
		tokens = 1
	}
	if tokens < 0 {
		return nil, fmt.Errorf("tokens must be positive")
	}
	if next == nil {
		next = http.NotFoundHandler()
	}
	keyFunc := options.KeyFunc
	if keyFunc == nil {
		keyFunc = RemoteIPKey
	}
	errorHandler := options.ErrorHandler
	if errorHandler == nil {
		errorHandler = defaultHandlerError
	}
	return &Handler{
		next:         next,
		limiter:      options.Limiter,
		keyFunc:      keyFunc,
		tokens:       tokens,
		errorHandler: errorHandler,
	}, nil
}

// ServeHTTP 는 요청별 token 소비 후 다음 handler를 실행한다.
func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	key := strings.TrimSpace(h.keyFunc(req))
	if key == "" {
		h.errorHandler(w, req, Result{Requested: h.tokens}, fmt.Errorf("rate limit key must not be empty"))
		return
	}

	result, err := h.limiter.Allow(req.Context(), key, h.tokens)
	if err != nil {
		h.errorHandler(w, req, result, err)
		return
	}
	if !result.Allowed {
		h.errorHandler(w, req, result, nil)
		return
	}
	h.next.ServeHTTP(w, req)
}

// RemoteIPKey 는 Request.RemoteAddr의 host 부분만 key로 쓴다.
func RemoteIPKey(req *http.Request) string {
	if req == nil {
		return ""
	}
	remote := strings.TrimSpace(req.RemoteAddr)
	if remote == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return remote
	}
	return host
}

func defaultHandlerError(w http.ResponseWriter, _ *http.Request, result Result, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if result.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds(result.RetryAfter), 10))
	}
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
	http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
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
