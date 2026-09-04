// Package redismap caller-owned Redis key-per-entry map primitive을 제공한다.
//
// MapCache는 caller가 직렬화한 entry를 독립 TTL로 저장하며 Redis client, retry
// policy, persistence, eviction 또는 local invalidation을 소유하지 않는다.
package redismap
