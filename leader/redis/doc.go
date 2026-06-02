// Package redisleader 는 Redis 기반 leader election을 제공한다.
//
// 여러 backend replica가 같은 batch, migration, polling 작업을 중복 실행하지
// 않도록 하나의 group 안에서 하나의 member만 leader가 되게 한다. 실제 Redis 기반
// 사용 예제는 coordination example test로 검증한다.
package redisleader
