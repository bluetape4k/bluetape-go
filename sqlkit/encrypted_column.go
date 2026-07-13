package sqlkit

import (
	"database/sql/driver"

	"github.com/bluetape4k/bluetape-go/encrypt"
)

const (
	// DefaultEncryptedColumnMaxPlaintextBytes limits plaintext to 1 MiB by default.
	DefaultEncryptedColumnMaxPlaintextBytes = 1 << 20
	// DefaultEncryptedColumnMaxCiphertextBytes limits stored ciphertext to 2 MiB by default.
	DefaultEncryptedColumnMaxCiphertextBytes = 2 << 20
)

type encryptedColumnConfig struct {
	encryptor      encrypt.Encryptor
	associatedData []byte
}

// EncryptedBytesColumn stores arbitrary plaintext bytes encrypted as a binary envelope.
//
// The zero value safely represents SQL NULL. Use NewEncryptedBytesColumn
// before scanning or valuing non-NULL data. Column values are mutable per-row
// state and must not be mutated concurrently.
type EncryptedBytesColumn struct {
	Data               []byte
	Valid              bool
	MaxPlaintextBytes  int
	MaxCiphertextBytes int
	config             encryptedColumnConfig
}

// NewEncryptedBytesColumn creates a byte column and copies associatedData.
func NewEncryptedBytesColumn(encryptor encrypt.Encryptor, associatedData []byte) EncryptedBytesColumn {
	return EncryptedBytesColumn{config: encryptedColumnConfig{
		encryptor:      encryptor,
		associatedData: append([]byte(nil), associatedData...),
	}}
}

// Scan decrypts a nil, string, or []byte database value.
//
// Scan clears prior plaintext before validation, copies driver-owned input,
// and publishes Data only after decryption and size validation succeed.
func (c *EncryptedBytesColumn) Scan(src any) error {
	if c == nil {
		return newColumnError(ErrInvalidColumnValue, "scan encrypted bytes", nil)
	}
	c.Data, c.Valid = nil, false
	raw, present, err := copiedColumnSource(src, "scan encrypted bytes")
	if err != nil || !present {
		return err
	}
	ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "scan encrypted bytes ciphertext limit")
	if err != nil {
		return err
	}
	plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "scan encrypted bytes plaintext limit")
	if err != nil {
		return err
	}
	if len(raw) > ciphertextLimit {
		return newColumnError(ErrColumnValueTooLarge, "scan encrypted bytes ciphertext", nil)
	}
	plaintext, err := c.config.encryptor.Decrypt(raw, c.config.associatedData)
	if err != nil {
		return newColumnError(ErrInvalidColumnValue, "scan encrypted bytes", err)
	}
	if len(plaintext) > plaintextLimit {
		return newColumnError(ErrColumnValueTooLarge, "scan encrypted bytes plaintext", nil)
	}
	c.Data, c.Valid = append([]byte(nil), plaintext...), true
	return nil
}

// Value encrypts Data as a binary envelope or returns nil when Valid is false.
//
// Value returns []byte for non-NULL values. Repeated calls use independent
// random nonces and may return different ciphertext for the same plaintext.
func (c EncryptedBytesColumn) Value() (value driver.Value, err error) {
	if !c.Valid {
		return nil, nil
	}
	defer recoverColumnPanic("encrypt bytes", &err)
	plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "encrypt bytes plaintext limit")
	if err != nil {
		return nil, err
	}
	if len(c.Data) > plaintextLimit {
		return nil, newColumnError(ErrColumnValueTooLarge, "encrypt bytes plaintext", nil)
	}
	ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "encrypt bytes ciphertext limit")
	if err != nil {
		return nil, err
	}
	ciphertext, err := c.config.encryptor.Encrypt(c.Data, c.config.associatedData)
	if err != nil {
		return nil, newColumnError(ErrInvalidColumnValue, "encrypt bytes", err)
	}
	if len(ciphertext) > ciphertextLimit {
		return nil, newColumnError(ErrColumnValueTooLarge, "encrypt bytes ciphertext", nil)
	}
	return ciphertext, nil
}
