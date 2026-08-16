package echoadapter_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

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
