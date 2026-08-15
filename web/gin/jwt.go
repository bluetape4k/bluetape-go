package ginadapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/gin-gonic/gin"
)

const maxBearerTokenBytes = 8 * 1024

// NewJWT creates Gin middleware for strict Bearer authentication.
// 엄격한 Bearer 인증을 수행하는 Gin middleware를 만든다.
//
// parser 오류와 token 원문은 AuthenticationError로 redaction하고, 성공한
// reader만 Gin context에 저장한다.
func NewJWT(options JWTOptions) (gin.HandlerFunc, error) {
	if err := validateParserOptions(options); err != nil {
		return nil, err
	}
	header := options.Header
	if header == "" {
		header = "Authorization"
	}
	if !validHTTPToken(header) {
		return nil, fmt.Errorf("header must be a valid HTTP token")
	}
	scheme := options.Scheme
	if scheme == "" {
		scheme = "Bearer"
	}
	if !validHTTPToken(scheme) {
		return nil, fmt.Errorf("scheme must be a valid HTTP token")
	}
	contextKey := options.ContextKey
	if contextKey == "" {
		contextKey = DefaultJWTContextKey
	}
	if !validContextKey(contextKey) {
		return nil, fmt.Errorf("context key must not contain whitespace or control characters")
	}

	parseOptions := append([]jwt.ParseOption(nil), options.ParseOptions...)
	parser := options.Parser
	contextParser := options.ContextParser

	return func(c *gin.Context) {
		if c == nil {
			return
		}
		original := c.Request
		defer func() { c.Request = original }()
		if original == nil {
			failJWT(c, AuthenticationError{Kind: JWTErrorMalformed}, header, options.ErrorHandler)
			return
		}

		token, kind := parseBearerToken(original, header, scheme)
		if kind != "" {
			failJWT(c, AuthenticationError{Kind: kind}, header, options.ErrorHandler)
			return
		}
		requestContext := original.Context()
		if err := requestContext.Err(); err != nil {
			failJWT(c, AuthenticationError{Kind: JWTErrorCanceled}, header, options.ErrorHandler)
			return
		}

		reader, parseErr := parseJWT(requestContext, token, parser, contextParser, parseOptions)
		if parseErr != nil || reader == nil {
			kind := classifyJWTError(requestContext, parseErr)
			failJWT(c, AuthenticationError{Kind: kind}, header, options.ErrorHandler)
			return
		}
		if err := requestContext.Err(); err != nil {
			failJWT(c, AuthenticationError{Kind: JWTErrorCanceled}, header, options.ErrorHandler)
			return
		}

		c.Set(contextKey, reader)
		c.Next()
	}, nil
}

func parseJWT(
	ctx context.Context,
	token string,
	parser jwt.Parser,
	contextParser ContextParser,
	options []jwt.ParseOption,
) (*jwt.Reader, error) {
	if contextParser != nil {
		return contextParser.ParseContext(ctx, token, options...)
	}
	return parser.Parse(token, options...)
}

func parseBearerToken(request *http.Request, header, scheme string) (string, JWTErrorKind) {
	values := request.Header.Values(header)
	if len(values) == 0 {
		return "", JWTErrorMissing
	}
	if len(values) != 1 {
		return "", JWTErrorMalformed
	}
	value := values[0]
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, ",") {
		return "", JWTErrorMalformed
	}
	parts := strings.Split(value, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], scheme) || parts[1] == "" {
		return "", JWTErrorMalformed
	}
	token := parts[1]
	if len(token) > maxBearerTokenBytes || !validToken(token) {
		return "", JWTErrorMalformed
	}
	return token, ""
}

func validToken(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return false
		}
	}
	return true
}

func validHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' {
			continue
		}
		switch char {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		default:
			return false
		}
	}
	return true
}

func validContextKey(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return false
		}
	}
	return true
}

func classifyJWTError(ctx context.Context, err error) JWTErrorKind {
	if ctx != nil {
		if ctx.Err() != nil {
			return JWTErrorCanceled
		}
	}
	if err == nil {
		return JWTErrorInvalid
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return JWTErrorCanceled
	}
	if errors.Is(err, jwt.ErrExpiredToken) {
		return JWTErrorExpired
	}
	return JWTErrorInvalid
}

func failJWT(c *gin.Context, err AuthenticationError, header string, callback func(*gin.Context, error)) {
	c.Abort()
	if callback == nil {
		_ = AbortWithProblem(c, err)
		return
	}

	original := c.Request
	if original == nil {
		callback(c, err)
		return
	}
	copyRequest := original.Clone(original.Context())
	for key := range copyRequest.Header {
		if strings.EqualFold(key, header) || strings.EqualFold(key, "Authorization") {
			delete(copyRequest.Header, key)
		}
	}
	c.Request = copyRequest
	defer func() { c.Request = original }()
	callback(c, err)
}
