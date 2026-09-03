package secretsmanager

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/bluetape4k/bluetape-go/cache"
)

const maxNameBytes = 2048

// Client - Provider가 사용하는 AWS Secrets Manager SDK method의 최소 집합이다.
// credentials, retry, transport와 lifecycle은 caller가 소유한다.
type Client interface {
	GetSecretValue(context.Context, *awssm.GetSecretValueInput, ...func(*awssm.Options)) (*awssm.GetSecretValueOutput, error)
}

var _ Client = (*awssm.Client)(nil)

// Options - Provider 생성에 필요한 caller-owned 설정이다.
type Options struct {
	// Client는 호출자가 생성하고 수명을 관리하는 Secrets Manager client다.
	Client Client
	// Cache는 caller-owned positive TTL cache다. CacheTTL이 positive이면
	// bounded capacity/eviction 정책을 가진 cache를 반드시 제공해야 한다.
	Cache cache.LoadingCache[string, Value]
	// CacheTTL이 0이면 cache를 사용하지 않는다. 음수는 거부된다.
	CacheTTL time.Duration
}

// Provider - caller-owned Secrets Manager client로 값을 조회한다.
// 생성 후 client와 cache 설정은 변경하지 않는다는 전제에서 concurrent Get이 안전하다.
type Provider struct {
	client   Client
	cache    cache.LoadingCache[string, Value]
	cacheTTL time.Duration
}

// New - client와 optional TTL cache 설정을 검증해 Provider를 생성한다.
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
	if options.CacheTTL > 0 && options.Cache == nil {
		return nil, newError(ErrInvalidOptions, "validate options", nil)
	}
	return &Provider{client: options.Client, cache: options.Cache, cacheTTL: options.CacheTTL}, nil
}

// Get - secret ARN 또는 name으로 현재 secret 값을 조회한다.
// 값은 cache hit에서도 독립된 immutable snapshot으로 반환된다.
func (p *Provider) Get(ctx context.Context, name string) (Value, error) {
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
		return p.lookup(ctx, name)
	}

	value, err := p.cache.GetOrLoad(ctx, name, p.cacheTTL, func(loadCtx context.Context, _ string) (Value, error) {
		return p.lookup(loadCtx, name)
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

func (p *Provider) lookup(ctx context.Context, name string) (Value, error) {
	var zero Value
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	output, callErr := p.client.GetSecretValue(ctx, &awssm.GetSecretValueInput{SecretId: aws.String(name)})
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if callErr != nil {
		return zero, newError(ErrLookupFailed, "lookup", callErr)
	}
	if output == nil {
		return zero, newError(ErrMalformedOutput, "lookup", nil)
	}
	hasString := output.SecretString != nil
	hasBinary := output.SecretBinary != nil
	if hasString && hasBinary {
		return zero, newError(ErrMalformedOutput, "lookup", nil)
	}
	if !hasString && !hasBinary {
		return zero, newError(ErrMissingValue, "lookup", nil)
	}
	var value Value
	if hasString {
		value = newTextValue(*output.SecretString)
	} else {
		value = newBinaryValue(output.SecretBinary)
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return value, nil
}

func (p *Provider) validate() error {
	if p == nil || isNilClient(p.client) || p.cacheTTL < 0 || (p.cacheTTL > 0 && (p.cache == nil || isNilCache(p.cache))) {
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
