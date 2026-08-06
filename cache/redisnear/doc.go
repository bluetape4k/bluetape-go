// Package redisnear Redis Pub/Sub 기반 near-cache invalidation을 제공한다.
//
// 이 패키지는 Redis를 값 저장소로 사용하지 않는다. 각 process는 local
// LoadingCache를 유지하고, peer의 Set/Delete/Clear 알림을 받으면 local entry를
// 제거한다.
package redisnear
