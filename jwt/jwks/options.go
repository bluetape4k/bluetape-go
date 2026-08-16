package jwks

import (
	"errors"
	"net/http"
	"time"

	rootjwt "github.com/bluetape4k/bluetape-go/jwt"
)

const (
	defaultCacheTTL        = 5 * time.Minute
	defaultFetchTimeout    = 10 * time.Second
	defaultMaxBodySize     = int64(1 << 20)
	defaultMaxHeaderSize   = int64(64 << 10)
	hardMaxBodySize        = int64(8 << 20)
	defaultRefreshCooldown = time.Second
)

// Algorithm 은 JWKS provider가 검증할 JOSE 서명 알고리즘 이름이다.
type Algorithm string

const (
	// RS256 은 RSA PKCS#1 v1.5 SHA-256이다.
	RS256 Algorithm = "RS256"
	// RS384 은 RSA PKCS#1 v1.5 SHA-384이다.
	RS384 Algorithm = "RS384"
	// RS512 은 RSA PKCS#1 v1.5 SHA-512이다.
	RS512 Algorithm = "RS512"
	// PS256 은 RSA-PSS SHA-256이다.
	PS256 Algorithm = "PS256"
	// PS384 은 RSA-PSS SHA-384이다.
	PS384 Algorithm = "PS384"
	// PS512 은 RSA-PSS SHA-512이다.
	PS512 Algorithm = "PS512"
	// ES256 은 ECDSA P-256 SHA-256이다.
	ES256 Algorithm = "ES256"
	// ES384 은 ECDSA P-384 SHA-384이다.
	ES384 Algorithm = "ES384"
	// ES512 은 ECDSA P-521 SHA-512이다.
	ES512 Algorithm = "ES512"
	// EdDSA 는 Ed25519 서명이다.
	EdDSA Algorithm = "EdDSA"
)

type config struct {
	client            *http.Client
	cacheTTL          time.Duration
	refreshCooldown   time.Duration
	fetchTimeout      time.Duration
	maxBodySize       int64
	allowedAlgorithms map[Algorithm]struct{}
	now               func() time.Time
}

// Option 은 Provider 생성 설정을 변경한다.
type Option func(*config) error

// WithHTTPClient 는 JWKS fetch에 사용할 HTTP client를 지정한다.
// 주입하는 RoundTripper는 request context 취소와 response body 수명을
// 준수해야 하며, 취소 뒤에도 네트워크 작업을 계속하지 않아야 한다.
func WithHTTPClient(client *http.Client) Option {
	return func(cfg *config) error {
		if client == nil {
			return optionError("http_client", errors.New("must not be nil"))
		}
		cfg.client = client
		return nil
	}
}

// WithCacheTTL 은 성공한 snapshot의 유효 기간을 지정한다.
func WithCacheTTL(ttl time.Duration) Option {
	return func(cfg *config) error {
		if ttl <= 0 {
			return optionError("cache_ttl", errors.New("must be positive"))
		}
		cfg.cacheTTL = ttl
		return nil
	}
}

// WithRefreshCooldown 은 unknown kid와 실패 refresh 재시도 간격을 지정한다.
func WithRefreshCooldown(cooldown time.Duration) Option {
	return func(cfg *config) error {
		if cooldown <= 0 {
			return optionError("refresh_cooldown", errors.New("must be positive"))
		}
		cfg.refreshCooldown = cooldown
		return nil
	}
}

// WithFetchTimeout 은 provider-owned fetch timeout을 지정한다.
func WithFetchTimeout(timeout time.Duration) Option {
	return func(cfg *config) error {
		if timeout <= 0 {
			return optionError("fetch_timeout", errors.New("must be positive"))
		}
		cfg.fetchTimeout = timeout
		return nil
	}
}

// WithMaxBodySize 는 JWKS response의 bounded body 크기를 지정한다.
func WithMaxBodySize(size int64) Option {
	return func(cfg *config) error {
		if size <= 0 {
			return optionError("max_body_size", errors.New("must be positive"))
		}
		if size > hardMaxBodySize {
			return optionError("max_body_size", errors.New("exceeds hard limit"))
		}
		cfg.maxBodySize = size
		return nil
	}
}

// WithAllowedAlgorithms 는 기본 asymmetric algorithm 집합을 더 좁힌다.
func WithAllowedAlgorithms(algorithms ...Algorithm) Option {
	return func(cfg *config) error {
		if len(algorithms) == 0 {
			return optionError("allowed_algorithms", errors.New("must not be empty"))
		}
		requested := make(map[Algorithm]struct{}, len(algorithms))
		for _, algorithm := range algorithms {
			if !isSupportedAlgorithm(algorithm) {
				return optionError("allowed_algorithms", errors.New("contains unsupported algorithm"))
			}
			if _, exists := requested[algorithm]; exists {
				return optionError("allowed_algorithms", errors.New("contains duplicate algorithm"))
			}
			requested[algorithm] = struct{}{}
		}
		allowed := make(map[Algorithm]struct{}, len(requested))
		for algorithm := range requested {
			if _, exists := cfg.allowedAlgorithms[algorithm]; exists {
				allowed[algorithm] = struct{}{}
			}
		}
		if len(allowed) == 0 {
			return optionError("allowed_algorithms", errors.New("does not narrow the current set"))
		}
		cfg.allowedAlgorithms = allowed
		return nil
	}
}

func defaultConfig() config {
	return config{
		client:            defaultHTTPClient(),
		cacheTTL:          defaultCacheTTL,
		refreshCooldown:   defaultRefreshCooldown,
		fetchTimeout:      defaultFetchTimeout,
		maxBodySize:       defaultMaxBodySize,
		allowedAlgorithms: defaultAlgorithmSet(),
		now:               time.Now,
	}
}

func optionError(name string, err error) error {
	return rootjwt.OptionError{Option: name, Err: err}
}

func defaultAlgorithmSet() map[Algorithm]struct{} {
	algorithms := []Algorithm{RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384, ES512, EdDSA}
	set := make(map[Algorithm]struct{}, len(algorithms))
	for _, algorithm := range algorithms {
		set[algorithm] = struct{}{}
	}
	return set
}

func isSupportedAlgorithm(algorithm Algorithm) bool {
	switch algorithm {
	case RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384, ES512, EdDSA:
		return true
	default:
		return false
	}
}
