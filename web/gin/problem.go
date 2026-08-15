package ginadapter

import (
	"net/http"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/gin-gonic/gin"
)

// AbortWithProblem은 RFC 9457 Problem 응답을 기록하고 Gin chain을 중단한다.
func AbortWithProblem(c *gin.Context, err error) error {
	if c == nil || err == nil {
		return web.ErrInvalidProblem
	}
	c.Abort()
	if c.Writer.Written() {
		return nil
	}
	writeErr := web.WriteProblem(c.Writer, c.Request, err)
	if writeErr == nil || c.Writer.Written() {
		return writeErr
	}
	if fallbackErr := web.WriteProblem(c.Writer, c.Request, fallbackProblemError{}); fallbackErr != nil {
		return writeErr
	}
	return writeErr
}

// JWTReader는 Gin context에 저장된 검증 reader를 읽는다.
func JWTReader(c *gin.Context, key string) (*jwt.Reader, bool) {
	if c == nil {
		return nil, false
	}
	if key == "" {
		key = DefaultJWTContextKey
	}
	value, ok := c.Get(key)
	if !ok {
		return nil, false
	}
	reader, ok := value.(*jwt.Reader)
	return reader, ok
}

type fallbackProblemError struct{}

func (fallbackProblemError) Error() string {
	return "internal problem response failure"
}

func (fallbackProblemError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Detail: "internal server error",
	}
}

var _ web.ProblemError = AuthenticationError{}
var _ web.ProblemError = fallbackProblemError{}
