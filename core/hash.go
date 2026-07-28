package core

import "github.com/cespare/xxhash/v2"

// XXH64Bytes 입력의 XXH64 hash 값을 반환한다.
//
// 매개변수:
//   - value: XXH64Bytes가 처리할 값 목록이다. nil과 빈 슬라이스는 구현의 입력 규칙에 따라 처리한다.
func XXH64Bytes(value []byte) uint64 {
	return xxhash.Sum64(value)
}

// XXH64String 입력의 XXH64 hash 값을 반환한다.
//
// 매개변수:
//   - value: XXH64String가 해석할 문자열이다. 빈 문자열과 공백은 구현 검증을 따른다.
func XXH64String(value string) uint64 {
	return xxhash.Sum64String(value)
}
