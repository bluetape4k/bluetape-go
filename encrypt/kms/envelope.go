package kms

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	envelopeMagic = "BTKMS"
	gcmNonceSize  = 12
	gcmOverhead   = 16
)

const (
	// MaxEnvelopeSize - 직렬화된 BTKMS envelope의 최대 byte 크기다.
	MaxEnvelopeSize = 64 << 20
	// MaxPlaintextSize - envelope overhead를 고려한 plaintext 최대 byte 크기다.
	MaxPlaintextSize = 32 << 20
	// MaxAssociatedDataSize - caller associated data의 최대 byte 크기다.
	MaxAssociatedDataSize = 64 << 10
	// MaxKeyIDSize - key ID의 최대 UTF-8 byte 크기다.
	MaxKeyIDSize = 2 << 10
	// MaxContextEntries - encryption context entry의 최대 개수다.
	MaxContextEntries = 64
	// MaxContextSize - context 모든 key/value UTF-8 byte 길이 합계의 최대값이다.
	MaxContextSize = 8 << 10
	// MaxEncryptedDataKeySize - KMS encrypted data key blob의 최대 byte 크기다.
	MaxEncryptedDataKeySize = 6144
)

// Algorithm - envelope가 사용하는 local encryption algorithm 식별자다.
type Algorithm string

const (
	// AlgorithmAES256GCM - KMS AES-256 data key와 local AES-GCM 조합을 나타낸다.
	AlgorithmAES256GCM Algorithm = "AES-256-GCM"
	// EnvelopeVersion - 현재 지원하는 BTKMS wire version이다.
	EnvelopeVersion uint8 = 1
)

// Envelope - KMS encrypted data key와 local AES-GCM payload를 담는 immutable wire 값이다.
// MarshalBinary와 ParseEnvelope는 반환되는 slice/map을 서로 공유하지 않는다.
type Envelope struct {
	// Version은 BTKMS envelope version이다.
	Version uint8
	// Algorithm은 local payload encryption algorithm이다.
	Algorithm Algorithm
	// KeyID는 GenerateDataKey에 사용한 caller 선택 key 식별자다.
	KeyID string
	// EncryptedDataKey는 KMS가 반환한 encrypted data key blob이다.
	EncryptedDataKey []byte
	// EncryptionContext는 KMS encryption context의 복사본이다.
	EncryptionContext map[string]string
	// Nonce는 local AES-GCM nonce다.
	Nonce []byte
	// Ciphertext는 local AES-GCM ciphertext와 authentication tag다.
	Ciphertext []byte
}

type wireContextEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type wireEnvelope struct {
	Version           uint8              `json:"version"`
	Algorithm         Algorithm          `json:"algorithm"`
	KeyID             string             `json:"key_id"`
	EncryptedDataKey  string             `json:"encrypted_data_key"`
	EncryptionContext []wireContextEntry `json:"encryption_context"`
	Nonce             string             `json:"nonce"`
	Ciphertext        string             `json:"ciphertext"`
}

type wireMetadata struct {
	Version           uint8              `json:"version"`
	Algorithm         Algorithm          `json:"algorithm"`
	KeyID             string             `json:"key_id"`
	EncryptedDataKey  string             `json:"encrypted_data_key"`
	EncryptionContext []wireContextEntry `json:"encryption_context"`
}

// MarshalBinary - Envelope를 canonical BTKMS bytes로 직렬화한다.
func (e Envelope) MarshalBinary() ([]byte, error) {
	if err := validateEnvelope(e); err != nil {
		return nil, err
	}
	wire := toWireEnvelope(e)
	jsonBytes, err := json.Marshal(wire)
	if err != nil {
		return nil, errorWith(ErrMalformedEnvelope, "marshal envelope", err)
	}
	out := make([]byte, 0, len(envelopeMagic)+len(jsonBytes))
	out = append(out, envelopeMagic...)
	out = append(out, jsonBytes...)
	if len(out) > MaxEnvelopeSize {
		return nil, errorWith(ErrInputTooLarge, "marshal envelope", nil)
	}
	return out, nil
}

