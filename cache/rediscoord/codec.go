package rediscoord

import "encoding/json"

// Codec는 interface 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type Codec[V any] interface {
	Marshal(V) ([]byte, error)
	Unmarshal([]byte) (V, error)
}

// JSONCodec는 struct 공개 타입이며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
// 필드와 zero value, nil 허용 여부, 동시성 소유권은 생성자와 메서드의 한국어 주석 및 테스트 계약을 따른다.
type JSONCodec[V any] struct{}

// Marshal는 Marshal 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - value: 직렬화하거나 cache에 보관할 값이다. nil, zero value, aliasing 의미는 serializer/cache 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (JSONCodec[V]) Marshal(value V) ([]byte, error) {
	return json.Marshal(value)
}

// Unmarshal는 Unmarshal 공개 API의 동작을 수행하며 Redis 조정, stampede 방지, codec envelope 계약을 보존한다.
//
// 매개변수:
//   - payload: Unmarshal 동작에 필요한 payload 값이다. zero value, 범위, nil 허용 여부는 함수 계약을 따른다.
//
// 반환 오류는 cache miss, 입력 검증 실패, 취소, Redis/backend 실패, 또는 package sentinel/typed error 계약을 보존한다.
func (JSONCodec[V]) Unmarshal(payload []byte) (V, error) {
	var value V
	err := json.Unmarshal(payload, &value)
	return value, err
}
