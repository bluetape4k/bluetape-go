package rediscoord

import "encoding/json"

// Codec interface 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Codec[V any] interface {
	Marshal(V) ([]byte, error)
	Unmarshal([]byte) (V, error)
}

// JSONCodec struct 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type JSONCodec[V any] struct{}

// Marshal Marshal 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, context 취소, Redis/backend 실패, package sentinel error와 typed error를 그대로 드러낸다.
func (JSONCodec[V]) Marshal(value V) ([]byte, error) {
	return json.Marshal(value)
}

// Unmarshal Unmarshal 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
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
