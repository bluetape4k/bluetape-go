package rediscoord

import "encoding/json"

// Codec 은 Redis result envelope payload를 변환한다.
type Codec[V any] interface {
	Marshal(V) ([]byte, error)
	Unmarshal([]byte) (V, error)
}

// JSONCodec 은 JSON payload codec이다.
type JSONCodec[V any] struct{}

// Marshal 은 value를 JSON으로 인코딩한다.
func (JSONCodec[V]) Marshal(value V) ([]byte, error) {
	return json.Marshal(value)
}

// Unmarshal 은 JSON payload를 value로 디코딩한다.
func (JSONCodec[V]) Unmarshal(payload []byte) (V, error) {
	var value V
	err := json.Unmarshal(payload, &value)
	return value, err
}
