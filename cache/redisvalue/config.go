package redisvalue

import (
	"fmt"
	"strings"
	"time"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

const (
	maxNamespaceBytes  = 128
	maxLogicalKeyBytes = 1024
	maxValueBytes      = 64 << 20
	maxClearBatchSize  = 1000
)

// ValueConfig configures serialized Redis L2 storage.
type ValueConfig struct {
	RemoteTTL      time.Duration
	MaxValueBytes  int
	ClearBatchSize int64
}

// TieredConfig configures process-local L1 behavior around a ValueCache.
type TieredConfig struct {
	LocalTTL                time.Duration
	InvalidationWaitTimeout time.Duration
	LocalCleanupTimeout     time.Duration
}

// Config contains the defaultable ValueCache and TieredCache configuration.
type Config struct {
	Value  ValueConfig
	Tiered TieredConfig
}

// DefaultConfig returns an independent configuration value with safe defaults.
func DefaultConfig() Config {
	return Config{
		Value: ValueConfig{
			RemoteTTL:      time.Hour,
			MaxValueBytes:  1 << 20,
			ClearBatchSize: 100,
		},
		Tiered: TieredConfig{
			LocalTTL:                30 * time.Minute,
			InvalidationWaitTimeout: 30 * time.Second,
			LocalCleanupTimeout:     time.Second,
		},
	}
}

// Validate verifies all configured bounds and the default cross-tier TTL
// relationship.
func (c Config) Validate() error {
	if err := validateValueConfig(c.Value); err != nil {
		return err
	}
	if err := validateTieredConfig(c.Tiered); err != nil {
		return err
	}
	if c.Value.RemoteTTL > 0 && c.Tiered.LocalTTL > c.Value.RemoteTTL {
		return newCacheError(
			"validate-config",
			ReasonConfiguration,
			"",
			fmt.Errorf("local ttl exceeds remote ttl: %w", btredis.ErrInvalidTTL),
		)
	}
	return nil
}

func validateValueConfig(config ValueConfig) error {
	if err := validateEntryTTL(config.RemoteTTL); err != nil {
		return newCacheError("validate-config", ReasonConfiguration, "", err)
	}
	if config.MaxValueBytes < 1 || config.MaxValueBytes > maxValueBytes {
		return newCacheError(
			"validate-config",
			ReasonConfiguration,
			"",
			fmt.Errorf("max value bytes must be between 1 and %d", maxValueBytes),
		)
	}
	if config.ClearBatchSize < 1 || config.ClearBatchSize > maxClearBatchSize {
		return newCacheError(
			"validate-config",
			ReasonConfiguration,
			"",
			fmt.Errorf("clear batch size must be between 1 and %d", maxClearBatchSize),
		)
	}
	return nil
}

func validateTieredConfig(config TieredConfig) error {
	if config.LocalTTL <= 0 {
		return invalidPositiveDuration("local ttl")
	}
	if config.InvalidationWaitTimeout <= 0 {
		return invalidPositiveDuration("invalidation wait timeout")
	}
	if config.LocalCleanupTimeout <= 0 {
		return invalidPositiveDuration("local cleanup timeout")
	}
	return nil
}

func invalidPositiveDuration(name string) error {
	return newCacheError(
		"validate-config",
		ReasonConfiguration,
		"",
		fmt.Errorf("%s must be positive: %w", name, btredis.ErrInvalidTTL),
	)
}

func validateNamespace(namespace string) error {
	if len(namespace) == 0 || len(namespace) > maxNamespaceBytes {
		return fmt.Errorf("%w: invalid namespace", btredis.ErrInvalidKey)
	}
	for i := 0; i < len(namespace); i++ {
		character := namespace[i]
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-') {
			return fmt.Errorf("%w: invalid namespace", btredis.ErrInvalidKey)
		}
	}
	return nil
}

func validateLogicalKey(key string) error {
	if len(key) == 0 || len(key) > maxLogicalKeyBytes || strings.TrimSpace(key) == "" {
		return fmt.Errorf("%w: invalid logical key", btredis.ErrInvalidKey)
	}
	return nil
}

func newValueKeyBuilder(namespace string) (btredis.KeyBuilder, error) {
	if err := validateNamespace(namespace); err != nil {
		return btredis.KeyBuilder{}, err
	}
	builder, err := btredis.NewKeyBuilder("bluetape:cache:value")
	if err != nil {
		return btredis.KeyBuilder{}, err
	}
	return builder.Structural(namespace)
}
