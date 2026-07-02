package neo4j

import "strings"

// Option configures a Client.
type Option func(*config) error

type config struct {
	database string
}

// WithDatabase sets the Neo4j database name used by query helpers.
func WithDatabase(name string) Option {
	return func(cfg *config) error {
		normalized := strings.TrimSpace(name)
		if normalized == "" {
			return errorWith(ErrInvalidOptions, "configure database", nil)
		}
		cfg.database = normalized
		return nil
	}
}

func applyOptions(options []Option) (config, error) {
	var cfg config
	for _, option := range options {
		if option == nil {
			return config{}, errorWith(ErrInvalidOptions, "apply option", nil)
		}
		if err := option(&cfg); err != nil {
			return config{}, err
		}
	}
	return cfg, nil
}
