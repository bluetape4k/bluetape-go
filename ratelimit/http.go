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

// KeyFunc는 func 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type KeyFunc func(*http.Request) string

// HandlerErrorHandler는 func 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type HandlerErrorHandler func(http.ResponseWriter, *http.Request, Result, error)

// HandlerOptions는 struct 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type HandlerOptions struct {
	Limiter      Limiter
	KeyFunc      KeyFunc
	Tokens       int64
	ErrorHandler HandlerErrorHandler
}

// Handler는 struct 공개 타입이며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Handler struct {
	next         http.Handler
	limiter      Limiter
	keyFunc      KeyFunc
	tokens       int64
	errorHandler HandlerErrorHandler
}

// NewHandler는 NewHandler 공개 API의 동작을 수행하며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
//
// 매개변수:
//   - next: NewHandler 동작에 필요한 next 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - options: NewHandler 동작에 필요한 options 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, 취소, Redis/backend 실패, lock ownership 불일치, quota 거절, 또는 package sentinel/typed error 계약을 보존한다.
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

// ServeHTTP는 ServeHTTP 공개 API의 동작을 수행하며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
//
// 매개변수:
//   - w: ServeHTTP 동작에 필요한 w 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//   - req: ServeHTTP 동작에 필요한 req 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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

// RemoteIPKey는 RemoteIPKey 공개 API의 동작을 수행하며 token bucket, limiter option, HTTP boundary, result quota 계약을 보존한다.
//
// 매개변수:
//   - req: RemoteIPKey 동작에 필요한 req 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
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
