package redislock

import (
	"fmt"
	"strings"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

type keySet struct {
	owner   string
	counter string
	keyID   string
}

func buildKeys(logicalKey string) (keySet, error) {
	if strings.TrimSpace(logicalKey) == "" {
		return keySet{}, fmt.Errorf("%w: lock key", btredis.ErrInvalidKey)
	}
	builder, err := btredis.NewKeyBuilder("bluetape:redis:lock")
	if err != nil {
		return keySet{}, err
	}
	keyID := btredis.RedactedKeyID(logicalKey)
	builder, err = builder.WithHashTag(keyID)
	if err != nil {
		return keySet{}, err
	}
	owner, err := builder.StructuralKey("owner")
	if err != nil {
		return keySet{}, err
	}
	counter, err := builder.StructuralKey("counter")
	if err != nil {
		return keySet{}, err
	}
	return keySet{owner: owner.Value, counter: counter.Value, keyID: keyID}, nil
}
