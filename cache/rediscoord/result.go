package rediscoord

import (
	"encoding/json"
	"errors"
	"fmt"
)

const resultVersion = 1

// ErrResultTooLarge reports that an encoded coordination result exceeded its configured bound.
var ErrResultTooLarge = errors.New("coordination result exceeds maximum size")

type resultEnvelope struct {
	Version int    `json:"version"`
	Token   string `json:"token"`
	Payload []byte `json:"payload"`
}

func encodeResult(token string, payload []byte) ([]byte, error) {
	if token == "" {
		return nil, fmt.Errorf("result token must not be empty")
	}
	if payload == nil {
		return nil, fmt.Errorf("result payload must not be nil")
	}
	return json.Marshal(resultEnvelope{
		Version: resultVersion,
		Token:   token,
		Payload: payload,
	})
}

func decodeResult(encoded []byte, expectedToken string) ([]byte, bool, error) {
	if expectedToken == "" {
		return nil, false, nil
	}

	var envelope resultEnvelope
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		return nil, false, err
	}
	if envelope.Version != resultVersion || envelope.Token != expectedToken || envelope.Payload == nil {
		return nil, false, nil
	}
	return envelope.Payload, true, nil
}
