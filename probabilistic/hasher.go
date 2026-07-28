package probabilistic

import "fmt"

const (
	stringHasherKey = "probabilistic:string:v1"
	bytesHasherKey  = "probabilistic:bytes:v1"
)

// Hasher는 struct 공개 타입이며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Hasher[T any] struct {
	key string
	sum func(T) []byte
}

// NewHasher는 NewHasher 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - key: 저장소 또는 Redis filter를 식별하는 key다. namespace와 compatibility 의미는 package 계약을 따른다.
//   - sum: NewHasher 동작에 필요한 sum 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func NewHasher[T any](key string, sum func(T) []byte) (Hasher[T], error) {
	if key == "" {
		return Hasher[T]{}, ErrEmptyHasherKey
	}
	if sum == nil {
		return Hasher[T]{}, ErrNilHasher
	}
	return Hasher[T]{key: key, sum: sum}, nil
}

// Key는 Key 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
func (h Hasher[T]) Key() string {
	return h.key
}

func (h Hasher[T]) validate() error {
	if h.key == "" {
		return ErrEmptyHasherKey
	}
	if h.sum == nil {
		return ErrNilHasher
	}
	return nil
}

// Bytes는 Bytes 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - value: Bloom/Redis filter에 추가하거나 검사할 값이다. nil/empty/hash input 의미는 hasher 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (h Hasher[T]) Bytes(value T) ([]byte, error) {
	if err := h.validate(); err != nil {
		return nil, err
	}
	return h.sum(value), nil
}

func (h Hasher[T]) bytes(value T) ([]byte, error) {
	return h.Bytes(value)
}

func stringHasher() Hasher[string] {
	hasher, err := NewHasher(stringHasherKey, func(value string) []byte {
		return []byte(value)
	})
	if err != nil {
		panic(fmt.Sprintf("invalid built-in string hasher: %v", err))
	}
	return hasher
}

func bytesHasher() Hasher[[]byte] {
	hasher, err := NewHasher(bytesHasherKey, func(value []byte) []byte {
		copied := make([]byte, len(value))
		copy(copied, value)
		return copied
	})
	if err != nil {
		panic(fmt.Sprintf("invalid built-in bytes hasher: %v", err))
	}
	return hasher
}
