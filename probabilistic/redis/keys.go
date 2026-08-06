package redisbloom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

const (
	keyPrefix         = "bluetape:probabilistic:bloom:v1"
	hyperLogLogPrefix = "bluetape:probabilistic:hll:v1"
)

type redisKeys struct {
	slot       string
	bits       string
	config     string
	redactedID string
}

type hyperLogLogKey struct {
	key        string
	redactedID string
}

func buildKeys(namespace string) (redisKeys, error) {
	builder, err := keyBuilderForNamespace(keyPrefix, namespace)
	if err != nil {
		return redisKeys{}, err
	}
	slot, err := structuralKeyValue(builder)
	if err != nil {
		return redisKeys{}, err
	}
	bits, err := structuralKeyValue(builder, "bits")
	if err != nil {
		return redisKeys{}, err
	}
	config, err := structuralKeyValue(builder, "config")
	if err != nil {
		return redisKeys{}, err
	}
	return redisKeys{
		slot:       slot,
		bits:       bits,
		config:     config,
		redactedID: redactedRedisKeyID(slot),
	}, nil
}

func buildHyperLogLogKey(namespace string) (hyperLogLogKey, error) {
	builder, err := keyBuilderForNamespace(hyperLogLogPrefix, namespace)
	if err != nil {
		return hyperLogLogKey{}, err
	}
	key, err := structuralKeyValue(builder)
	if err != nil {
		return hyperLogLogKey{}, err
	}
	return hyperLogLogKey{
		key:        key,
		redactedID: redactedRedisKeyID(key),
	}, nil
}

func keyBuilderForNamespace(prefix string, namespace string) (btredis.KeyBuilder, error) {
	if err := validateNamespace(namespace); err != nil {
		return btredis.KeyBuilder{}, err
	}
	builder, err := btredis.NewKeyBuilder(prefix)
	if err != nil {
		return btredis.KeyBuilder{}, keyBuilderConfigurationError()
	}
	builder, err = builder.WithHashTag(namespace)
	if err != nil {
		return btredis.KeyBuilder{}, keyBuilderConfigurationError()
	}
	return builder, nil
}

func structuralKeyValue(builder btredis.KeyBuilder, parts ...string) (string, error) {
	key, err := builder.StructuralKey(parts...)
	if err != nil {
		return "", keyBuilderConfigurationError()
	}
	return key.Value, nil
}

func keyBuilderConfigurationError() error {
	return fmt.Errorf("redis probabilistic: invalid key builder configuration")
}

func redactedRedisKeyID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "redis-key:" + hex.EncodeToString(sum[:6])
}
