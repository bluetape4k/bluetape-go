package echoadapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/jwt"
	"github.com/labstack/echo/v4"
)

const maxBearerTokenBytes = 8 * 1024

// NewJWT creates Echo middleware for strict Bearer authentication.
// 엄격한 Bearer 인증을 수행하는 Echo middleware를 만든다.
func NewJWT(options JWTOptions) (echo.MiddlewareFunc, error) {
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
	if isNilInterface(parser) {
		parser = nil
	}
	contextParser := options.ContextParser
	if isNilInterface(contextParser) {
		contextParser = nil
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if isNilInterface(c) {
				return nil
			}
			original := c.Request()
			if original == nil {
				return failJWT(c, AuthenticationError{Kind: JWTErrorMalformed}, header, options.ErrorHandler)
			}
			requestContext := original.Context()
			if requestContext == nil {
				requestContext = context.Background()
			}

			token, kind := parseBearerToken(original, header, scheme)
			if kind != "" {
				return failJWT(c, AuthenticationError{Kind: kind}, header, options.ErrorHandler)
			}
			if err := requestContext.Err(); err != nil {
				return failJWT(c, AuthenticationError{Kind: JWTErrorCanceled}, header, options.ErrorHandler)
			}

			reader, parseErr := parseJWT(requestContext, token, parser, contextParser, parseOptions)
			if parseErr != nil || reader == nil {
				return failJWT(c, AuthenticationError{Kind: classifyJWTError(requestContext, parseErr)}, header, options.ErrorHandler)
			}
			if err := requestContext.Err(); err != nil {
				return failJWT(c, AuthenticationError{Kind: JWTErrorCanceled}, header, options.ErrorHandler)
			}

			c.Set(contextKey, reader)
			if next == nil {
				return nil
			}
			return next(c)
		}
	}, nil
}

func parseJWT(ctx context.Context, token string, parser jwt.Parser, contextParser ContextParser, options []jwt.ParseOption) (*jwt.Reader, error) {
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
	if ctx != nil && ctx.Err() != nil {
		return JWTErrorCanceled
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

func failJWT(c echo.Context, err AuthenticationError, header string, callback func(echo.Context, error)) error {
	if callback == nil {
		return AbortWithProblem(c, err)
	}

	original := c.Request()
	if original == nil {
		callback(c, err)
		return nil
	}
	copyRequest := original.Clone(original.Context())
	copyRequest.Body = http.NoBody
	copyRequest.GetBody = nil
	copyRequest.ContentLength = 0
	for key := range copyRequest.Header {
		if strings.EqualFold(key, header) || strings.EqualFold(key, "Authorization") {
			delete(copyRequest.Header, key)
		}
	}
	c.SetRequest(copyRequest)
	defer c.SetRequest(original)
	callback(c, err)
	return nil
}
