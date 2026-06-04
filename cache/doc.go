// Package cache 는 TTL과 로더 중복 실행 억제를 제공하는 작은 캐시 계약이다.
//
// Cache miss는 ErrCacheMiss로 보고되어 errors.Is로 확인할 수 있다. TTL이 0이면
// 만료되지 않고, 양수 TTL은 저장 시점 기준으로 만료된다. 음수 TTL은 거부된다.
//
// LoadingCache는 같은 key의 동시 GetOrLoad 호출을 한 번의 로더 실행으로 합친다.
// 이 보장은 한 캐시 인스턴스 안의 in-flight 호출에만 적용된다. Redis invalidation
// 이나 cross-process stampede protection은 별도 backend가 담당해야 한다.
package cache
