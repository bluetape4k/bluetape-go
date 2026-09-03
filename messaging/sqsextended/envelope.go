package sqsextended

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	// EnvelopeVersion - 현재 지원하는 wire-format version이다.
	EnvelopeVersion = 1
	// DefaultMaxPayloadSize - option을 지정하지 않았을 때 Provider가 읽는
	// payload의 상한이다.
	DefaultMaxPayloadSize int64 = 256 << 20
	// DefaultMaxReceivePayloadSize - option을 지정하지 않았을 때 하나의
	// Receive 호출이 누적해 보관할 payload byte의 상한이다.
	DefaultMaxReceivePayloadSize int64 = 512 << 20
	// MaxEnvelopeSize - 인코딩된 SQS envelope body의 상한이다.
	MaxEnvelopeSize = 64 << 10

	maxBucketSize      = 1024
	maxObjectKeySize   = 1024
	maxContentTypeSize = 1024
	maxMetadataEntries = 32
	maxMetadataKeySize = 128
	maxMetadataValSize = 2048
)

// Envelope - 하나의 S3 payload object를 가리키는 versioned SQS body이다.
//
// Bucket과 Key는 호출자가 소유하며 입력을 그대로 보존한다. Checksum은
// payload byte의 lowercase SHA-256 hexadecimal digest이다. EncryptionMetadata는
// 설명용일 뿐이며 이 package는 key를 생성하거나 decrypt하지 않는다.
type Envelope struct {
	Version            int               `json:"version"`
	Bucket             string            `json:"bucket"`
	Key                string            `json:"key"`
	ContentSize        int64             `json:"content_size"`
	Checksum           string            `json:"checksum"`
	ContentType        string            `json:"content_type,omitempty"`
	EncryptionMetadata map[string]string `json:"encryption_metadata,omitempty"`
}

// EncodeEnvelope - Envelope를 검증하고 canonical JSON 표현으로 인코딩한다.
// 반환한 byte는 입력의 mutable map storage를 공유하지 않는다.
func EncodeEnvelope(envelope Envelope) ([]byte, error) {
	if err := validateEnvelope(envelope); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, newError(ErrInvalidEnvelope, "encode envelope", err, false, false)
	}
	if len(encoded) > MaxEnvelopeSize {
		return nil, newError(ErrEnvelopeTooLarge, "encode envelope", nil, false, false)
	}
	return encoded, nil
}

// DecodeEnvelope - canonical Envelope body를 검증하고 디코딩한다.
func DecodeEnvelope(data []byte) (Envelope, error) {
	if len(data) == 0 || len(data) > MaxEnvelopeSize {
		if len(data) > MaxEnvelopeSize {
			return Envelope{}, newError(ErrEnvelopeTooLarge, "decode envelope", nil, false, false)
		}
		return Envelope{}, newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
	}
	if err := rejectDuplicateOrTrailingFields(data); err != nil {
		return Envelope{}, err
	}
	var envelope Envelope
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, newError(ErrInvalidEnvelope, "decode envelope", err, false, false)
	}
	if err := validateEnvelope(envelope); err != nil {
		return Envelope{}, err
	}
	canonical, err := EncodeEnvelope(envelope)
	if err != nil {
		return Envelope{}, err
	}
	if !bytes.Equal(canonical, data) {
		return Envelope{}, newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
	}
	envelope.EncryptionMetadata = cloneMetadata(envelope.EncryptionMetadata)
	return envelope, nil
}

func validateEnvelope(envelope Envelope) error {
	if envelope.Version != EnvelopeVersion {
		return newError(ErrUnsupportedVersion, "validate envelope", nil, false, false)
	}
	if !validRequiredString(envelope.Bucket, maxBucketSize) || !validRequiredString(envelope.Key, maxObjectKeySize) {
		return newError(ErrInvalidEnvelope, "validate envelope", nil, false, false)
	}
	if envelope.ContentSize < 0 || envelope.ContentSize > DefaultMaxPayloadSize {
		return newError(ErrPayloadTooLarge, "validate envelope", nil, false, false)
	}
	if len(envelope.Checksum) != 64 || !isLowerHex(envelope.Checksum) {
		return newError(ErrInvalidEnvelope, "validate envelope", nil, false, false)
	}
	if envelope.ContentType != "" && !validRequiredString(envelope.ContentType, maxContentTypeSize) {
		return newError(ErrInvalidEnvelope, "validate envelope", nil, false, false)
	}
	if len(envelope.EncryptionMetadata) > maxMetadataEntries {
		return newError(ErrInvalidEnvelope, "validate envelope", nil, false, false)
	}
	for key, value := range envelope.EncryptionMetadata {
		if !validRequiredString(key, maxMetadataKeySize) || !validString(value, maxMetadataValSize) {
			return newError(ErrInvalidEnvelope, "validate envelope", nil, false, false)
		}
	}
	return nil
}

func rejectDuplicateOrTrailingFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := inspectJSONValue(decoder, true); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return newError(ErrInvalidEnvelope, "decode envelope", err, false, false)
	}
	return nil
}

func inspectJSONValue(decoder *json.Decoder, requireObject bool) error {
	token, err := decoder.Token()
	if err != nil {
		return newError(ErrInvalidEnvelope, "decode envelope", err, false, false)
	}
	delim, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		if requireObject {
			return newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
		}
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{}, 8)
		for decoder.More() {
			token, err = decoder.Token()
			if err != nil {
				return newError(ErrInvalidEnvelope, "decode envelope", err, false, false)
			}
			key, ok := token.(string)
			if !ok {
				return newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
			}
			if _, exists := seen[key]; exists {
				return newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
			}
			seen[key] = struct{}{}
			if err := inspectJSONValue(decoder, false); err != nil {
				return err
			}
		}
		token, err = decoder.Token()
		if err != nil {
			return newError(ErrInvalidEnvelope, "decode envelope", err, false, false)
		}
		if delim, ok := token.(json.Delim); !ok || delim != '}' {
			return newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
		}
	case '[':
		for decoder.More() {
			if err := inspectJSONValue(decoder, false); err != nil {
				return err
			}
		}
		token, err = decoder.Token()
		if err != nil {
			return newError(ErrInvalidEnvelope, "decode envelope", err, false, false)
		}
		if delim, ok := token.(json.Delim); !ok || delim != ']' {
			return newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
		}
	case '}', ']':
		return newError(ErrInvalidEnvelope, "decode envelope", nil, false, false)
	}
	return nil
}

func validRequiredString(value string, maxBytes int) bool {
	return value != "" && validString(value, maxBytes) && strings.TrimSpace(value) != ""
}

func validString(value string, maxBytes int) bool {
	return len(value) <= maxBytes && utf8.ValidString(value)
}

func isLowerHex(value string) bool {
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	clone := make(map[string]string, len(metadata))
	for key, value := range metadata {
		clone[key] = value
	}
	return clone
}
