// Package redisleader Redis 기반 leader election을 제공한다.
//
// 여러 backend replica가 같은 batch, migration, polling 작업을 중복 실행하지
// 않도록 하나의 group 안에서 하나의 member만 leader가 되게 한다. 실제 Redis 기반
// 사용 예제는 coordination example test로 검증한다. GroupElector는 Redis ZSET으로
// 최대 N개의 member가 동시에 leader slot을 보유하게 한다. StrategicElector는 Redis
// candidate registry와 deterministic strategy로 winner node만 action을 실행한다.
//
// 이 주석은 backend lease, ownership, consistency, cancellation 조건을 설명한다.
// `bluetape:leader:<group>` key에 `memberID:random` token을 저장하고 TTL을
// 갱신한다. Go group elector는 `bluetape:leader-group:<group>` ZSET에
// `memberID:random` token과 만료 score를 저장한다. Go strategic elector는
// `bluetape:leader-strategy:<group>` 아래 candidate JSON과 live index ZSET을
// 저장한다. Kotlin/JVM bluetape4k-leader의 Lettuce/Redisson backend와 같은 Redis
// key를 공유하는 혼합 참가자는 지원하지 않는다.
package redisleader
