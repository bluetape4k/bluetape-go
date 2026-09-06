package geocoding

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/geo"
)

const (
	// DefaultMaxResponseBytes는 bounded JSON response 기본 상한이다.
	DefaultMaxResponseBytes int64 = 1 << 20
	// MaxResponseBytes는 호출자가 지정할 수 있는 response 상한이다.
	MaxResponseBytes int64 = 16 << 20
)

// Options는 reverse 요청의 언어, 상세도와 attribution 선택을 보관한다.
type Options struct {
	// Language는 Nominatim accept-language 값이다.
	Language string
	// Zoom은 0이면 provider 기본값을 사용하고, 1..18이면 zoom query를 보낸다.
	Zoom int
	// AddressDetails는 address object 확장을 요청한다.
	AddressDetails bool
	// ExtraTags는 provider extra tags 확장을 요청한다.
	ExtraTags bool
	// NameDetails는 provider name details 확장을 요청한다.
	NameDetails bool
	// IncludeAttribution은 결과에 provider licence를 보존할지 결정한다.
	IncludeAttribution bool
}

// Validate는 요청 option의 범위와 bounded 문자열을 검사한다.
func (o Options) Validate() error {
	if o.Zoom < 0 || o.Zoom > 18 {
		return fmt.Errorf("%w: zoom must be between 0 and 18", ErrInvalidOptions)
	}
	if len(o.Language) > 128 {
		return fmt.Errorf("%w: language is too long", ErrInvalidOptions)
	}
	return nil
}

// CacheKey는 좌표와 요청 의미를 stable low-cardinality 문자열로 반환한다.
func (o Options) CacheKey(point geo.Point) string {
	return strings.Join([]string{
		"nominatim-reverse-v1",
		strconv.FormatFloat(point.Latitude(), 'g', -1, 64),
		strconv.FormatFloat(point.Longitude(), 'g', -1, 64),
		o.Language,
		strconv.Itoa(o.Zoom),
		strconv.FormatBool(o.AddressDetails),
		strconv.FormatBool(o.ExtraTags),
		strconv.FormatBool(o.NameDetails),
		strconv.FormatBool(o.IncludeAttribution),
	}, "|")
}

// RateLimiter는 실제 요청 전에 caller-owned rate policy를 적용한다.
type RateLimiter interface {
	Wait(context.Context) error
}

// Cache는 reverse 결과를 저장할 caller-owned 선택적 cache 경계다.
type Cache interface {
	Get(context.Context, string) (Result, bool, error)
	Set(context.Context, string, Result) error
}

// Option은 Nominatim client 설정을 변경한다.
type Option func(*Nominatim) error

// WithMaxResponseBytes는 response body 상한을 설정한다.
func WithMaxResponseBytes(limit int64) Option {
	return func(client *Nominatim) error {
		if limit <= 0 || limit > MaxResponseBytes {
			return fmt.Errorf("%w: response limit must be between 1 and %d", ErrInvalidOptions, MaxResponseBytes)
		}
		client.maxResponseBytes = limit
		return nil
	}
}

// WithTimeout은 HTTP 요청에 사용할 adapter-owned 상한을 설정한다.
func WithTimeout(timeout time.Duration) Option {
	return func(client *Nominatim) error {
		if timeout <= 0 {
			return fmt.Errorf("%w: timeout must be positive", ErrInvalidOptions)
		}
		client.timeout = timeout
		return nil
	}
}

// WithRateLimiter는 요청 전 대기를 caller-owned limiter에 위임한다.
func WithRateLimiter(limiter RateLimiter) Option {
	return func(client *Nominatim) error {
		if limiter == nil {
			return fmt.Errorf("%w: nil rate limiter", ErrInvalidOptions)
		}
		client.rateLimiter = limiter
		return nil
	}
}

// WithCache는 결과 cache를 caller-owned hook으로 주입한다.
func WithCache(cache Cache) Option {
	return func(client *Nominatim) error {
		if cache == nil {
			return fmt.Errorf("%w: nil cache", ErrInvalidOptions)
		}
		client.cache = cache
		return nil
	}
}
