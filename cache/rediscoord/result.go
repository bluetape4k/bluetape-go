package rediscoord

import (
	"encoding/base64"
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

func encodedResultSize(token string, payload []byte) (int, error) {
	if token == "" {
		return 0, fmt.Errorf("result token must not be empty")
	}
	if payload == nil {
		return 0, fmt.Errorf("result payload must not be nil")
	}
	quotedToken, err := json.Marshal(token)
	if err != nil {
		return 0, err
	}
	size := uint64(len(`{"version":1,"token":`)) + uint64(len(quotedToken)) +
		uint64(len(`,"payload":"`)) + uint64(base64.StdEncoding.EncodedLen(len(payload))) + uint64(len(`"}`))
	if size > uint64(^uint(0)>>1) {
		return 0, ErrResultTooLarge
	}
	return int(size), nil
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
