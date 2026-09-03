package ssm

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/bluetape4k/bluetape-go/cache"
)

const maxNameBytes = 2048

// Client - Provider가 사용하는 AWS Systems Manager SDK method의 최소 집합이다.
// credentials, retry, transport와 lifecycle은 caller가 소유한다.
type Client interface {
	GetParameter(context.Context, *awsssm.GetParameterInput, ...func(*awsssm.Options)) (*awsssm.GetParameterOutput, error)
}

var _ Client = (*awsssm.Client)(nil)

// Options - Provider 생성에 필요한 caller-owned 설정이다.
type Options struct {
	// Client는 호출자가 생성하고 수명을 관리하는 Parameter Store client다.
	Client Client
	// Cache는 optional caller-owned positive TTL cache다. nil이면 CacheTTL이
	// positive일 때 process-local cache.Memory를 provider가 생성한다.
	Cache cache.LoadingCache[string, Value]
	// CacheTTL이 0이면 cache를 사용하지 않는다. 음수는 거부된다.
	CacheTTL time.Duration
	// WithDecryption은 Get이 SecureString을 복호화하도록 요청할지 결정한다.
	WithDecryption bool
}

// Provider - caller-owned Parameter Store client로 값을 조회한다.
// 생성 후 client와 cache 설정은 변경하지 않는다는 전제에서 concurrent Get이 안전하다.
type Provider struct {
	client         Client
	cache          cache.LoadingCache[string, Value]
	cacheTTL       time.Duration
	withDecryption bool
}

// New - client, decryption과 optional TTL cache 설정을 검증해 Provider를 생성한다.
func New(options Options) (*Provider, error) {
	if isNilClient(options.Client) {
		return nil, ErrNilClient
	}
	if options.CacheTTL < 0 {
		return nil, newError(ErrInvalidOptions, "validate options", nil)
	}
	if options.Cache != nil && isNilCache(options.Cache) {
		return nil, newError(ErrInvalidOptions, "validate options", nil)
	}
	lookupCache := options.Cache
	if lookupCache == nil && options.CacheTTL > 0 {
		lookupCache = cache.NewMemory[string, Value]()
	}
	return &Provider{
		client:         options.Client,
		cache:          lookupCache,
		cacheTTL:       options.CacheTTL,
		withDecryption: options.WithDecryption,
	}, nil
}

// Get - provider option에 설정된 decryption mode로 parameter를 조회한다.
func (p *Provider) Get(ctx context.Context, name string) (Value, error) {
	return p.get(ctx, name, p != nil && p.withDecryption)
}

// GetSecure - SecureString 복호화를 명시적으로 요청해 parameter를 조회한다.
func (p *Provider) GetSecure(ctx context.Context, name string) (Value, error) {
	return p.get(ctx, name, true)
}

func (p *Provider) get(ctx context.Context, name string, withDecryption bool) (Value, error) {
	var zero Value
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if err := p.validate(); err != nil {
		return zero, err
	}
	if err := validateName(name); err != nil {
		return zero, err
	}
	if p.cache == nil || p.cacheTTL == 0 {
		return p.lookup(ctx, name, withDecryption)
	}

	cacheKey := cacheKey(name, withDecryption)
	value, err := p.cache.GetOrLoad(ctx, cacheKey, p.cacheTTL, func(loadCtx context.Context, _ string) (Value, error) {
		return p.lookup(loadCtx, name, withDecryption)
	})
	if contextErr := ctx.Err(); contextErr != nil {
		return zero, contextErr
	}
	if err != nil {
		var providerErr *Error
		if isContextError(err) || errors.As(err, &providerErr) {
			return zero, err
		}
		return zero, newError(ErrCacheFailed, "cache", err)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return value.clone(), nil
}

func (p *Provider) lookup(ctx context.Context, name string, withDecryption bool) (Value, error) {
	var zero Value
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	output, callErr := p.client.GetParameter(ctx, &awsssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(withDecryption),
	})
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if callErr != nil {
		return zero, newError(ErrLookupFailed, "lookup", callErr)
	}
	if output == nil || output.Parameter == nil {
		return zero, newError(ErrMalformedOutput, "lookup", nil)
	}
	if output.Parameter.Value == nil {
		return zero, newError(ErrMissingValue, "lookup", nil)
	}
	value := newValue(*output.Parameter.Value)
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return value, nil
}

func cacheKey(name string, withDecryption bool) string {
	if withDecryption {
		return "secure\x00" + name
	}
	return "plain\x00" + name
}

func (p *Provider) validate() error {
	if p == nil || isNilClient(p.client) || p.cacheTTL < 0 {
		return newError(ErrInvalidOptions, "validate options", nil)
	}
	if p.cache != nil && isNilCache(p.cache) {
		return newError(ErrInvalidOptions, "validate options", nil)
	}
	return nil
}

func validateName(name string) error {
	if !utf8.ValidString(name) || strings.TrimSpace(name) == "" || len(name) > maxNameBytes {
		return newError(ErrInvalidName, "validate name", nil)
	}
	return nil
}

func isNilClient(client Client) bool {
	return isNilValue(client)
}

func isNilCache(lookupCache cache.LoadingCache[string, Value]) bool {
	return isNilValue(lookupCache)
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
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

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
