package ginadapter_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bluetape4k/bluetape-go/jwt"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/gin-gonic/gin"
)

func TestNewJWTStoresReaderAndCallsDownstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parser := &fakeJWTParser{parse: func(string, ...jwt.ParseOption) (*jwt.Reader, error) {
		return new(jwt.Reader), nil
	}}
	middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: parser})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	router := gin.New()
	router.Use(middleware)
	router.GET("/orders", func(c *gin.Context) {
		reader, ok := ginadapter.JWTReader(c, "")
		if !ok || reader == nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if parser.lastToken != "valid-token" {
		t.Fatalf("parser token = %q, want valid-token", parser.lastToken)
	}
}

func TestNewJWTStrictlyRejectsAmbiguousAuthorization(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*http.Request)
		kind   ginadapter.JWTErrorKind
		called bool
	}{
		{name: "missing", setup: func(*http.Request) {}, kind: ginadapter.JWTErrorMissing},
		{name: "scheme", setup: func(r *http.Request) { r.Header.Set("Authorization", "Basic token") }, kind: ginadapter.JWTErrorMalformed},
		{name: "empty", setup: func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") }, kind: ginadapter.JWTErrorMalformed},
		{name: "comma", setup: func(r *http.Request) { r.Header.Set("Authorization", "Bearer a,b") }, kind: ginadapter.JWTErrorMalformed},
		{name: "control", setup: func(r *http.Request) { r.Header.Set("Authorization", "Bearer a\tb") }, kind: ginadapter.JWTErrorMalformed},
		{name: "too long", setup: func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 8193)) }, kind: ginadapter.JWTErrorMalformed},
		{name: "duplicate", setup: func(r *http.Request) { r.Header["Authorization"] = []string{"Bearer a", "Bearer b"} }, kind: ginadapter.JWTErrorMalformed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			parser := &fakeJWTParser{parse: func(string, ...jwt.ParseOption) (*jwt.Reader, error) {
				tt.called = true
				return new(jwt.Reader), nil
			}}
			var got ginadapter.AuthenticationError
			middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{
				Parser: parser,
				ErrorHandler: func(c *gin.Context, err error) {
					if !errors.As(err, &got) {
						t.Errorf("error type = %T, want AuthenticationError", err)
					}
					c.Status(http.StatusUnauthorized)
				},
			})
			if err != nil {
				t.Fatalf("NewJWT() error = %v", err)
			}
			router := gin.New()
			downstreamCalls := 0
			router.Use(middleware)
			router.GET("/orders", func(c *gin.Context) {
				downstreamCalls++
				c.Status(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
			tt.setup(req)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusUnauthorized || downstreamCalls != 0 {
				t.Fatalf("response = (%d, downstream=%d), want (401, 0)", recorder.Code, downstreamCalls)
			}
			if got.Kind != tt.kind {
				t.Fatalf("error kind = %q, want %q", got.Kind, tt.kind)
			}
			if tt.called {
				t.Fatal("parser was called for malformed authorization")
			}
		})
	}
}

func TestNewJWTRedactsParserErrorAndSanitizesCallbackRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const raw = "parser secret token detail"
	parser := &fakeJWTParser{parse: func(string, ...jwt.ParseOption) (*jwt.Reader, error) {
		return nil, errors.New(raw)
	}}
	var (
		callbackErr      error
		callbackAuth     string
		callbackRestored bool
	)
	var original *http.Request
	middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{
		Parser: parser,
		ErrorHandler: func(c *gin.Context, err error) {
			callbackErr = err
			callbackAuth = c.Request.Header.Get("Authorization")
			callbackRestored = c.Request != original
			c.Status(http.StatusUnauthorized)
		},
	})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		original = c.Request
		c.Next()
		if c.Request != original {
			t.Errorf("request pointer after middleware = %p, want %p", c.Request, original)
		}
	})
	router.Use(middleware)
	router.GET("/orders", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
	req.Header.Set("Authorization", "Bearer raw-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if callbackAuth != "" || !callbackRestored {
		t.Fatalf("callback request = (authorization=%q, temporary=%t), want sanitized temporary copy", callbackAuth, callbackRestored)
	}
	if callbackErr == nil || strings.Contains(callbackErr.Error(), raw) {
		t.Fatalf("callback error = %v, contains raw parser detail", callbackErr)
	}
	var authErr ginadapter.AuthenticationError
	if !errors.As(callbackErr, &authErr) || authErr.Kind != ginadapter.JWTErrorInvalid {
		t.Fatalf("callback error = %#v, want invalid AuthenticationError", callbackErr)
	}
}

