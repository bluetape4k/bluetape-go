package rediscoord

import "encoding/json"

// Codec Redis 조정, stampede 방지, codec envelope에서 사용하는 인터페이스이다.
type Codec[V any] interface {
	Marshal(V) ([]byte, error)
	Unmarshal([]byte) (V, error)
}

// JSONCodec Redis 조정, stampede 방지, codec envelope에서 사용하는 구조체다.
type JSONCodec[V any] struct{}

// Marshal Redis 조정, stampede 방지, codec envelope 값을 직렬화한다.
//
// 매개변수:
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (JSONCodec[V]) Marshal(value V) ([]byte, error) {
	return json.Marshal(value)
}

// Unmarshal Redis 조정, stampede 방지, codec envelope 값을 복원한다.
//
// 매개변수:
//   - payload: Unmarshal에 전달되는 값이다. 허용 범위와 nil 처리 방식은 구현 검증을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (JSONCodec[V]) Unmarshal(payload []byte) (V, error) {
	var value V
	err := json.Unmarshal(payload, &value)
	return value, err
}
