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
	// JSON escaping can use up to six source bytes for one decoded byte
	// (for example, "\\u003c"). Keep the parser bound conservative so
	// validation happens before json.Unmarshal allocates a decoded string.
	maxJSONStringEscapeBytes = 6
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
	var out bytes.Buffer
	out.Grow(len(envelopeMagic) + 256 + len(e.KeyID) + base64.StdEncoding.EncodedLen(len(e.EncryptedDataKey)) + base64.StdEncoding.EncodedLen(len(e.Nonce)) + base64.StdEncoding.EncodedLen(len(e.Ciphertext)))
	out.WriteString(envelopeMagic)
	out.WriteString(`{"version":1,"algorithm":"AES-256-GCM","key_id":`)
	if err := writeJSONString(&out, e.KeyID); err != nil {
		return nil, errorWith(ErrMalformedEnvelope, "marshal envelope", err)
	}
	out.WriteString(`,"encrypted_data_key":"`)
	writeBase64(&out, e.EncryptedDataKey)
	out.WriteString(`","encryption_context":[`)
	for index, entry := range contextEntries(e.EncryptionContext) {
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString(`{"key":`)
		if err := writeJSONString(&out, entry.Key); err != nil {
			return nil, errorWith(ErrMalformedEnvelope, "marshal envelope", err)
		}
		out.WriteString(`,"value":`)
		if err := writeJSONString(&out, entry.Value); err != nil {
			return nil, errorWith(ErrMalformedEnvelope, "marshal envelope", err)
		}
		out.WriteByte('}')
	}
	out.WriteString(`],"nonce":"`)
	writeBase64(&out, e.Nonce)
	out.WriteString(`","ciphertext":"`)
	writeBase64(&out, e.Ciphertext)
	out.WriteString(`"}`)
	if out.Len() > MaxEnvelopeSize {
		return nil, errorWith(ErrInputTooLarge, "marshal envelope", nil)
	}
	return out.Bytes(), nil
}

func writeJSONString(out *bytes.Buffer, value string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, _ = out.Write(encoded)
	return nil
}

