package webtest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluetape4k/bluetape-go/webtest"
)

func ExampleScenario() {
	scenario := webtest.Scenario{
		Name:    "example",
		Adapter: func(next http.Handler) http.Handler { return next },
		NewRequest: func(ctx context.Context) *http.Request {
			return httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.test/", nil)
		},
		Next:   http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		Assert: func(*testing.T, webtest.Observation) {},
	}
	_ = scenario
	// Output:
}
