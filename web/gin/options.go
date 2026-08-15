package ginadapter

import (
	"context"
	"fmt"
	"reflect"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/bluetape4k/bluetape-go/ratelimit"
	"github.com/bluetape4k/bluetape-go/resilience"
	"github.com/bluetape4k/bluetape-go/web"
	"github.com/gin-gonic/gin"
)

// DefaultJWTContextKey는 검증된 JWT reader를 저장하는 기본 Gin key다.
const DefaultJWTContextKey = "bluetape.web.gin.jwt"

// RateLimitKeyFunc는 Gin 요청에서 rate-limit key를 추출한다.
type RateLimitKeyFunc func(*gin.Context) string

// RateLimitOptions는 Gin rate-limit middleware 설정이다.
type RateLimitOptions struct {
	Limiter      ratelimit.Limiter
	KeyFunc      RateLimitKeyFunc
	Tokens       int64
	ErrorHandler func(*gin.Context, ratelimit.Result, error)
}

// ContextParser는 request context를 받는 JWT parser 계약이다.
type ContextParser interface {
	ParseContext(context.Context, string, ...jwt.ParseOption) (*jwt.Reader, error)
}

// JWTOptions는 Gin JWT middleware 설정이다.
type JWTOptions struct {
	Parser        jwt.Parser
	ContextParser ContextParser
	Header        string
	Scheme        string
	ContextKey    string
	ParseOptions  []jwt.ParseOption
	ErrorHandler  func(*gin.Context, error)
}

// JWTErrorKind는 redacted JWT 인증 실패 분류다.
type JWTErrorKind string

const (
	// JWTErrorMissing은 인증 header가 없음을 나타낸다.
	JWTErrorMissing JWTErrorKind = "missing"
	// JWTErrorMalformed는 인증 header 문법 오류를 나타낸다.
	JWTErrorMalformed JWTErrorKind = "malformed"
	// JWTErrorInvalid은 검증 실패를 나타낸다.
	JWTErrorInvalid JWTErrorKind = "invalid"
	// JWTErrorExpired는 만료된 token을 나타낸다.
	JWTErrorExpired JWTErrorKind = "expired"
	// JWTErrorCanceled는 request cancellation 또는 deadline을 나타낸다.
	JWTErrorCanceled JWTErrorKind = "canceled"
)

// AuthenticationError는 token 또는 parser 원인을 포함하지 않는 인증 오류다.
type AuthenticationError struct {
	Kind JWTErrorKind
}

// Error는 안정된 redacted 인증 오류 문자열을 반환한다.
func (e AuthenticationError) Error() string {
	kind := e.Kind
	if kind == "" {
		kind = JWTErrorInvalid
	}
	return fmt.Sprintf("authentication failed: %s", kind)
}

// ProblemDetails는 인증 실패를 공개 가능한 401 Problem으로 변환한다.
func (e AuthenticationError) ProblemDetails() web.Problem {
	return web.Problem{
		Type:   "about:blank",
		Title:  "Unauthorized",
		Status: 401,
		Detail: "authentication failed",
	}
}

// ResilienceOptions는 route-level resilience middleware 설정이다.
type ResilienceOptions struct {
	Policies     []resilience.Policy[struct{}]
	ErrorHandler func(*gin.Context, error)
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
