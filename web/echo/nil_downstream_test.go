package echoadapter_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestEchoMiddlewareNilDownstreamReturnsNotFound(t *testing.T) {
	parser := &nilDownstreamJWTParser{}
	rateLimit, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{
		Limiter: &fakeLimiter{allow: func(context.Context, string, int64) (ratelimit.Result, error) {
			t.Fatal("nil downstream must not call the limiter")
			return ratelimit.Result{}, nil
		}},
	})
	if err != nil {
		t.Fatalf("NewRateLimit() error = %v", err)
	}
	jwtMiddleware, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: parser})
	if err != nil {
		t.Fatalf("NewJWT() error = %v", err)
	}

	cases := []struct {
		name string
		run  func(echo.Context) error
	}{
		{
			name: "request context",
			run:  echoadapter.RequestContext(web.RequestContextOptions{})(nil),
		},
		{
			name: "jwt",
			run:  jwtMiddleware(nil),
		},
		{
			name: "rate limit",
			run:  rateLimit(nil),
		},
		{
			name: "resilience",
			run: echoadapter.WrapResilience(nil, echoadapter.ResilienceOptions{
				Policies: []resilience.Policy[struct{}]{resilience.PolicyFunc[struct{}](func(resilience.Operation[struct{}]) resilience.Operation[struct{}] {
					return func(context.Context) (struct{}, error) {
						t.Fatal("nil downstream must not call resilience policies")
						return struct{}{}, nil
					}
				})},
			}),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
			ctx.Request().Header.Set("Authorization", "Bearer valid-token")
			if err := tt.run(ctx); err != nil {
				t.Fatalf("middleware() error = %v", err)
			}
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
			}
			if !ctx.Response().Committed {
				t.Fatal("nil downstream response was not committed")
			}
		})
	}
	if parser.called {
		t.Fatal("nil downstream must not call the JWT parser")
	}
}

type nilDownstreamJWTParser struct {
	called bool
}

func (p *nilDownstreamJWTParser) Parse(string, ...jwt.ParseOption) (*jwt.Reader, error) {
	p.called = true
	return &jwt.Reader{}, nil
}

func (p *nilDownstreamJWTParser) TryParse(token string, options ...jwt.ParseOption) (*jwt.Reader, bool) {
	reader, err := p.Parse(token, options...)
	return reader, err == nil
}
