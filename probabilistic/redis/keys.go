package redisbloom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	if err := validateNamespace(namespace); err != nil {
		return redisKeys{}, err
	}
	slot := fmt.Sprintf("%s:{%s}", keyPrefix, namespace)
	return redisKeys{
		slot:       slot,
		bits:       slot + ":bits",
		config:     slot + ":config",
		redactedID: redactedRedisKeyID(slot),
	}, nil
}

func buildHyperLogLogKey(namespace string) (hyperLogLogKey, error) {
	if err := validateNamespace(namespace); err != nil {
		return hyperLogLogKey{}, err
	}
	key := fmt.Sprintf("%s:{%s}", hyperLogLogPrefix, namespace)
	return hyperLogLogKey{
		key:        key,
		redactedID: redactedRedisKeyID(key),
	}, nil
}

func redactedRedisKeyID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "redis-key:" + hex.EncodeToString(sum[:6])
}
