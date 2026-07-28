package probabilistic

import "fmt"

const (
	stringHasherKey = "probabilistic:string:v1"
	bytesHasherKey  = "probabilistic:bytes:v1"
)

// Hasher Bloom filter 값에서 stable hash 입력 bytes를 만드는 전략입니다.
//
// Custom hasher 함수는 deterministic하고 goroutine-safe해야 합니다.
type Hasher[T any] struct {
	key string
	sum func(T) []byte
}

// NewHasher compatibility key와 hash 입력 함수를 가진 Hasher를 만듭니다.
// 같은 key를 가진 Hasher는 PutAll에서 호환된다고 간주되므로, caller는 key와 함수
// 구현의 의미를 stable하게 유지해야 합니다.
func NewHasher[T any](key string, sum func(T) []byte) (Hasher[T], error) {
	if key == "" {
		return Hasher[T]{}, ErrEmptyHasherKey
	}
	if sum == nil {
		return Hasher[T]{}, ErrNilHasher
	}
	return Hasher[T]{key: key, sum: sum}, nil
}

// Key PutAll 호환성 판단에 사용하는 stable hasher key를 반환합니다.
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

// Bytes Bytes 공개 API의 동작을 수행하며 Bloom filter의 capacity, false-positive rate, hasher, compatibility 계약을 보존한다.
//
// 매개변수:
//   - value: Bloom/Redis filter에 추가하거나 검사할 값이다. nil/empty/hash input 의미는 hasher 계약을 따른다.
//
// 반환 오류는 입력 검증 실패, compatibility 불일치, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
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
