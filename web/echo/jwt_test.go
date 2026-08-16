package echoadapter_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/jwt"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestNewJWTRejectsTypedNilParsers(t *testing.T) {
	var parser *fakeJWTParser
	if _, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: parser}); err == nil {
		t.Fatal("NewJWT() accepted typed-nil parser")
	}
	var contextParser *fakeContextParser
	if _, err := echoadapter.NewJWT(echoadapter.JWTOptions{ContextParser: contextParser}); err == nil {
		t.Fatal("NewJWT() accepted typed-nil context parser")
	}
}

func TestJWTRejectsStrictHeaderForms(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		duplicate bool
	}{
		{name: "duplicate", value: "Bearer one", duplicate: true},
		{name: "comma", value: "Bearer one,two"},
		{name: "leading whitespace", value: " Bearer token"},
		{name: "inner whitespace", value: "Bearer token value"},
		{name: "control", value: "Bearer to\tken"},
		{name: "oversized", value: "Bearer " + strings.Repeat("a", 8*1024+1)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
			ctx.Request().Header.Set("Authorization", tt.value)
			if tt.duplicate {
				ctx.Request().Header.Add("Authorization", "Bearer two")
			}
			middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: &fakeJWTParser{}})
			if err != nil {
				t.Fatalf("NewJWT() error = %v", err)
			}
			nextCalls := 0
			if err := middleware(func(next echo.Context) error {
				nextCalls++
				return next.NoContent(http.StatusNoContent)
			})(ctx); err != nil {
				t.Fatalf("middleware() error = %v", err)
			}
			if recorder.Code != http.StatusUnauthorized || nextCalls != 0 {
				t.Fatalf("status=%d next=%d, want 401/0", recorder.Code, nextCalls)
			}
		})
	}
}

func TestJWTReaderRejectsNilReader(t *testing.T) {
	ctx, _ := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	ctx.Set(echoadapter.DefaultJWTContextKey, (*jwt.Reader)(nil))
	if _, ok := echoadapter.JWTReader(ctx, ""); ok {
		t.Fatal("JWTReader() accepted typed-nil reader")
	}
}

type fakeContextParser struct{}

func (*fakeContextParser) ParseContext(_ context.Context, _ string, _ ...jwt.ParseOption) (*jwt.Reader, error) {
	return nil, errors.New("unused")
}

func TestJWTContextParserReceivesRequestContext(t *testing.T) {
	marker := parserContextKey{}
	requestContext := context.WithValue(context.Background(), marker, "value")
	request := httptest.NewRequestWithContext(requestContext, http.MethodGet, "http://example.test/orders", nil)
	request.Header.Set("Authorization", "Bearer token")
	ctx, recorder := newEchoContextFromRequest(request)
	seen := false
	middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{
		ContextParser: contextJWTParserFunc(func(ctx context.Context, token string, options ...jwt.ParseOption) (*jwt.Reader, error) {
			if token != "token" || ctx.Value(marker) != "value" || len(options) != 1 {
				return nil, errors.New("context contract failed")
			}
			seen = true
			return nil, context.Canceled
		}),
		ParseOptions: []jwt.ParseOption{jwt.WithExpectedIssuer("issuer")},
	})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	if err := middleware(func(echo.Context) error { return nil })(ctx); err != nil {
		t.Fatalf("middleware() error = %v", err)
	}
	if !seen || recorder.Code != http.StatusUnauthorized {
		t.Fatalf("parser seen/status = %t/%d, want true/401", seen, recorder.Code)
	}
}

func TestJWTContextParserCancellationReturnsPromptly(t *testing.T) {
	started := make(chan struct{})
	parser := contextJWTParserFunc(func(ctx context.Context, _ string, _ ...jwt.ParseOption) (*jwt.Reader, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	middleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{
		ContextParser: parser,
		ErrorHandler:  func(ctx echo.Context, _ error) { _ = ctx.NoContent(http.StatusUnauthorized) },
	})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	requestContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequestWithContext(requestContext, http.MethodGet, "http://example.test/orders", nil)
	request.Header.Set("Authorization", "Bearer token")
	ctx, recorder := newEchoContextFromRequest(request)
	done := make(chan struct{})
	go func() {
		_ = middleware(func(echo.Context) error { return nil })(ctx)
		close(done)
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("context parser did not start")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("JWT middleware did not return after cancellation")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

type contextJWTParserFunc func(context.Context, string, ...jwt.ParseOption) (*jwt.Reader, error)

type parserContextKey struct{}

func (fn contextJWTParserFunc) ParseContext(ctx context.Context, token string, options ...jwt.ParseOption) (*jwt.Reader, error) {
	return fn(ctx, token, options...)
}
