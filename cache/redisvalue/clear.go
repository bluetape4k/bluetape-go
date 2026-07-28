package redisvalue

import (
	"context"

	btredis "github.com/bluetape4k/bluetape-go/redis"
)

// Clear Clear 공개 API의 동작을 수행하며 tiered Redis value cache의 local/remote ownership, TTL, clear coordination 계약을 보존한다.
//
// 매개변수:
//   - ctx: 호출자가 소유한 취소, deadline, 요청 범위를 전달한다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (c *ValueCache[V]) Clear(ctx context.Context) error {
	ctx = normalizeContext(ctx)
	if err := c.validateInitialized("clear"); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	pattern := "bluetape:cache:value:" + c.namespace + ":*"
	patternID := btredis.RedactedKeyID(pattern)
	progress := ClearProgress{}
	var cursor uint64
	for {
		if err := ctx.Err(); err != nil {
			return newPartialClearError("clear", progress, err)
		}
		keys, next, err := c.client.Scan(ctx, cursor, pattern, c.config.ClearBatchSize).Result()
		if err != nil {
			return newPartialClearError("clear", progress, c.operationError("scan", patternID, err, false))
		}
		progress.ScannedKeys += int64(len(keys))
		for start := 0; start < len(keys); start += int(c.config.ClearBatchSize) {
			if err := ctx.Err(); err != nil {
				return newPartialClearError("clear", progress, err)
			}
			end := min(start+int(c.config.ClearBatchSize), len(keys))
			if err := c.client.Unlink(ctx, keys[start:end]...).Err(); err != nil {
				return newPartialClearError("clear", progress, c.operationError("unlink", patternID, err, true))
			}
			progress.UnlinkedBatches++
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
