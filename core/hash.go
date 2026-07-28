package core

import "github.com/cespare/xxhash/v2"

// XXH64Bytes는 XXH64Bytes 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: XXH64Bytes가 읽거나 복사하는 value 목록이다. nil과 빈 슬라이스 의미는 함수 계약을 따른다.
func XXH64Bytes(value []byte) uint64 {
	return xxhash.Sum64(value)
}

// XXH64String는 XXH64String 공개 API의 동작을 수행한다.
//
// 매개변수:
//   - value: XXH64String가 해석하거나 검증하는 문자열 값이다. 빈 문자열과 공백 처리 의미는 함수 계약을 따른다.
func XXH64String(value string) uint64 {
	return xxhash.Sum64String(value)
}
