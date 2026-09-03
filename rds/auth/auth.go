package auth

import (
	"context"
	"net"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsauth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
)

const (
	maxEndpointBytes = 512
	maxRegionBytes   = 128
	maxUsernameBytes = 128
)

// Request - RDS IAM auth token signing에 필요한 caller-owned 값이다.
// Endpoint는 scheme/path/query/fragment가 없는 host:port 형식이어야 한다.
type Request struct {
	Endpoint string
	Region   string
	Username string
}

// BuildAuthToken - AWS SDK RDS auth signing을 검증된 redacted Token으로
// 반환한다. SDK가 생성하는 token의 유효 기간은 15분이며 refresh 시점은
// caller가 소유한다.
func BuildAuthToken(ctx context.Context, request Request, credentials aws.CredentialsProvider) (Token, error) {
	var zero Token
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := validateRequest(request); err != nil {
		return zero, err
	}
	if isNilCredentials(credentials) {
		return zero, newError(ErrNilCredentials, "validate credentials", nil)
	}

	token, callErr := awsauth.BuildAuthToken(ctx, request.Endpoint, request.Region, request.Username, credentials)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if callErr != nil {
		return zero, newError(ErrBuildFailed, "build", callErr)
	}
	if token == "" || !utf8.ValidString(token) {
		return zero, newError(ErrMalformedToken, "validate token", nil)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return newToken(token), nil
}

func validateRequest(request Request) error {
	if !validEndpoint(request.Endpoint) || !validField(request.Region, maxRegionBytes) || !validField(request.Username, maxUsernameBytes) {
		return newError(ErrInvalidRequest, "validate request", nil)
	}
	return nil
}

func validEndpoint(endpoint string) bool {
	if !validField(endpoint, maxEndpointBytes) || strings.ContainsAny(endpoint, "/?#") || strings.Contains(endpoint, "://") {
		return false
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" || port == "" || !validHost(host) {
		return false
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	return err == nil && portNumber > 0
}

func validHost(host string) bool {
	if !utf8.ValidString(host) || strings.TrimSpace(host) != host || strings.ContainsAny(host, "/?#[]") {
		return false
	}
	for _, character := range host {
		if unicode.IsSpace(character) {
			return false
		}
	}
	if strings.Contains(host, ":") {
		return net.ParseIP(host) != nil
	}
	return true
}

func validField(value string, maxBytes int) bool {
	if !utf8.ValidString(value) || value == "" || len(value) > maxBytes || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return false
		}
	}
	return true
}

func isNilCredentials(credentials aws.CredentialsProvider) bool {
	if credentials == nil {
		return true
	}
	value := reflect.ValueOf(credentials)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