// ParseEnvelope - canonical BTKMS bytes를 엄격히 검증하고 독립 복사본을 반환한다.
func ParseEnvelope(data []byte) (Envelope, error) {
	if len(data) > MaxEnvelopeSize {
		return Envelope{}, errorWith(ErrInputTooLarge, "parse envelope", nil)
	}
	if len(data) <= len(envelopeMagic) || !bytes.Equal(data[:len(envelopeMagic)], []byte(envelopeMagic)) {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	jsonBytes := data[len(envelopeMagic):]
	if !utf8.Valid(jsonBytes) {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	values, err := decodeStrictObject(jsonBytes)
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	if err := requireFields(values, "version", "algorithm", "key_id", "encrypted_data_key", "encryption_context", "nonce", "ciphertext"); err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}

	version, err := decodeUint8(values["version"])
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	if version != EnvelopeVersion {
		return Envelope{}, errorWith(ErrUnsupportedVersion, "parse envelope", nil)
	}
	algorithm, err := decodeString(values["algorithm"])
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	if Algorithm(algorithm) != AlgorithmAES256GCM {
		return Envelope{}, errorWith(ErrUnsupportedAlgorithm, "parse envelope", nil)
	}
	keyID, err := decodeString(values["key_id"])
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	encodedDataKey, err := decodeString(values["encrypted_data_key"])
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	dataKey, err := decodeCanonicalBase64(encodedDataKey, MaxEncryptedDataKeySize)
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	nonceEncoded, err := decodeString(values["nonce"])
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	nonce, err := decodeCanonicalBase64(nonceEncoded, gcmNonceSize)
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	ciphertextEncoded, err := decodeString(values["ciphertext"])
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	if len(ciphertextEncoded) > base64.StdEncoding.EncodedLen(MaxPlaintextSize+gcmOverhead) {
		return Envelope{}, errorWith(ErrInputTooLarge, "parse envelope", nil)
	}
	ciphertext, err := decodeCanonicalBase64(ciphertextEncoded, MaxPlaintextSize+gcmOverhead)
	if err != nil {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	context, err := decodeContext(values["encryption_context"])
	if err != nil {
		if errors.Is(err, ErrInputTooLarge) {
			return Envelope{}, err
		}
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}

	envelope := Envelope{
		Version:           version,
		Algorithm:         Algorithm(algorithm),
		KeyID:             keyID,
		EncryptedDataKey:  append([]byte(nil), dataKey...),
		EncryptionContext: cloneContext(context),
		Nonce:             append([]byte(nil), nonce...),
		Ciphertext:        append([]byte(nil), ciphertext...),
	}
	if err := validateEnvelope(envelope); err != nil {
		return Envelope{}, err
	}
	canonical, err := envelope.MarshalBinary()
	if err != nil {
		return Envelope{}, err
	}
	if !bytes.Equal(canonical, data) {
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	return envelope, nil
}

func canonicalMetadata(e Envelope) ([]byte, error) {
	if err := validateMetadata(e); err != nil {
		return nil, err
	}
	metadata := wireMetadata{
		Version:           e.Version,
		Algorithm:         e.Algorithm,
		KeyID:             e.KeyID,
		EncryptedDataKey:  base64.StdEncoding.EncodeToString(e.EncryptedDataKey),
		EncryptionContext: contextEntries(e.EncryptionContext),
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, errorWith(ErrMalformedEnvelope, "marshal metadata", err)
	}
	return encoded, nil
}

func buildAssociatedData(metadata, callerAD []byte) ([]byte, error) {
	if len(metadata) > MaxEnvelopeSize || len(callerAD) > MaxAssociatedDataSize {
		return nil, errorWith(ErrInputTooLarge, "build associated data", nil)
	}
	out := make([]byte, 0, len("BTKMS-AAD\x01")+4+len(metadata)+4+len(callerAD))
	out = append(out, []byte("BTKMS-AAD\x01")...)
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(metadata)))
	out = append(out, size[:]...)
	out = append(out, metadata...)
	binary.BigEndian.PutUint32(size[:], uint32(len(callerAD)))
	out = append(out, size[:]...)
	out = append(out, callerAD...)
	return out, nil
}

func validateEnvelope(e Envelope) error {
	if err := validateMetadata(e); err != nil {
		return err
	}
	if len(e.Nonce) != gcmNonceSize {
		return errorWith(ErrMalformedEnvelope, "validate envelope", nil)
	}
	if len(e.Ciphertext) < gcmOverhead {
		return errorWith(ErrMalformedEnvelope, "validate envelope", nil)
	}
	if len(e.Ciphertext)-gcmOverhead > MaxPlaintextSize {
		return errorWith(ErrInputTooLarge, "validate envelope", nil)
	}
	return nil
}

func validateMetadata(e Envelope) error {
	if e.Version != EnvelopeVersion {
		return errorWith(ErrUnsupportedVersion, "validate envelope", nil)
	}
	if e.Algorithm != AlgorithmAES256GCM {
		return errorWith(ErrUnsupportedAlgorithm, "validate envelope", nil)
	}
	if !utf8.ValidString(e.KeyID) || strings.TrimSpace(e.KeyID) == "" || len(e.KeyID) > MaxKeyIDSize {
		return errorWith(ErrInvalidKeyID, "validate envelope", nil)
	}
	if len(e.EncryptedDataKey) == 0 || len(e.EncryptedDataKey) > MaxEncryptedDataKeySize {
		return errorWith(ErrMalformedEnvelope, "validate envelope", nil)
	}
	if err := validateContext(e.EncryptionContext); err != nil {
		return err
	}
	return nil
}

func validateContext(context map[string]string) error {
	if len(context) > MaxContextEntries {
		return errorWith(ErrInputTooLarge, "validate context", nil)
	}
	total := 0
	for key, value := range context {
		if !utf8.ValidString(key) || !utf8.ValidString(value) || key == "" {
			return errorWith(ErrMalformedEnvelope, "validate context", nil)
		}
		total += len(key) + len(value)
		if total > MaxContextSize {
			return errorWith(ErrInputTooLarge, "validate context", nil)
		}
	}
	return nil
}

func toWireEnvelope(e Envelope) wireEnvelope {
	return wireEnvelope{
		Version:           e.Version,
		Algorithm:         e.Algorithm,
		KeyID:             e.KeyID,
		EncryptedDataKey:  base64.StdEncoding.EncodeToString(e.EncryptedDataKey),
		EncryptionContext: contextEntries(e.EncryptionContext),
		Nonce:             base64.StdEncoding.EncodeToString(e.Nonce),
		Ciphertext:        base64.StdEncoding.EncodeToString(e.Ciphertext),
	}
}

func contextEntries(context map[string]string) []wireContextEntry {
	entries := make([]wireContextEntry, 0, len(context))
	for key, value := range context {
		entries = append(entries, wireContextEntry{Key: key, Value: value})
	}
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Key < entries[j-1].Key; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	return entries
}

func cloneContext(context map[string]string) map[string]string {
	if len(context) == 0 {
		return map[string]string{}
	}
	contextCopy := make(map[string]string, len(context))
	for key, value := range context {
		contextCopy[key] = value
	}
	return contextCopy
}

func decodeStrictObject(data []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return nil, io.ErrUnexpectedEOF
	}
	values := make(map[string]json.RawMessage)
	lower := make(map[string]string)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok || key == "" {
			return nil, io.ErrUnexpectedEOF
		}
		lowerKey := strings.ToLower(key)
		if _, exists := values[key]; exists {
			return nil, io.ErrUnexpectedEOF
		}
		if previous, exists := lower[lowerKey]; exists && previous != key {
			return nil, io.ErrUnexpectedEOF
		}
		lower[lowerKey] = key
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, io.ErrUnexpectedEOF
		}
		values[key] = append([]byte(nil), raw...)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, io.ErrUnexpectedEOF
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, io.ErrUnexpectedEOF
	}
	return values, nil
}

