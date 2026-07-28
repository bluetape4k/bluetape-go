package cache

import "errors"

// ErrCacheMiss 캐시에 값이 없거나 만료됐을 때 반환된다.
var ErrCacheMiss = errors.New("cache miss")
