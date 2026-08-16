package echoadapter

import (
	"context"
	"fmt"
	"reflect"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/labstack/echo/v4"
)

// DefaultJWTContextKey is the default Echo key for a validated JWT reader.
// 검증된 JWT reader를 저장하는 기본 Echo key다.
const DefaultJWTContextKey = "bluetape.web.echo.jwt"

// DefaultResilienceErrorContextKey is the Echo key for a redacted resilience observer error.
// redacted resilience observer 오류를 저장하는 기본 Echo key다.
const DefaultResilienceErrorContextKey = "bluetape.web.echo.resilience.error"

// RateLimitKeyFunc extracts a rate-limit key from an Echo request.
// Echo request에서 rate-limit key를 추출한다.
type RateLimitKeyFunc func(echo.Context) string

// RateLimitOptions configures Echo rate-limit middleware.
// Echo rate-limit middleware 설정이다.
type RateLimitOptions struct {
	Limiter      ratelimit.Limiter
	KeyFunc      RateLimitKeyFunc
	Tokens       int64
	ErrorHandler func(echo.Context, ratelimit.Result, error)
}

// ContextParser defines a JWT parser contract that receives request context.
// request context를 받는 JWT parser 계약이다.
type ContextParser interface {
	ParseContext(context.Context, string, ...jwt.ParseOption) (*jwt.Reader, error)
}

// JWTOptions configures Echo JWT middleware.
// Echo JWT middleware 설정이다.
type JWTOptions struct {
	Parser        jwt.Parser
	ContextParser ContextParser
	Header        string
	Scheme        string
	ContextKey    string
	ParseOptions  []jwt.ParseOption
	ErrorHandler  func(echo.Context, error)
}

// JWTErrorKind classifies a redacted JWT authentication failure.
// redacted JWT 인증 실패 분류다.
type JWTErrorKind string

const (
	// JWTErrorMissing indicates that the authentication header is absent.
	JWTErrorMissing JWTErrorKind = "missing"
	// JWTErrorMalformed indicates malformed authentication header syntax.
	JWTErrorMalformed JWTErrorKind = "malformed"
	// JWTErrorInvalid indicates token verification failure.
	JWTErrorInvalid JWTErrorKind = "invalid"
	// JWTErrorExpired indicates an expired token.
	JWTErrorExpired JWTErrorKind = "expired"
	// JWTErrorCanceled indicates request cancellation or a deadline.
	JWTErrorCanceled JWTErrorKind = "canceled"
)

// AuthenticationError is a redacted authentication error without token or parser cause.
// token 또는 parser 원인을 포함하지 않는 인증 오류다.
type AuthenticationError struct {
	Kind JWTErrorKind
}

// Error returns a stable redacted authentication error string.
func (e AuthenticationError) Error() string {
	kind := e.Kind
	if kind == "" {
		kind = JWTErrorInvalid
	}
	return fmt.Sprintf("authentication failed: %s", kind)
}

// ProblemDetails converts an authentication failure to a public 401 Problem.
func (e AuthenticationError) ProblemDetails() web.Problem {
	return web.Problem{Type: "about:blank", Title: "Unauthorized", Status: 401, Detail: "authentication failed"}
}

// ResilienceOptions configures an Echo route-level resilience wrapper.
// Echo route-level resilience wrapper 설정이다.
type ResilienceOptions struct {
	Policies     []resilience.Policy[struct{}]
	ErrorHandler func(echo.Context, error)
}

// ResilienceError reads the redacted resilience observer stored by WrapResilience.
// WrapResilience가 저장한 redacted resilience observer 오류를 읽는다.
func ResilienceError(c echo.Context) (error, bool) {
	if isNilInterface(c) {
		return nil, false
	}
	value := c.Get(DefaultResilienceErrorContextKey)
	err, ok := value.(error)
	if !ok || isNilInterface(err) {
		return nil, false
	}
	return err, true
}

func validateParserOptions(options JWTOptions) error {
	if isNilInterface(options.Parser) == isNilInterface(options.ContextParser) {
		return fmt.Errorf("exactly one JWT parser must be configured")
	}
	for index, option := range options.ParseOptions {
		if option == nil {
			return fmt.Errorf("parse option %d must not be nil", index)
		}
	}
	return nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
