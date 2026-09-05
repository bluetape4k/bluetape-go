package echoadapter

import (
	"net/http"

	"github.com/bluetape4k/bluetape-go/web"
	"github.com/labstack/echo/v4"
)

// RequestContext bridges a framework-neutral request context to Echo.
// framework-neutral request context를 Echo request에 연결한다.
func RequestContext(options web.RequestContextOptions) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if isNilInterface(c) {
				return nil
			}
			if next == nil {
				return c.NoContent(http.StatusNotFound)
			}
			original := c.Request()
			defer c.SetRequest(original)

			request, _, err := web.WithRequestContextOnRequest(original, options)
			if err != nil {
				return AbortWithProblem(c, requestContextProblemError{})
			}
			c.SetRequest(request)
			return next(c)
		}
	}
}

type requestContextProblemError struct{}

func (requestContextProblemError) Error() string { return "invalid request context" }

func (requestContextProblemError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: http.StatusText(http.StatusBadRequest), Status: http.StatusBadRequest, Detail: "invalid request context"}
}

var _ web.ProblemError = requestContextProblemError{}
