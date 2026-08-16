package echoadapter_test

import (
	"net/http"
	"testing"

	"github.com/bluetape4k/bluetape-go/web"
	echoadapter "github.com/bluetape4k/bluetape-go/web/echo"
	"github.com/labstack/echo/v4"
)

func TestRequestContextRestoresOriginalRequest(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	original := ctx.Request()
	err := echoadapter.RequestContext(web.RequestContextOptions{
		GenerateID: func() (string, error) { return "request-1", nil },
	})(func(next echo.Context) error {
		if next.Request() == original {
			t.Fatal("middleware did not install a request copy")
		}
		value, ok := web.RequestContextFromContext(next.Request().Context())
		if !ok || value.RequestID != "request-1" {
			t.Fatalf("request context = %#v/%t, want request-1/true", value, ok)
		}
		return next.NoContent(http.StatusNoContent)
	})(ctx)
	if err != nil || recorder.Code != http.StatusNoContent || ctx.Request() != original {
		t.Fatalf("err=%v status=%d request-restored=%t, want nil/204/true", err, recorder.Code, ctx.Request() == original)
	}
}

func TestRequestContextInvalidHeaderWritesBadRequest(t *testing.T) {
	ctx, recorder := newEchoContext(http.MethodGet, "http://example.test/orders", nil)
	err := echoadapter.RequestContext(web.RequestContextOptions{RequestIDHeader: "bad header"})(func(echo.Context) error {
		return nil
	})(ctx)
	if err != nil || recorder.Code != http.StatusBadRequest {
		t.Fatalf("err=%v status=%d, want nil/400", err, recorder.Code)
	}
}
