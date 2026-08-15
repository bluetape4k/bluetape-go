package ginadapter

import (
	"net/http"

	"github.com/bluetape4k/bluetape-go/web"
	"github.com/gin-gonic/gin"
)

// RequestContext는 framework-neutral request context를 Gin request에 연결한다.
//
// middleware가 반환되거나 panic이 전파될 때 원래 request 포인터를 복원한다.
func RequestContext(options web.RequestContextOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil {
			return
		}

		original := c.Request
		defer func() { c.Request = original }()

		request, _, err := web.WithRequestContextOnRequest(original, options)
		if err != nil {
			_ = AbortWithProblem(c, requestContextProblemError{})
			return
		}

		c.Request = request
		c.Next()
	}
}

type requestContextProblemError struct{}

func (requestContextProblemError) Error() string {
	return "invalid request context"
}

func (requestContextProblemError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusBadRequest),
		Status: http.StatusBadRequest,
		Detail: "invalid request context",
	}
}

var _ web.ProblemError = requestContextProblemError{}
