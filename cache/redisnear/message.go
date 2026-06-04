package redisnear

import (
	"encoding/json"
	"fmt"
)

const messageVersion = 1

type operation string

const (
	operationSet    operation = "set"
	operationDelete operation = "delete"
	operationClear  operation = "clear"
)

type invalidationMessage struct {
	Version   int       `json:"version"`
	Namespace string    `json:"namespace"`
	OriginID  string    `json:"originID"`
	Operation operation `json:"operation"`
	Key       string    `json:"key,omitempty"`
}

func encodeMessage(message invalidationMessage) ([]byte, error) {
	message.Version = messageVersion
	return json.Marshal(message)
}

func decodeMessage(payload string) (invalidationMessage, error) {
	var message invalidationMessage
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		return message, fmt.Errorf("decode near-cache invalidation: %w", err)
	}
	if message.Version != messageVersion {
		return message, fmt.Errorf("unsupported near-cache invalidation version: %d", message.Version)
	}
	switch message.Operation {
	case operationSet, operationDelete, operationClear:
	default:
		return message, fmt.Errorf("unsupported near-cache invalidation operation: %q", message.Operation)
	}
	if message.Namespace == "" {
		return message, fmt.Errorf("near-cache invalidation namespace must not be empty")
	}
	if message.OriginID == "" {
		return message, fmt.Errorf("near-cache invalidation origin must not be empty")
	}
	return message, nil
}
