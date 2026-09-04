package echoadapter_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/jwt"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestJWTLegacyParserUsesContextAwarePathWhenAvailable(t *testing.T) {
	marker := parserContextKey{}
	requestContext := context.WithValue(context.Background(), marker, "request")
	request := httptest.NewRequestWithContext(requestContext, http.MethodGet, "http://example.test/orders", nil)
	request.Header.Set("Authorization", "Bearer token")
	ctx, recorder := newEchoContextFromRequest(request)
	parser := &contextAwareLegacyParser{}

	middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: parser})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	nextCalls := 0
	if err := middleware(func(echo.Context) error {
		nextCalls++
		return ctx.NoContent(http.StatusNoContent)
	})(ctx); err != nil {
		t.Fatalf("middleware() error = %v", err)
	}

	if parser.contextCalls.Load() != 1 || parser.parseCalls.Load() != 0 {
		t.Fatalf("context/legacy calls = %d/%d, want 1/0", parser.contextCalls.Load(), parser.parseCalls.Load())
	}
	if parser.contextValue != "request" {
		t.Fatalf("context value = %q, want request", parser.contextValue)
	}
	if recorder.Code != http.StatusNoContent || nextCalls != 1 {
		t.Fatalf("status/next calls = %d/%d, want 204/1", recorder.Code, nextCalls)
	}
}

func TestJWTLegacyParserAutoUpgradePropagatesCancellation(t *testing.T) {
	started := make(chan struct{})
	parser := &cancelAwareLegacyParser{started: started}
	observedKind := echoadapter.JWTErrorKind("")
	middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{
		Parser: parser,
		ErrorHandler: func(ctx echo.Context, err error) {
			var authErr echoadapter.AuthenticationError
			if errors.As(err, &authErr) {
				observedKind = authErr.Kind
			}
			_ = ctx.NoContent(http.StatusUnauthorized)
		},
	})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	requestContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequestWithContext(requestContext, http.MethodGet, "http://example.test/orders", nil)
	request.Header.Set("Authorization", "Bearer token")
	ctx, recorder := newEchoContextFromRequest(request)
	nextCalls := atomic.Int32{}
	done := make(chan error, 1)
	go func() {
		done <- middleware(func(echo.Context) error {
			nextCalls.Add(1)
			return nil
		})(ctx)
	}()

	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("auto-upgraded context parser did not start")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("middleware() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("auto-upgraded context parser did not return after cancellation")
	}

	if parser.contextCalls.Load() != 1 || parser.parseCalls.Load() != 0 {
		t.Fatalf("context/legacy calls = %d/%d, want 1/0", parser.contextCalls.Load(), parser.parseCalls.Load())
	}
	if observedKind != echoadapter.JWTErrorCanceled {
		t.Fatalf("authentication error kind = %q, want %q", observedKind, echoadapter.JWTErrorCanceled)
	}
	if recorder.Code != http.StatusUnauthorized || nextCalls.Load() != 0 {
		t.Fatalf("status/next calls = %d/%d, want 401/0", recorder.Code, nextCalls.Load())
	}
}

func TestJWTLegacyParserSkipsPreCanceledRequest(t *testing.T) {
	requestContext, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequestWithContext(requestContext, http.MethodGet, "http://example.test/orders", nil)
	request.Header.Set("Authorization", "Bearer token")
	ctx, recorder := newEchoContextFromRequest(request)
	parser := &blockingLegacyParser{}

	middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: parser})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	nextCalls := 0
	if err := middleware(func(echo.Context) error {
		nextCalls++
		return nil
	})(ctx); err != nil {
		t.Fatalf("middleware() error = %v", err)
	}

	if parser.calls.Load() != 0 {
		t.Fatalf("parser calls = %d, want 0", parser.calls.Load())
	}
	if recorder.Code != http.StatusUnauthorized || nextCalls != 0 {
		t.Fatalf("status/next calls = %d/%d, want 401/0", recorder.Code, nextCalls)
	}
}

