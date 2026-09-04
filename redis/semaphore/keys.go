package redissem

import (
	"fmt"
	"strings"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

type keySet struct {
	leases string
	keyID  string
}

func buildKeys(logicalKey string) (keySet, error) {
	if strings.TrimSpace(logicalKey) == "" {
		return keySet{}, fmt.Errorf("%w: semaphore key", btredis.ErrInvalidKey)
	}
	builder, err := btredis.NewKeyBuilder("bluetape:redis:semaphore")
	if err != nil {
		return keySet{}, err
	}
	keyID := btredis.RedactedKeyID(logicalKey)
	builder, err = builder.WithHashTag(keyID)
	if err != nil {
		return keySet{}, err
	}
	leases, err := builder.StructuralKey("leases")
	if err != nil {
		return keySet{}, err
	}
	return keySet{leases: leases.Value, keyID: keyID}, nil
}
