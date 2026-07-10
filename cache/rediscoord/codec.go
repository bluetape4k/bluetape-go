package rediscoord

import "encoding/json"

// Codec transforms Redis result-envelope payloads.
type Codec[V any] interface {
	Marshal(V) ([]byte, error)
	Unmarshal([]byte) (V, error)
}

// JSONCodec encodes and decodes payloads as JSON.
type JSONCodec[V any] struct{}

// Marshal encodes a value as JSON.
func (JSONCodec[V]) Marshal(value V) ([]byte, error) {
	return json.Marshal(value)
}

// Unmarshal decodes a JSON payload into a value.
func (JSONCodec[V]) Unmarshal(payload []byte) (V, error) {
	var value V
	err := json.Unmarshal(payload, &value)
	return value, err
}
