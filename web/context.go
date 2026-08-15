package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/bluetape4k/bluetape-go/id"
)

const (
	// RequestIDHeader 는 기본 request ID header 이름이다.
	RequestIDHeader = "X-Request-ID"
	// CorrelationIDHeader 는 기본 correlation ID header 이름이다.
	CorrelationIDHeader = "X-Correlation-ID"
	// AuthSubjectHeader 는 trusted proxy가 전달할 수 있는 subject header 이름이다.
	AuthSubjectHeader = "X-Auth-Subject"
	// TraceParentHeader 는 W3C traceparent header 이름이다.
	TraceParentHeader = "traceparent"
	// TraceStateHeader 는 W3C tracestate header 이름이다.
	TraceStateHeader = "tracestate"
)

var (
	// ErrInvalidRequestContext 는 request context 입력이 계약을 위반할 때 반환된다.
	ErrInvalidRequestContext = errors.New("web: invalid request context")
)

// RequestContext 는 request에 연결할 식별자와 제한된 전달 컨텍스트다.
type RequestContext struct {
	RequestID     string
	CorrelationID string
	AuthSubject   string
	TraceParent   string
	TraceState    string
}

// RequestContextOptions 는 header 이름, ID 생성기, trusted proxy 정책을 설정한다.
type RequestContextOptions struct {
	TrustedProxy        func(*http.Request) bool
	GenerateID          func() (string, error)
	RequestIDHeader     string
	CorrelationIDHeader string
	AuthSubjectHeader   string
	TraceParentHeader   string
	TraceStateHeader    string
}

type requestContextKey struct{}

// ExtractRequestContext 는 request header에서 검증된 RequestContext를 추출한다.
func ExtractRequestContext(req *http.Request, options RequestContextOptions) (RequestContext, error) {
	if req == nil {
		return RequestContext{}, invalidRequestContext("request must not be nil")
	}

	headers, err := resolveHeaderNames(options)
	if err != nil {
		return RequestContext{}, err
	}

	requestID, err := validatedHeaderValue(req.Header.Get(headers.requestID), headers.requestID)
	if err != nil {
		return RequestContext{}, err
	}
	if requestID == "" {
		generateID := options.GenerateID
		if generateID == nil {
			generateID = id.NewUUIDV7
		}
		requestID, err = generateID()
		if err != nil {
			return RequestContext{}, err
		}
		requestID, err = validatedHeaderValue(requestID, "generated request ID")
		if err != nil {
			return RequestContext{}, err
		}
		if requestID == "" {
			return RequestContext{}, invalidRequestContext("generated request ID must not be empty")
		}
	}

	correlationID, err := validatedHeaderValue(req.Header.Get(headers.correlationID), headers.correlationID)
	if err != nil {
		return RequestContext{}, err
	}
	if correlationID == "" {
		correlationID = requestID
	}

	value := RequestContext{RequestID: requestID, CorrelationID: correlationID}
	trusted := options.TrustedProxy != nil && options.TrustedProxy(req)
	if !trusted {
		return value, nil
	}

	value.AuthSubject, err = validatedHeaderValue(req.Header.Get(headers.authSubject), headers.authSubject)
	if err != nil {
		return RequestContext{}, err
	}
	value.TraceParent, err = validatedHeaderValue(req.Header.Get(headers.traceParent), headers.traceParent)
	if err != nil {
		return RequestContext{}, err
	}
	if value.TraceParent != "" {
		if err := validateTraceParent(value.TraceParent); err != nil {
			return RequestContext{}, err
		}
	}
	value.TraceState, err = validatedHeaderValue(req.Header.Get(headers.traceState), headers.traceState)
	if err != nil {
		return RequestContext{}, err
	}
	return value, nil
}

// WithRequestContext 는 RequestContext를 context에 저장한다.
func WithRequestContext(ctx context.Context, value RequestContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, requestContextKey{}, value)
}

// RequestContextFromContext 는 context에 저장된 RequestContext를 읽는다.
func RequestContextFromContext(ctx context.Context) (RequestContext, bool) {
	if ctx == nil {
		return RequestContext{}, false
	}
	value, ok := ctx.Value(requestContextKey{}).(RequestContext)
	return value, ok
}

// WithRequestContextOnRequest 는 원본을 보존한 채 context가 연결된 request를 반환한다.
func WithRequestContextOnRequest(req *http.Request, options RequestContextOptions) (*http.Request, RequestContext, error) {
	if req == nil {
		return nil, RequestContext{}, invalidRequestContext("request must not be nil")
	}

	value, err := ExtractRequestContext(req, options)
	if err != nil {
		return nil, RequestContext{}, err
	}
	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return req.WithContext(WithRequestContext(ctx, value)), value, nil
}

type resolvedHeaderNames struct {
	requestID     string
	correlationID string
	authSubject   string
	traceParent   string
	traceState    string
}

func resolveHeaderNames(options RequestContextOptions) (resolvedHeaderNames, error) {
	headers := resolvedHeaderNames{
		requestID:     options.RequestIDHeader,
		correlationID: options.CorrelationIDHeader,
		authSubject:   options.AuthSubjectHeader,
		traceParent:   options.TraceParentHeader,
		traceState:    options.TraceStateHeader,
	}
	if headers.requestID == "" {
		headers.requestID = RequestIDHeader
	}
	if headers.correlationID == "" {
		headers.correlationID = CorrelationIDHeader
	}
	if headers.authSubject == "" {
		headers.authSubject = AuthSubjectHeader
	}
	if headers.traceParent == "" {
		headers.traceParent = TraceParentHeader
	}
	if headers.traceState == "" {
		headers.traceState = TraceStateHeader
	}

	for _, header := range []string{
		headers.requestID,
		headers.correlationID,
		headers.authSubject,
		headers.traceParent,
		headers.traceState,
	} {
		if !isHTTPToken(header) {
			return resolvedHeaderNames{}, invalidRequestContext("invalid header name %q", header)
		}
	}
	return headers, nil
}

func validatedHeaderValue(value, name string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if !utf8.ValidString(value) || len(value) > 256 {
		return "", invalidRequestContext("invalid %s value", name)
	}
	for i := 0; i < len(value); i++ {
		if value[i] < 0x21 || value[i] > 0x7e {
			return "", invalidRequestContext("invalid %s value", name)
		}
	}
	return value, nil
}

func validateTraceParent(value string) error {
	if len(value) != 55 || value[2] != '-' || value[35] != '-' || value[52] != '-' {
		return invalidRequestContext("invalid traceparent value")
	}
	version := value[:2]
	traceID := value[3:35]
	parentID := value[36:52]
	flags := value[53:]
	if !isLowerHex(version) || version == "ff" || !isLowerHex(traceID) || !isLowerHex(parentID) || !isLowerHex(flags) {
		return invalidRequestContext("invalid traceparent value")
	}
	if allZero(traceID) || allZero(parentID) {
		return invalidRequestContext("invalid traceparent value")
	}
	return nil
}

func isLowerHex(value string) bool {
	for i := 0; i < len(value); i++ {
		if (value[i] < '0' || value[i] > '9') && (value[i] < 'a' || value[i] > 'f') {
			return false
		}
	}
	return true
}

func allZero(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] != '0' {
			return false
		}
	}
	return true
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !isHTTPTokenChar(value[i]) {
			return false
		}
	}
	return true
}

func isHTTPTokenChar(value byte) bool {
	if value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' {
		return true
	}
	switch value {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}

func invalidRequestContext(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRequestContext, fmt.Sprintf(format, args...))
}
