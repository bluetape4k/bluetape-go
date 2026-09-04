// Package redisbucket caller-owned Redis single-key value primitive을 제공한다.
//
// Bucket은 caller가 직렬화한 payload를 저장하며 Redis client, retry policy,
// persistence, eviction 또는 local cache invalidation을 소유하지 않는다.
package redisbucket
