package redisbloom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/bluetape4k/bluetape-go/probabilistic"
	"github.com/redis/go-redis/v9"
)

const (
	configVersion = "1"
	configFamily  = "redis-bloom"
)

type metadata struct {
	expectedInsertions       uint64
	falsePositiveProbability float64
	bitSize                  uint64
	hashFunctionCount        uint64
	hasherKey                string
	fingerprint              string
}

func newMetadata(cfg probabilistic.Config, hasherKey string) metadata {
	fpInput := fmt.Sprintf("%s|%s|%d|%g|%d|%d|%s",
		configVersion,
		configFamily,
		cfg.ExpectedInsertions(),
		cfg.FalsePositiveProbability(),
		cfg.BitSize(),
		cfg.HashFunctionCount(),
		hasherKey,
	)
	sum := sha256.Sum256([]byte(fpInput))
	return metadata{
		expectedInsertions:       cfg.ExpectedInsertions(),
		falsePositiveProbability: cfg.FalsePositiveProbability(),
		bitSize:                  cfg.BitSize(),
		hashFunctionCount:        cfg.HashFunctionCount(),
		hasherKey:                hasherKey,
		fingerprint:              hex.EncodeToString(sum[:]),
	}
}

func (m metadata) argv() []any {
	return []any{
		configVersion,
		configFamily,
		strconv.FormatUint(m.expectedInsertions, 10),
		strconv.FormatFloat(m.falsePositiveProbability, 'g', -1, 64),
		strconv.FormatUint(m.bitSize, 10),
		strconv.FormatUint(m.hashFunctionCount, 10),
		m.hasherKey,
		m.fingerprint,
	}
}

func initializeConfig(ctx context.Context, client redis.Cmdable, keys redisKeys, meta metadata) error {
	if err := initConfigScript.Run(normalizeContext(ctx), client, []string{keys.bits, keys.config}, meta.argv()...).Err(); err != nil {
		return mapScriptError("init", keys.redactedID, err)
	}
	return nil
}
