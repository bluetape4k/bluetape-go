// Package redislock 는 Redis 단일 인스턴스 기반 owner-token lock을 제공한다.
//
// Lock 획득은 Redis SET NX + TTL로 수행하고, 해제는 저장된 token이 lease token과
// 같을 때만 DEL하는 Lua script로 수행한다. 이 package는 Redlock quorum, fencing
// token, blocking retry를 제공하지 않는다.
package redislock