func requireFields(values map[string]json.RawMessage, fields ...string) error {
	if len(values) != len(fields) {
		return io.ErrUnexpectedEOF
	}
	for _, field := range fields {
		if _, ok := values[field]; !ok {
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}

func decodeString(raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !utf8.ValidString(value) {
		return "", io.ErrUnexpectedEOF
	}
	return value, nil
}

func decodeUint8(raw json.RawMessage) (uint8, error) {
	var value uint8
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	return value, nil
}

func decodeCanonicalBase64(encoded string, maxDecoded int) ([]byte, error) {
	if maxDecoded < 0 || len(encoded) > base64.StdEncoding.EncodedLen(maxDecoded) {
		return nil, io.ErrUnexpectedEOF
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) > maxDecoded || base64.StdEncoding.EncodeToString(decoded) != encoded {
		return nil, io.ErrUnexpectedEOF
	}
	return decoded, nil
}

func decodeContext(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 || raw[0] != '[' {
		return nil, io.ErrUnexpectedEOF
	}
	var entries []json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&entries); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, io.ErrUnexpectedEOF
	}
	if len(entries) > MaxContextEntries {
		return nil, errorWith(ErrInputTooLarge, "parse context", nil)
	}
	context := make(map[string]string, len(entries))
	previous := ""
	for index, entry := range entries {
		values, err := decodeStrictObject(entry)
		if err != nil || len(values) != 2 {
			return nil, io.ErrUnexpectedEOF
		}
		if err := requireFields(values, "key", "value"); err != nil {
			return nil, err
		}
		key, err := decodeString(values["key"])
		if err != nil || key == "" {
			return nil, io.ErrUnexpectedEOF
		}
		value, err := decodeString(values["value"])
		if err != nil {
			return nil, err
		}
		if index > 0 && key <= previous {
			return nil, io.ErrUnexpectedEOF
		}
		if _, exists := context[key]; exists {
			return nil, io.ErrUnexpectedEOF
		}
		context[key] = value
		previous = key
	}
	if err := validateContext(context); err != nil {
		return nil, err
	}
	return context, nil
}