func writeBase64(out *bytes.Buffer, value []byte) {
	encoder := base64.NewEncoder(base64.StdEncoding, out)
	_, _ = encoder.Write(value)
	_ = encoder.Close()
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
	envelope, err := parseCanonicalEnvelope(jsonBytes)
	if err != nil {
		if errors.Is(err, ErrInputTooLarge) {
			return Envelope{}, err
		}
		if errors.Is(err, ErrUnsupportedVersion) || errors.Is(err, ErrUnsupportedAlgorithm) {
			return Envelope{}, err
		}
		return Envelope{}, errorWith(ErrMalformedEnvelope, "parse envelope", nil)
	}
	if err := validateEnvelope(envelope); err != nil {
		return Envelope{}, err
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

type canonicalJSONParser struct {
	data   []byte
	offset int
}

func parseCanonicalEnvelope(data []byte) (Envelope, error) {
	parser := canonicalJSONParser{data: data}
	if !parser.consume('{') {
		return Envelope{}, io.ErrUnexpectedEOF
	}

	fieldNames := [...]string{
		"version",
		"algorithm",
		"key_id",
		"encrypted_data_key",
		"encryption_context",
		"nonce",
		"ciphertext",
	}
	envelope := Envelope{}
	for index, fieldName := range fieldNames {
		if index > 0 && !parser.consume(',') {
			return Envelope{}, io.ErrUnexpectedEOF
		}
		field, err := parser.stringRaw()
		if err != nil || !bytes.Equal(field, []byte(`"`+fieldName+`"`)) {
			return Envelope{}, io.ErrUnexpectedEOF
		}
		if !parser.consume(':') {
			return Envelope{}, io.ErrUnexpectedEOF
		}

		switch fieldName {
		case "version":
			version, err := parser.uint8()
			if err != nil {
				return Envelope{}, err
			}
			if version != EnvelopeVersion {
				return Envelope{}, errorWith(ErrUnsupportedVersion, "parse envelope", nil)
			}
			envelope.Version = version
		case "algorithm":
			raw, err := parser.stringRawBounded(len(AlgorithmAES256GCM) + 2)
			if err != nil {
				return Envelope{}, err
			}
			algorithm, err := decodeCanonicalJSONString(raw)
			if err != nil {
				return Envelope{}, err
			}
			if Algorithm(algorithm) != AlgorithmAES256GCM {
				return Envelope{}, errorWith(ErrUnsupportedAlgorithm, "parse envelope", nil)
			}
			envelope.Algorithm = Algorithm(algorithm)
		case "key_id":
			raw, err := parser.stringRawBounded(maxJSONStringBytes(MaxKeyIDSize))
			if err != nil {
				return Envelope{}, err
			}
			value, err := decodeCanonicalJSONString(raw)
			if err != nil {
				return Envelope{}, err
			}
			envelope.KeyID = value
		case "encrypted_data_key":
			raw, err := parser.stringRaw()
			if err != nil {
				return Envelope{}, err
			}
			value, err := decodeCanonicalBase64Raw(raw, MaxEncryptedDataKeySize)
			if err != nil {
				return Envelope{}, err
			}
			envelope.EncryptedDataKey = value
		case "encryption_context":
			value, err := parser.context()
			if err != nil {
				return Envelope{}, err
			}
			envelope.EncryptionContext = value
		case "nonce":
			raw, err := parser.stringRaw()
			if err != nil {
				return Envelope{}, err
			}
			value, err := decodeCanonicalBase64Raw(raw, gcmNonceSize)
			if err != nil {
				return Envelope{}, err
			}
			envelope.Nonce = value
		case "ciphertext":
			raw, err := parser.stringRaw()
			if err != nil {
				return Envelope{}, err
			}
			encoded := raw[1 : len(raw)-1]
			if len(encoded) > base64.StdEncoding.EncodedLen(MaxPlaintextSize+gcmOverhead) {
				return Envelope{}, errorWith(ErrInputTooLarge, "parse envelope", nil)
			}
			value, err := decodeCanonicalBase64Raw(raw, MaxPlaintextSize+gcmOverhead)
			if err != nil {
				return Envelope{}, err
			}
			envelope.Ciphertext = value
		}
	}
	if !parser.consume('}') || parser.offset != len(parser.data) {
		return Envelope{}, io.ErrUnexpectedEOF
	}
	return envelope, nil
}

func (p *canonicalJSONParser) consume(expected byte) bool {
	if p.offset >= len(p.data) || p.data[p.offset] != expected {
		return false
	}
	p.offset++
	return true
}

func (p *canonicalJSONParser) stringRaw() ([]byte, error) {
	return p.stringRawBounded(0)
}

func (p *canonicalJSONParser) stringRawBounded(maxRawBytes int) ([]byte, error) {
	if !p.consume('"') {
		return nil, io.ErrUnexpectedEOF
	}
	start := p.offset - 1
	for index := p.offset; index < len(p.data); index++ {
		switch p.data[index] {
		case '"':
			p.offset = index + 1
			if maxRawBytes > 0 && p.offset-start > maxRawBytes {
				return nil, errorWith(ErrInputTooLarge, "parse string", nil)
			}
			return p.data[start:p.offset], nil
		case '\\':
			index++
			if index >= len(p.data) {
				return nil, io.ErrUnexpectedEOF
			}
			if p.data[index] == 'u' {
				if index+4 >= len(p.data) {
					return nil, io.ErrUnexpectedEOF
				}
				index += 4
			}
		default:
			if p.data[index] < 0x20 {
				return nil, io.ErrUnexpectedEOF
			}
		}
		if maxRawBytes > 0 && index-start+1 > maxRawBytes {
			return nil, errorWith(ErrInputTooLarge, "parse string", nil)
		}
	}
	return nil, io.ErrUnexpectedEOF
}

func (p *canonicalJSONParser) uint8() (uint8, error) {
	start := p.offset
	for p.offset < len(p.data) && p.data[p.offset] >= '0' && p.data[p.offset] <= '9' {
		p.offset++
	}
	if start == p.offset {
		return 0, io.ErrUnexpectedEOF
	}
	var value uint8
	if err := json.Unmarshal(p.data[start:p.offset], &value); err != nil {
		return 0, io.ErrUnexpectedEOF
	}
	return value, nil
}

func (p *canonicalJSONParser) context() (map[string]string, error) {
	if !p.consume('[') {
		return nil, io.ErrUnexpectedEOF
	}
	context := make(map[string]string)
	if p.consume(']') {
		return context, nil
	}
	previous := ""
	rawBytes := 0
	for index := 0; ; index++ {
		if index >= MaxContextEntries {
			return nil, errorWith(ErrInputTooLarge, "parse context", nil)
		}
		if !p.consume('{') {
			return nil, io.ErrUnexpectedEOF
		}
		keyField, err := p.stringRaw()
		if err != nil || !bytes.Equal(keyField, []byte(`"key"`)) || !p.consume(':') {
			return nil, io.ErrUnexpectedEOF
		}
		keyRaw, err := p.stringRawBounded(maxJSONStringBytes(MaxContextSize))
		if err != nil {
			return nil, err
		}
		rawBytes += len(keyRaw)
		if rawBytes > maxContextJSONStringBytes() {
			return nil, errorWith(ErrInputTooLarge, "parse context", nil)
		}
		key, err := decodeCanonicalJSONString(keyRaw)
		if err != nil || key == "" {
			return nil, io.ErrUnexpectedEOF
		}
		if !p.consume(',') {
			return nil, io.ErrUnexpectedEOF
		}
		valueField, err := p.stringRaw()
		if err != nil || !bytes.Equal(valueField, []byte(`"value"`)) || !p.consume(':') {
			return nil, io.ErrUnexpectedEOF
		}
		valueRaw, err := p.stringRawBounded(maxJSONStringBytes(MaxContextSize))
		if err != nil {
			return nil, err
		}
		rawBytes += len(valueRaw)
		if rawBytes > maxContextJSONStringBytes() {
			return nil, errorWith(ErrInputTooLarge, "parse context", nil)
		}
		value, err := decodeCanonicalJSONString(valueRaw)
		if err != nil {
			return nil, err
		}
		if !p.consume('}') || (index > 0 && key <= previous) {
			return nil, io.ErrUnexpectedEOF
		}
		if _, exists := context[key]; exists {
			return nil, io.ErrUnexpectedEOF
		}
		context[key] = value
		previous = key
		if p.consume(']') {
			break
		}
		if !p.consume(',') {
			return nil, io.ErrUnexpectedEOF
		}
	}
	if err := validateContext(context); err != nil {
		return nil, err
	}
	return context, nil
}

func maxJSONStringBytes(maxDecodedBytes int) int {
	return maxDecodedBytes*maxJSONStringEscapeBytes + 2
}

func maxContextJSONStringBytes() int {
	// Every context entry has two quoted strings. This bound permits the
	// largest valid escaped representation while keeping decode allocations
	// bounded by the aggregate context contract.
	return MaxContextSize*maxJSONStringEscapeBytes + 4*MaxContextEntries
}

func decodeCanonicalJSONString(raw []byte) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !utf8.ValidString(value) {
		return "", io.ErrUnexpectedEOF
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return "", io.ErrUnexpectedEOF
	}
	return value, nil
}

func decodeCanonicalBase64Raw(raw []byte, maxDecoded int) ([]byte, error) {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return nil, io.ErrUnexpectedEOF
	}
	encoded := raw[1 : len(raw)-1]
	if len(encoded) > base64.StdEncoding.EncodedLen(maxDecoded) || !validCanonicalBase64(encoded) {
		return nil, io.ErrUnexpectedEOF
	}
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	length, err := base64.StdEncoding.Decode(decoded, encoded)
	if err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	return decoded[:length], nil
}

func validCanonicalBase64(encoded []byte) bool {
	if len(encoded)%4 != 0 {
		return false
	}
	padding := 0
	for index := len(encoded) - 1; index >= 0 && encoded[index] == '='; index-- {
		padding++
	}
	if padding > 2 {
		return false
	}
	contentLength := len(encoded) - padding
	for index, value := range encoded {
		if index >= contentLength {
			if value != '=' {
				return false
			}
			continue
		}
		if base64Value(value) < 0 {
			return false
		}
	}
	if padding == 0 {
		return contentLength%4 == 0
	}
	if padding == 1 {
		return contentLength%4 == 3 && base64Value(encoded[contentLength-1])&0x03 == 0
	}
	return contentLength%4 == 2 && base64Value(encoded[contentLength-1])&0x0f == 0
}

func base64Value(value byte) int {
	switch {
	case value >= 'A' && value <= 'Z':
		return int(value - 'A')
	case value >= 'a' && value <= 'z':
		return int(value-'a') + 26
	case value >= '0' && value <= '9':
		return int(value-'0') + 52
	case value == '+':
		return 62
	case value == '/':
		return 63
	default:
		return -1
	}
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