func TestNewJWTClassifiesExpiredAndContextCancellation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind ginadapter.JWTErrorKind
	}{
		{name: "expired", err: jwt.TokenError{Kind: jwt.ErrExpiredToken, Err: errors.New("expired raw")}, kind: ginadapter.JWTErrorExpired},
		{name: "canceled", err: context.Canceled, kind: ginadapter.JWTErrorCanceled},
		{name: "deadline", err: context.DeadlineExceeded, kind: ginadapter.JWTErrorCanceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			parser := &fakeJWTParser{parse: func(string, ...jwt.ParseOption) (*jwt.Reader, error) {
				return nil, tt.err
			}}
			var got ginadapter.AuthenticationError
			middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{
				Parser: parser,
				ErrorHandler: func(c *gin.Context, err error) {
					if !errors.As(err, &got) {
						t.Errorf("error type = %T, want AuthenticationError", err)
					}
					c.Status(http.StatusUnauthorized)
				},
			})
			if err != nil {
				t.Fatalf("NewJWT() error = %v", err)
			}
			router := gin.New()
			router.Use(middleware)
			router.GET("/orders", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
			req.Header.Set("Authorization", "Bearer token")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if got.Kind != tt.kind {
				t.Fatalf("kind = %q, want %q", got.Kind, tt.kind)
			}
		})
	}
}

func TestNewJWTUsesContextParserAndPropagatesRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const marker = "parser-context"
	var seen atomic.Bool
	parser := contextJWTParserFunc(func(ctx context.Context, token string, options ...jwt.ParseOption) (*jwt.Reader, error) {
		if token != "token" || ctx.Value(marker) != "value" || len(options) != 1 {
			return nil, errors.New("context contract failed")
		}
		seen.Store(true)
		return nil, context.Canceled
	})
	var got ginadapter.AuthenticationError
	middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{
		ContextParser: parser,
		ParseOptions:  []jwt.ParseOption{jwt.WithExpectedIssuer("issuer")},
		ErrorHandler: func(c *gin.Context, err error) {
			_ = errors.As(err, &got)
			c.Status(http.StatusUnauthorized)
		},
	})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), marker, "value"))
		c.Next()
	})
	router.Use(middleware)
	router.GET("/orders", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if !seen.Load() || got.Kind != ginadapter.JWTErrorCanceled {
		t.Fatalf("context parser = (seen=%t, kind=%q), want (true, canceled)", seen.Load(), got.Kind)
	}
}

func TestNewJWTCopiesParseOptionsAndRejectsTypedNilParser(t *testing.T) {
	var typedNil *fakeJWTParser
	for name, options := range map[string]ginadapter.JWTOptions{
		"none":       {},
		"both":       {Parser: &fakeJWTParser{}, ContextParser: contextJWTParserFunc(func(context.Context, string, ...jwt.ParseOption) (*jwt.Reader, error) { return nil, nil })},
		"typed nil":  {Parser: typedNil},
		"nil option": {Parser: &fakeJWTParser{}, ParseOptions: []jwt.ParseOption{nil}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ginadapter.NewJWT(options); err == nil {
				t.Fatal("NewJWT() error = nil, want validation error")
			}
		})
	}

	parseOptions := []jwt.ParseOption{jwt.WithExpectedIssuer("original")}
	parser := &fakeJWTParser{parse: func(_ string, options ...jwt.ParseOption) (*jwt.Reader, error) {
		if len(options) != 1 {
			return nil, errors.New("parse options not copied")
		}
		return new(jwt.Reader), nil
	}}
	middleware, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: parser, ParseOptions: parseOptions})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}
	parseOptions[0] = jwt.WithExpectedIssuer("mutated")
	router := gin.New()
	router.Use(middleware)
	router.GET("/orders", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "http://example.test/orders", nil)
	req.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}

type fakeJWTParser struct {
	parse     func(string, ...jwt.ParseOption) (*jwt.Reader, error)
	lastToken string
}

func (p *fakeJWTParser) Parse(token string, options ...jwt.ParseOption) (*jwt.Reader, error) {
	p.lastToken = token
	if p.parse == nil {
		return new(jwt.Reader), nil
	}
	return p.parse(token, options...)
}

func (p *fakeJWTParser) TryParse(token string, options ...jwt.ParseOption) (*jwt.Reader, bool) {
	reader, err := p.Parse(token, options...)
	return reader, err == nil
}

type contextJWTParserFunc func(context.Context, string, ...jwt.ParseOption) (*jwt.Reader, error)

func (f contextJWTParserFunc) ParseContext(ctx context.Context, token string, options ...jwt.ParseOption) (*jwt.Reader, error) {
	return f(ctx, token, options...)
}

var _ jwt.Parser = (*fakeJWTParser)(nil)
var _ ginadapter.ContextParser = contextJWTParserFunc(nil)
