package resilience

import (
	"context"
	"fmt"
	"net/http"
)

// HTTPStatusPredicate func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type HTTPStatusPredicate func(int) bool

// RetryableServerError RetryableServerError 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - statusCode: HTTP 상태 코드다.
func RetryableServerError(statusCode int) bool {
	return statusCode >= http.StatusInternalServerError && statusCode <= 599
}

// StatusError struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type StatusError struct {
	StatusCode int
	Status     string
}

func (e StatusError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("http status %s", e.Status)
	}
	return fmt.Sprintf("http status %d", e.StatusCode)
}

// RoundTripperOptions struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type RoundTripperOptions struct {
	Transport       http.RoundTripper
	Policies        []Policy[*http.Response]
	RetryableStatus HTTPStatusPredicate
}

// ResilientRoundTripper struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type ResilientRoundTripper struct {
	transport       http.RoundTripper
	policies        []Policy[*http.Response]
	retryableStatus HTTPStatusPredicate
}

// NewRoundTripper NewRoundTripper 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
func NewRoundTripper(options RoundTripperOptions) *ResilientRoundTripper {
	transport := options.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &ResilientRoundTripper{
		transport:       transport,
		policies:        append([]Policy[*http.Response](nil), options.Policies...), //nolint:bodyclose
		retryableStatus: options.RetryableStatus,
	}
}

// RoundTrip RoundTrip 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - req: 처리할 HTTP request다.
//
// 반환 오류는 입력 검증 실패, context 취소/deadline, 상태 전이 실패, 패키지 sentinel error와 typed error를 그대로 드러낸다.
func (t *ResilientRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("request must not be nil")
	}

	ctx := req.Context()
	attemptNumber := 0
	operation := func(ctx context.Context) (*http.Response, error) {
		attemptNumber++
		attempt, err := cloneRequestForAttempt(ctx, req, attemptNumber)
		if err != nil {
			return nil, err
		}

		transport := t.transport
		if transport == nil {
			transport = http.DefaultTransport
		}

		response, err := transport.RoundTrip(attempt)
		if err != nil {
			if response != nil && response.Body != nil {
				_ = response.Body.Close()
			}
			return nil, err
		}

		if response != nil && t.retryableStatus != nil && t.retryableStatus(response.StatusCode) {
			if response.Body != nil {
				_ = response.Body.Close()
			}
			return nil, StatusError{
				StatusCode: response.StatusCode,
				Status:     response.Status,
			}
		}

		return response, nil
	}

	return Run(ctx, operation, t.policies...)
}

func cloneRequestForAttempt(ctx context.Context, req *http.Request, attemptNumber int) (*http.Request, error) {
	attempt := req.Clone(ctx)
	if req.Body == nil {
		return attempt, nil
	}
	if req.GetBody == nil {
		if attemptNumber == 1 {
			attempt.Body = req.Body
			return attempt, nil
		}
		return nil, fmt.Errorf("request body is not replayable: set GetBody before using resilience retry")
	}

	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	attempt.Body = body
	return attempt, nil
}

// HandlerErrorHandler func 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type HandlerErrorHandler func(http.ResponseWriter, *http.Request, error)

// HandlerOptions struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type HandlerOptions struct {
	Policies     []Policy[struct{}]
	ErrorHandler HandlerErrorHandler
}

// ResilientHandler struct 공개 타입이며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
type ResilientHandler struct {
	next         http.Handler
	policies     []Policy[struct{}]
	errorHandler HandlerErrorHandler
}

// NewHandler NewHandler 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - next: 다음 HTTP handler다.
//   - options: 적용할 옵션 목록이다. nil이면 기본값만 사용한다.
func NewHandler(next http.Handler, options HandlerOptions) *ResilientHandler {
	if next == nil {
		next = http.NotFoundHandler()
	}

	errorHandler := options.ErrorHandler
	if errorHandler == nil {
		errorHandler = defaultHandlerError
	}

	return &ResilientHandler{
		next:         next,
		policies:     append([]Policy[struct{}](nil), options.Policies...),
		errorHandler: errorHandler,
	}
}

// ServeHTTP ServeHTTP 공개 API의 동작을 수행하며 취소, deadline, retry, timeout, circuit breaker 상태를 보존한다.
//
// 매개변수:
//   - w: 응답을 기록할 HTTP writer다.
//   - req: 처리할 HTTP request다.
func (h *ResilientHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	operation := func(ctx context.Context) (struct{}, error) {
		h.next.ServeHTTP(w, req.WithContext(ctx))
		if err := ctx.Err(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, nil
	}

	if _, err := Run(req.Context(), operation, h.policies...); err != nil {
		h.errorHandler(w, req, err)
	}
}

func defaultHandlerError(w http.ResponseWriter, _ *http.Request, err error) {
	http.Error(w, err.Error(), http.StatusServiceUnavailable)
}
