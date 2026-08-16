package echoadapter_test

import (
	"net/http"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/web"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func Example() {
	provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		return
	}
	limiter, err := ratelimit.New(ratelimit.Options{RatePerSecond: 10, Burst: 10})
	if err != nil {
		return
	}
	rateLimit, err := echoadapter.NewRateLimit(echoadapter.RateLimitOptions{Limiter: limiter})
	if err != nil {
		return
	}
	authentication, err := echoadapter.NewJWT(echoadapter.JWTOptions{Parser: provider})
	if err != nil {
		return
	}

	server := echo.New()
	server.Use(echoadapter.RequestContext(web.RequestContextOptions{}), rateLimit, authentication)
	server.GET("/orders", echoadapter.WrapResilience(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	}, echoadapter.ResilienceOptions{}))
	_ = server
	// Output:
}

func Example_migration() {
	legacy := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	server := echo.New()
	server.GET("/legacy", echo.WrapHandler(legacy))
	server.GET("/orders", echoadapter.WrapResilience(func(c echo.Context) error {
		return legacyServe(c, legacy)
	}, echoadapter.ResilienceOptions{}))
	_ = server
	// Output:
}

func legacyServe(c echo.Context, handler http.Handler) error {
	handler.ServeHTTP(c.Response(), c.Request())
	return nil
}
