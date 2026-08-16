package ginadapter_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/web"
	ginadapter "github.com/bluetape4k/bluetape-go/web/gin"
	"github.com/gin-gonic/gin"
)

// Bootstrap and Migration anchor the runnable examples without adding
// production API surface.
type Bootstrap struct{}
type Migration struct{}

func ExampleBootstrap() {
	router, _, err := exampleRouter()
	if err != nil {
		panic(err)
	}

	fmt.Println(router != nil)

	// Output:
	// true
}

func ExampleMigration() {
	legacy := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	legacyRouter := gin.New()
	legacyRouter.GET("/legacy", gin.WrapH(legacy))
	legacyRequest := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/legacy", nil)
	legacyRecorder := httptest.NewRecorder()
	legacyRouter.ServeHTTP(legacyRecorder, legacyRequest)

	router, token, err := exampleRouter()
	if err != nil {
		panic(err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	fmt.Println(legacyRecorder.Code, recorder.Code)

	// Output:
	// 204 204
}

func exampleRouter() (*gin.Engine, string, error) {
	gin.SetMode(gin.TestMode)
	provider, err := jwt.NewFixedHMACProvider(jwt.HS256, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		return nil, "", err
	}
	token, err := provider.Compose(jwt.WithSubject("account-42"))
	if err != nil {
		return nil, "", err
	}
	limiter, err := ratelimit.New(ratelimit.Options{RatePerSecond: 10, Burst: 10})
	if err != nil {
		return nil, "", err
	}
	requestContext := ginadapter.RequestContext(web.RequestContextOptions{})
	rateLimit, err := ginadapter.NewRateLimit(ginadapter.RateLimitOptions{Limiter: limiter})
	if err != nil {
		return nil, "", err
	}
	authentication, err := ginadapter.NewJWT(ginadapter.JWTOptions{Parser: provider})
	if err != nil {
		return nil, "", err
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestContext, rateLimit, authentication)
	router.GET("/orders", ginadapter.WrapResilience(func(c *gin.Context) {
		if _, ok := ginadapter.JWTReader(c, ""); !ok {
			_ = ginadapter.AbortWithProblem(c, ginadapter.AuthenticationError{Kind: ginadapter.JWTErrorInvalid})
			return
		}
		c.Status(http.StatusNoContent)
	}, ginadapter.ResilienceOptions{}))
	return router, token, nil
}
