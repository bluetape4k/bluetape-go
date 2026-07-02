package encrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"unicode/utf8"
)

const (
	envelopeVersion byte = 1
	algorithmAESGCM byte = 1

	minAESKeySize = 16
)

var (
	envelopeMagic  = []byte{'B', 'T', 'E', 'N', 'C'}
	envelopeHeader = []byte{'B', 'T', 'E', 'N', 'C', envelopeVersion, algorithmAESGCM}
)

// Option configures an Encryptor.
type Option func(*config) error

type config struct{}

// Encryptor encrypts and decrypts bytes with caller-owned AES key material.
//
// Encryptor values are immutable after construction and safe for concurrent
// use by multiple goroutines.
type Encryptor struct {
	aead cipher.AEAD
}

// New creates an Encryptor from a 16, 24, or 32 byte AES key.
//
// The key is copied before use. Callers must persist and rotate key material
// outside this package; generated keys are test or provisioning helpers only.
func New(key []byte, options ...Option) (Encryptor, error) {
	var cfg config
	for _, option := range options {
		if option == nil {
			return Encryptor{}, errorWith(ErrInvalidOptions, "apply option", nil)
		}
		if err := option(&cfg); err != nil {
			return Encryptor{}, errorWith(ErrInvalidOptions, "apply option", err)
		}
	}

	if !validAESKeySize(len(key)) {
		return Encryptor{}, errorWith(ErrInvalidKey, "new", nil)
	}

	copied := append([]byte(nil), key...)
	block, err := aes.NewCipher(copied)
	if err != nil {
		return Encryptor{}, errorWith(ErrInvalidKey, "new", err)
	}

	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return Encryptor{}, errorWith(ErrInvalidOptions, "new", err)
	}

	return Encryptor{aead: aead}, nil
}

// Encrypt encrypts plaintext and returns a versioned ciphertext envelope.
func Encrypt(key, plaintext, associatedData []byte) ([]byte, error) {
	encryptor, err := New(key)
	if err != nil {
		return nil, err
	}
	return encryptor.Encrypt(plaintext, associatedData)
}

// Decrypt decrypts a versioned ciphertext envelope.
func Decrypt(key, ciphertext, associatedData []byte) ([]byte, error) {
	encryptor, err := New(key)
	if err != nil {
		return nil, err
	}
	return encryptor.Decrypt(ciphertext, associatedData)
}

// Encrypt encrypts plaintext and returns a versioned ciphertext envelope.
func (e Encryptor) Encrypt(plaintext, associatedData []byte) ([]byte, error) {
	if e.aead == nil {
		return nil, errorWith(ErrInvalidKey, "encrypt", nil)
	}

	sealed := e.aead.Seal(nil, nil, plaintext, associatedData)
	out := make([]byte, 0, len(envelopeHeader)+len(sealed))
	out = append(out, envelopeHeader...)
	out = append(out, sealed...)
	return out, nil
}

// Decrypt decrypts a versioned ciphertext envelope.
func (e Encryptor) Decrypt(ciphertext, associatedData []byte) ([]byte, error) {
	if e.aead == nil {
		return nil, errorWith(ErrInvalidKey, "decrypt", nil)
	}

	payload, err := parseEnvelope(ciphertext, e.aead.Overhead())
	if err != nil {
		return nil, err
	}

	plaintext, err := e.aead.Open(nil, nil, payload, associatedData)
	if err != nil {
		return nil, errorWith(ErrAuthenticationFailed, "decrypt", err)
	}
	return plaintext, nil
}

// EncryptString encrypts UTF-8 text and returns a URL-safe base64 envelope.
func (e Encryptor) EncryptString(plaintext string, associatedData []byte) (string, error) {
	if !utf8.ValidString(plaintext) {
		return "", errorWith(ErrInvalidOptions, "encrypt string", nil)
	}
	ciphertext, err := e.Encrypt([]byte(plaintext), associatedData)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a URL-safe base64 envelope into UTF-8 text.
func (e Encryptor) DecryptString(ciphertext string, associatedData []byte) (string, error) {
	envelope, err := base64.RawURLEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", errorWith(ErrMalformedCiphertext, "decode string", err)
	}

	plaintext, err := e.Decrypt(envelope, associatedData)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(plaintext) {
		return "", errorWith(ErrMalformedCiphertext, "decode string", nil)
	}
	return string(plaintext), nil
}

func parseEnvelope(ciphertext []byte, overhead int) ([]byte, error) {
	headerLen := len(envelopeHeader)
	if len(ciphertext) < headerLen+overhead {
		return nil, errorWith(ErrMalformedCiphertext, "parse envelope", nil)
	}
	if !bytes.Equal(ciphertext[:len(envelopeMagic)], envelopeMagic) {
		return nil, errorWith(ErrMalformedCiphertext, "parse envelope", nil)
	}
	if version := ciphertext[len(envelopeMagic)]; version != envelopeVersion {
		return nil, errorWith(ErrMalformedCiphertext, "parse envelope", fmt.Errorf("unsupported envelope version"))
	}
	if algorithm := ciphertext[len(envelopeMagic)+1]; algorithm != algorithmAESGCM {
		return nil, errorWith(ErrMalformedCiphertext, "parse envelope", fmt.Errorf("unsupported envelope algorithm"))
	}
	return ciphertext[headerLen:], nil
}

func validAESKeySize(size int) bool {
	switch size {
	case minAESKeySize, 24, 32:
		return true
	default:
		return false
	}
}
