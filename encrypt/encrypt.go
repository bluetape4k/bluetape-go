package encrypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
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

// Option Encryptor 생성 설정을 적용한다.
type Option func(*config) error

type config struct{}

// Encryptor 호출자가 소유한 AES key material로 byte를 암호화하고 복호화한다.
//
// Encryptor 값은 생성 후 불변이며 여러 goroutine에서 동시에 사용해도 안전하다.
type Encryptor struct {
	aead cipher.AEAD
}

// New 16, 24, 32 byte AES key로 Encryptor를 생성한다.
//
// key는 사용 전에 복사된다. 호출자는 이 package 밖에서 key material을 영속화하고 회전해야 하며,
// 생성된 key는 test나 provisioning helper 용도에 한정된다.
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
	defer zeroBytes(copied)
	block, err := aes.NewCipher(copied)
	if err != nil {
		return Encryptor{}, errorWith(ErrInvalidKey, "new", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Encryptor{}, errorWith(ErrInvalidOptions, "new", err)
	}

	return Encryptor{aead: aead}, nil
}

// Encrypt plaintext를 암호화하고 versioned ciphertext envelope를 반환한다.
func Encrypt(key, plaintext, associatedData []byte) ([]byte, error) {
	encryptor, err := New(key)
	if err != nil {
		return nil, err
	}
	return encryptor.Encrypt(plaintext, associatedData)
}

// Decrypt versioned ciphertext envelope를 복호화한다.
func Decrypt(key, ciphertext, associatedData []byte) ([]byte, error) {
	encryptor, err := New(key)
	if err != nil {
		return nil, err
	}
	return encryptor.Decrypt(ciphertext, associatedData)
}

// Encrypt plaintext를 암호화하고 versioned ciphertext envelope를 반환한다.
func (e Encryptor) Encrypt(plaintext, associatedData []byte) ([]byte, error) {
	nonce, ciphertext, err := e.EncryptDetached(plaintext, associatedData)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(envelopeHeader)+len(nonce)+len(ciphertext))
	out = append(out, envelopeHeader...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt versioned ciphertext envelope를 복호화한다.
func (e Encryptor) Decrypt(ciphertext, associatedData []byte) ([]byte, error) {
	if e.aead == nil {
		return nil, errorWith(ErrInvalidKey, "decrypt", nil)
	}

	payload, err := parseEnvelope(ciphertext, e.aead.NonceSize()+e.aead.Overhead())
	if err != nil {
		return nil, err
	}

	return e.DecryptDetached(payload[:e.aead.NonceSize()], payload[e.aead.NonceSize():], associatedData)
}

// EncryptDetached plaintext를 암호화하고 nonce와 authentication tag를 포함한 ciphertext를 분리해 반환한다.
// 반환된 두 slice는 입력과 분리된 새 버퍼이며, 호출자는 이를 outer envelope에 저장할 책임이 있다.
func (e Encryptor) EncryptDetached(plaintext, associatedData []byte) ([]byte, []byte, error) {
	if e.aead == nil {
		return nil, nil, errorWith(ErrInvalidKey, "encrypt detached", nil)
	}

	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, errorWith(ErrInvalidOptions, "encrypt detached", err)
	}
	ciphertext := e.aead.Seal(nil, nonce, plaintext, associatedData)
	return nonce, ciphertext, nil
}

// DecryptDetached 분리된 nonce와 ciphertext를 검증하고 plaintext를 반환한다.
// nonce와 ciphertext 길이가 유효하지 않으면 authentication 전에 malformed 오류를 반환한다.
func (e Encryptor) DecryptDetached(nonce, ciphertext, associatedData []byte) ([]byte, error) {
	if e.aead == nil {
		return nil, errorWith(ErrInvalidKey, "decrypt detached", nil)
	}
	if len(nonce) != e.aead.NonceSize() || len(ciphertext) < e.aead.Overhead() {
		return nil, errorWith(ErrMalformedCiphertext, "decrypt detached", nil)
	}

	plaintext, err := e.aead.Open(nil, nonce, ciphertext, associatedData)
	if err != nil {
		return nil, errorWith(ErrAuthenticationFailed, "decrypt detached", err)
	}
	return plaintext, nil
}

// EncryptString UTF-8 text를 암호화하고 URL-safe base64 envelope를 반환한다.
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

// DecryptString URL-safe base64 envelope를 UTF-8 text로 복호화한다.
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

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
