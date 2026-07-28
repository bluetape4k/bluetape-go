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

// ValueConfig struct 공개 타입이며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type ValueConfig struct {
	// RemoteTTL 기본 Redis expiry다. zero는 항목 영속화를 뜻하고 음수는 invalid다.
	RemoteTTL time.Duration
	// MaxValueBytes serialized value 허용 상한이며 1 byte부터 64 MiB까지 허용한다.
	MaxValueBytes int
	// ClearBatchSize SCAN COUNT hint이자 한 번의 UNLINK argument 수 상한이다.
	ClearBatchSize int64
}

// TieredConfig struct 공개 타입이며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type TieredConfig struct {
	// LocalTTL 양수 L1 expiry 상한이며 finite RemoteTTL을 초과하면 안 된다.
	LocalTTL time.Duration
	// InvalidationWaitTimeout 같은 key의 active work 대기 시간을 제한한다.
	InvalidationWaitTimeout time.Duration
	// LocalCleanupTimeout 필수/명시적 L1 delete 또는 clear 작업 시간을 제한한다.
	LocalCleanupTimeout time.Duration
}

// Config struct 공개 타입이며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Config struct {
	// Value serialized Redis L2 설정을 담는다.
	Value ValueConfig
	// Tiered 선택적 process-local L1 decorator 설정을 담는다.
	Tiered TieredConfig
}

// DefaultConfig DefaultConfig 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
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

// Validate Validate 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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
		if (character < 'a' || character > 'z') &&
			(character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') &&
			character != '.' && character != '_' && character != '-' {
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
