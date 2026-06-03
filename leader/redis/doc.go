// Package redisleader 는 Redis 기반 leader election을 제공한다.
//
// 여러 backend replica가 같은 batch, migration, polling 작업을 중복 실행하지
// 않도록 하나의 group 안에서 하나의 member만 leader가 되게 한다. 실제 Redis 기반
// 사용 예제는 coordination example test로 검증한다. GroupElector는 Redis ZSET으로
// 최대 N개의 member가 동시에 leader slot을 보유하게 한다.
//
// Redis key 형식은 Go 전용이다. Go elector는
// `bluetape:leader:<group>` key에 `memberID:random` token을 저장하고 TTL을
// 갱신한다. Go group elector는 `bluetape:leader-group:<group>` ZSET에
// `memberID:random` token과 만료 score를 저장한다. Kotlin/JVM bluetape4k-leader의
// Lettuce/Redisson backend와 같은 Redis key를 공유하는 혼합 참가자는 지원하지 않는다.
package redisleader
