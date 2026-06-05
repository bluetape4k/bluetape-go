// Package rediscoord 는 Redis 기반 cache load 조정을 제공한다.
//
// 이 패키지는 durable Redis cache가 아니다. 기존 LoadingCache를 감싸고,
// cold miss burst 동안 한 process의 loader 결과를 짧게 공유해 다른 process가
// 같은 loader를 동시에 실행하지 않게 한다.
package rediscoord
