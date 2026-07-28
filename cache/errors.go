package cache

import "errors"

// ErrCacheMiss 변수 공개 값이며 cache key, miss, TTL, serialization 계약을 보존한다.
// 호출자는 이 식별자를 cache 오류, 옵션, event, 또는 기본값 계약을 비교할 때 사용한다.
var ErrCacheMiss = errors.New("cache miss")