func TestJWTLegacyParserLateCancellationWaitsForProviderAndDoesNotDetach(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	parser := &blockingLegacyParser{
		started: started,
		release: release,
		reader:  &jwt.Reader{},
	}
	middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{
		Parser: parser,
		ErrorHandler: func(ctx echo.Context, err error) {
			var authErr echoadapter.AuthenticationError
			if !errors.As(err, &authErr) || authErr.Kind != echoadapter.JWTErrorCanceled {
				t.Errorf("authentication error = %v, want canceled", err)
			}
			_ = ctx.NoContent(http.StatusUnauthorized)
		},
	})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	requestContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequestWithContext(requestContext, http.MethodGet, "http://example.test/orders", nil)
	request.Header.Set("Authorization", "Bearer token")
	ctx, recorder := newEchoContextFromRequest(request)
	nextCalls := atomic.Int32{}
	done := make(chan error, 1)
	go func() {
		done <- middleware(func(echo.Context) error {
			nextCalls.Add(1)
			return nil
		})(ctx)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("legacy parser did not start")
	}
	cancel()
	select {
	case err := <-done:
		t.Fatalf("middleware returned while legacy parser was blocked: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if parser.active.Load() != 1 {
		t.Fatalf("active parser calls = %d, want 1 before release", parser.active.Load())
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("middleware() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("middleware did not join the released legacy parser")
	}

	if parser.calls.Load() != 1 || parser.active.Load() != 0 {
		t.Fatalf("parser calls/active = %d/%d, want 1/0", parser.calls.Load(), parser.active.Load())
	}
	if recorder.Code != http.StatusUnauthorized || nextCalls.Load() != 0 {
		t.Fatalf("status/next calls = %d/%d, want 401/0", recorder.Code, nextCalls.Load())
	}
}

type contextAwareLegacyParser struct {
	parseCalls   atomic.Int32
	contextCalls atomic.Int32
	contextValue string
}

func (p *contextAwareLegacyParser) Parse(string, ...jwt.ParseOption) (*jwt.Reader, error) {
	p.parseCalls.Add(1)
	return nil, errors.New("legacy parser path used")
}

func (p *contextAwareLegacyParser) TryParse(token string, options ...jwt.ParseOption) (*jwt.Reader, bool) {
	reader, err := p.Parse(token, options...)
	return reader, err == nil
}

func (p *contextAwareLegacyParser) ParseContext(ctx context.Context, _ string, _ ...jwt.ParseOption) (*jwt.Reader, error) {
	p.contextCalls.Add(1)
	p.contextValue, _ = ctx.Value(parserContextKey{}).(string)
	return &jwt.Reader{}, nil
}

type cancelAwareLegacyParser struct {
	parseCalls   atomic.Int32
	contextCalls atomic.Int32
	started      chan<- struct{}
}

func (p *cancelAwareLegacyParser) Parse(string, ...jwt.ParseOption) (*jwt.Reader, error) {
	p.parseCalls.Add(1)
	return nil, errors.New("legacy parser path used")
}

func (p *cancelAwareLegacyParser) TryParse(token string, options ...jwt.ParseOption) (*jwt.Reader, bool) {
	reader, err := p.Parse(token, options...)
	return reader, err == nil
}

func (p *cancelAwareLegacyParser) ParseContext(ctx context.Context, _ string, _ ...jwt.ParseOption) (*jwt.Reader, error) {
	p.contextCalls.Add(1)
	close(p.started)
	<-ctx.Done()
	return &jwt.Reader{}, nil
}

type blockingLegacyParser struct {
	calls   atomic.Int32
	active  atomic.Int32
	started chan struct{}
	release <-chan struct{}
	reader  *jwt.Reader
}

func (p *blockingLegacyParser) Parse(string, ...jwt.ParseOption) (*jwt.Reader, error) {
	p.calls.Add(1)
	p.active.Add(1)
	defer p.active.Add(-1)
	if p.started != nil {
		close(p.started)
	}
	if p.release != nil {
		<-p.release
	}
	return p.reader, nil
}

func (p *blockingLegacyParser) TryParse(token string, options ...jwt.ParseOption) (*jwt.Reader, bool) {
	reader, err := p.Parse(token, options...)
	return reader, err == nil
}
