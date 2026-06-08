package jwt

import (
	"io"
	"time"
)

type providerConfig struct {
	now        func() time.Time
	entropy    io.Reader
	keyTTL     time.Duration
	capacity   int
	keyID      func() (string, error)
	rsaKeyBits int
}

// ProviderOption 은 provider 생성 설정을 변경한다.
type ProviderOption func(*providerConfig) error

// WithClock 은 provider 생성, Compose 기본 시각, rotation에 사용할 clock을 지정한다.
func WithClock(now func() time.Time) ProviderOption {
	return func(cfg *providerConfig) error {
		if now == nil {
			return OptionError{Option: "clock", Err: errorsNew("must not be nil")}
		}
		cfg.now = now
		return nil
	}
}

// WithEntropy 는 key 생성과 kid 생성에 사용할 entropy reader를 지정한다.
func WithEntropy(entropy io.Reader) ProviderOption {
	return func(cfg *providerConfig) error {
		if entropy == nil {
			return OptionError{Option: "entropy", Err: errorsNew("must not be nil")}
		}
		cfg.entropy = entropy
		return nil
	}
}

// WithKeyTTL 은 회전 KeyChain의 TTL을 지정한다.
func WithKeyTTL(ttl time.Duration) ProviderOption {
	return func(cfg *providerConfig) error {
		if ttl <= 0 {
			return OptionError{Option: "key_ttl", Err: errorsNew("must be positive")}
		}
		cfg.keyTTL = ttl
		return nil
	}
}

// WithRepositoryCapacity 는 in-memory KeyChain 보존 개수를 지정한다.
func WithRepositoryCapacity(capacity int) ProviderOption {
	return func(cfg *providerConfig) error {
		cfg.capacity = capacity
		return nil
	}
}

// WithKeyIDGenerator 는 custom kid 생성 함수를 지정한다.
//
// 반환 값은 provider 안에서 unique해야 하며, 공유 provider에서 사용할 때는
// generator 자체가 concurrent use에 안전해야 한다.
func WithKeyIDGenerator(keyID func() (string, error)) ProviderOption {
	return func(cfg *providerConfig) error {
		if keyID == nil {
			return OptionError{Option: "key_id", Err: errorsNew("must not be nil")}
		}
		cfg.keyID = keyID
		return nil
	}
}

// WithRSAKeyBits 는 생성 RSA key 크기를 지정한다.
func WithRSAKeyBits(bits int) ProviderOption {
	return func(cfg *providerConfig) error {
		if bits < 2048 {
			return OptionError{Option: "rsa_key_bits", Err: errorsNew("must be at least 2048")}
		}
		cfg.rsaKeyBits = bits
		return nil
	}
}

func normalizeProviderConfig(options []ProviderOption) (providerConfig, error) {
	cfg := providerConfig{
		now:        time.Now,
		keyTTL:     defaultKeyTTL,
		capacity:   defaultRepositorySize,
		rsaKeyBits: defaultRSAKeyBits,
	}
	for _, option := range options {
		if option == nil {
			return cfg, OptionError{Option: "option", Err: errorsNew("must not be nil")}
		}
		if err := option(&cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}
