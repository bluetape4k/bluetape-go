package gremlin

import (
	"strings"
	"time"

	gremlingo "github.com/apache/tinkerpop/gremlin-go/v3/driver"
)

const (
	defaultMaxResults = 1024
	maxQueryBytes     = 1 << 20
	maxTimeout        = 10 * time.Minute
)

type config struct {
	maxResults int
	timeout    time.Duration
	connection []func(*gremlingo.DriverRemoteConnectionSettings)
}

// Option gremlin remote adapter의 caller-owned 설정을 조정한다.
type Option func(*config) error

// WithMaxResults는 한 query가 materialize할 최대 result 수를 설정한다.
func WithMaxResults(limit int) Option {
	return func(cfg *config) error {
		if limit < 1 || limit > defaultMaxResults {
			return invalid("max results must be between 1 and 1024")
		}
		cfg.maxResults = limit
		return nil
	}
}

// WithTimeout은 remote request의 evaluation timeout과 local collection 상한을 설정한다.
func WithTimeout(timeout time.Duration) Option {
	return func(cfg *config) error {
		if timeout <= 0 || timeout > maxTimeout {
			return invalid("timeout is outside the supported range")
		}
		cfg.timeout = timeout
		return nil
	}
}

// WithConnectionConfiguration은 공식 Gremlin-Go connection 설정을 caller에게 위임한다.
func WithConnectionConfiguration(configuration func(*gremlingo.DriverRemoteConnectionSettings)) Option {
	return func(cfg *config) error {
		if configuration == nil {
			return invalid("connection configuration is nil")
		}
		cfg.connection = append(cfg.connection, configuration)
		return nil
	}
}

func applyOptions(options []Option) (config, error) {
	cfg := config{maxResults: defaultMaxResults}
	for _, option := range options {
		if option == nil {
			return config{}, invalid("option is nil")
		}
		if err := option(&cfg); err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}

func normalizeBindings(bindings []map[string]any) (map[string]any, error) {
	if len(bindings) > 1 {
		return nil, classified(ErrInvalidQuery, "multiple binding maps", nil)
	}
	if len(bindings) == 0 || bindings[0] == nil {
		return nil, nil
	}
	copy := make(map[string]any, len(bindings[0]))
	for key, value := range bindings[0] {
		if strings.TrimSpace(key) == "" {
			return nil, classified(ErrInvalidQuery, "blank binding name", nil)
		}
		copy[key] = value
	}
	return copy, nil
}
