package redisbloom

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

const keyPrefix = "bluetape:probabilistic:bloom:v1"

type redisKeys struct {
	slot       string
	bits       string
	config     string
	redactedID string
}

func buildKeys(namespace string) (redisKeys, error) {
	if err := validateNamespace(namespace); err != nil {
		return redisKeys{}, err
	}
	slot := fmt.Sprintf("%s:{%s}", keyPrefix, namespace)
	sum := sha256.Sum256([]byte(slot))
	return redisKeys{
		slot:       slot,
		bits:       slot + ":bits",
		config:     slot + ":config",
		redactedID: "redis-key:" + hex.EncodeToString(sum[:6]),
	}, nil
}
