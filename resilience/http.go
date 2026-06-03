package resilience

import (
	"context"
	"fmt"
	"net/http"
)

// HTTPStatusPredicate decides whether an HTTP status should be treated as a
// retryable transport error by ResilientRoundTripper.
type HTTPStatusPredicate func(int) bool

// RetryableServerError returns true for 5xx status codes.
func RetryableServerError(statusCode int) bool {
	return statusCode >= http.StatusInternalServerError && statusCode <= 599
}

// StatusError reports an HTTP response status that a RoundTripper converted
// into an error so retry and circuit breaker policies can observe it.
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

// RoundTripperOptions configures a resilient HTTP client transport.
type RoundTripperOptions struct {
	Transport       http.RoundTripper
	Policies        []Policy[*http.Response]
	RetryableStatus HTTPStatusPredicate
}

// ResilientRoundTripper applies resilience policies to each HTTP request.
type ResilientRoundTripper struct {
	transport       http.RoundTripper
	policies        []Policy[*http.Response]
	retryableStatus HTTPStatusPredicate
}

// NewRoundTripper creates a RoundTripper that runs requests through policies.
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

// RoundTrip executes req through the configured transport and policies.
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

// HandlerErrorHandler writes policy errors produced by ResilientHandler.
type HandlerErrorHandler func(http.ResponseWriter, *http.Request, error)

// HandlerOptions configures a resilient HTTP server handler.
type HandlerOptions struct {
	Policies     []Policy[struct{}]
	ErrorHandler HandlerErrorHandler
}

// ResilientHandler applies resilience policies before running a server handler.
type ResilientHandler struct {
	next         http.Handler
	policies     []Policy[struct{}]
	errorHandler HandlerErrorHandler
}

// NewHandler creates a handler that runs next through policies.
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

// ServeHTTP applies policies and delegates to the wrapped handler.
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
