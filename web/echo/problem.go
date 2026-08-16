package echoadapter

import (
	"net/http"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/labstack/echo/v4"
)

// AbortWithProblem writes an RFC 9457 Problem response for an Echo context.
// RFC 9457 Problem 응답을 기록한다. 이미 commit된 응답은 덮어쓰지 않는다.
func AbortWithProblem(c echo.Context, err error) error {
	if isNilInterface(c) || err == nil {
		return web.ErrInvalidProblem
	}
	response := c.Response()
	if response == nil || response.Writer == nil {
		return web.ErrInvalidProblem
	}
	if response.Committed {
		return nil
	}

	writeErr := web.WriteProblem(response.Writer, c.Request(), err)
	if writeErr == nil || response.Committed {
		return writeErr
	}
	if fallbackErr := web.WriteProblem(response.Writer, c.Request(), fallbackProblemError{}); fallbackErr != nil {
		return writeErr
	}
	return writeErr
}

// JWTReader reads a validated JWT reader stored in the Echo context.
// Echo context에 저장된 검증 reader를 읽는다.
func JWTReader(c echo.Context, key string) (*jwt.Reader, bool) {
	if isNilInterface(c) {
		return nil, false
	}
	if key == "" {
		key = DefaultJWTContextKey
	}
	value := c.Get(key)
	reader, ok := value.(*jwt.Reader)
	if !ok || reader == nil {
		return nil, false
	}
	return reader, true
}

type fallbackProblemError struct{}

func (fallbackProblemError) Error() string { return "internal problem response failure" }

func (fallbackProblemError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: http.StatusText(http.StatusInternalServerError), Status: http.StatusInternalServerError, Detail: "internal server error"}
}

var (
	_ web.ProblemError = AuthenticationError{}
	_ web.ProblemError = fallbackProblemError{}
)
