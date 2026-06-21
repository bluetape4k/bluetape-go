package jwt

import "time"

type parseConfig struct {
	leeway             time.Duration
	expectedIssuer     string
	expectedAudience   []string
	expectedSubject    string
	expirationRequired bool
	now                func() time.Time
	customClock        bool
}

// ParseOption 은 JWT parse와 claim 검증 option이다.
type ParseOption func(*parseConfig) error

// WithLeeway 는 exp/nbf/iat 검증 여유 시간을 지정한다.
func WithLeeway(leeway time.Duration) ParseOption {
	return func(cfg *parseConfig) error {
		if leeway < 0 {
			return OptionError{Option: "leeway", Err: errorsNew("must not be negative")}
		}
		cfg.leeway = leeway
		return nil
	}
}

// WithExpectedIssuer 는 기대 issuer를 지정한다.
func WithExpectedIssuer(issuer string) ParseOption {
	return func(cfg *parseConfig) error {
		cfg.expectedIssuer = issuer
		return nil
	}
}

// WithExpectedAudience 는 기대 audience를 지정한다.
func WithExpectedAudience(audience ...string) ParseOption {
	return func(cfg *parseConfig) error {
		cfg.expectedAudience = append([]string(nil), audience...)
		return nil
	}
}

// WithExpectedSubject 는 기대 subject를 지정한다.
func WithExpectedSubject(subject string) ParseOption {
	return func(cfg *parseConfig) error {
		cfg.expectedSubject = subject
		return nil
	}
}

// WithExpirationRequired 는 exp claim을 필수로 만든다.
func WithExpirationRequired() ParseOption {
	return func(cfg *parseConfig) error {
		cfg.expirationRequired = true
		return nil
	}
}

// WithParseClock 은 parse 검증 clock을 지정한다.
func WithParseClock(now func() time.Time) ParseOption {
	return func(cfg *parseConfig) error {
		if now == nil {
			return OptionError{Option: "parse_clock", Err: errorsNew("must not be nil")}
		}
		cfg.now = now
		cfg.customClock = true
		return nil
	}
}

func normalizeParseConfig(defaultNow func() time.Time, options []ParseOption) (parseConfig, error) {
	cfg := parseConfig{now: defaultNow}
	for _, option := range options {
		if option == nil {
			return cfg, OptionError{Option: "parse_option", Err: errorsNew("must not be nil")}
		}
		if err := option(&cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}
