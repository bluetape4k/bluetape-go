package sqlkit

import (
	"database/sql/driver"

	"github.com/bluetape4k/bluetape-go/encrypt"
)

const (
	// DefaultEncryptedColumnMaxPlaintextBytes는 plaintext 기본 한도를 1 MiB로 제한한다.
	DefaultEncryptedColumnMaxPlaintextBytes = 1 << 20
	// DefaultEncryptedColumnMaxCiphertextBytes는 저장할 ciphertext 기본 한도를 2 MiB로 제한한다.
	DefaultEncryptedColumnMaxCiphertextBytes = 2 << 20
)

type encryptedColumnConfig struct {
	encryptor      encrypt.Encryptor
	associatedData []byte
}

// EncryptedBytesColumn은 임의 plaintext byte를 binary envelope로 암호화해 저장한다.
//
// zero value는 SQL NULL을 안전하게 나타낸다. non-NULL data를 Scan하거나 Value로 만들기 전에
// NewEncryptedBytesColumn을 사용한다. Column value는 row별 mutable state이므로 동시에 변경하면 안 된다.
type EncryptedBytesColumn struct {
	Data               []byte
	Valid              bool
	MaxPlaintextBytes  int
	MaxCiphertextBytes int
	config             encryptedColumnConfig
}

// NewEncryptedBytesColumn은 byte column을 생성하고 associatedData를 복사한다.
func NewEncryptedBytesColumn(encryptor encrypt.Encryptor, associatedData []byte) EncryptedBytesColumn {
	return EncryptedBytesColumn{config: encryptedColumnConfig{
		encryptor:      encryptor,
		associatedData: append([]byte(nil), associatedData...),
	}}
}

// Scan은 nil, string, []byte database value를 복호화한다.
//
// Scan은 validation 전에 기존 plaintext를 지우고 driver 소유 input을 복사하며,
// 복호화와 size validation이 성공한 뒤에만 Data를 공개한다.
func (c *EncryptedBytesColumn) Scan(src any) error {
	if c == nil {
		return newColumnError(ErrInvalidColumnValue, "scan encrypted bytes", nil)
	}
	c.Data, c.Valid = nil, false
	if src == nil {
		return nil
	}
	ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "scan encrypted bytes ciphertext limit")
	if err != nil {
		return err
	}
	plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "scan encrypted bytes plaintext limit")
	if err != nil {
		return err
	}
	raw, present, err := boundedCopiedColumnSource(src, ciphertextLimit, "scan encrypted bytes ciphertext")
	if err != nil || !present {
		return err
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

// Value는 Data를 binary envelope로 암호화하거나 Valid가 false이면 nil을 반환한다.
//
// Value는 non-NULL 값에 대해 []byte를 반환한다. 반복 호출은 독립적인 random nonce를 사용하므로
// 같은 plaintext라도 다른 ciphertext를 반환할 수 있다.
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

// EncryptedStringColumn은 UTF-8 plaintext를 raw URL-safe base64 text로 암호화해 저장한다.
//
// zero value는 SQL NULL을 안전하게 나타낸다. non-NULL data를 Scan하거나 Value로 만들기 전에
// NewEncryptedStringColumn을 사용한다. Column value는 row별 mutable state이므로 동시에 변경하면 안 된다.
type EncryptedStringColumn struct {
	Data               string
	Valid              bool
	MaxPlaintextBytes  int
	MaxCiphertextBytes int
	config             encryptedColumnConfig
}

// NewEncryptedStringColumn은 string column을 생성하고 associatedData를 복사한다.
func NewEncryptedStringColumn(encryptor encrypt.Encryptor, associatedData []byte) EncryptedStringColumn {
	return EncryptedStringColumn{config: encryptedColumnConfig{
		encryptor:      encryptor,
		associatedData: append([]byte(nil), associatedData...),
	}}
}

// Scan은 nil, string, []byte database value를 복호화한다.
//
// non-NULL input은 encrypt.Encryptor.EncryptString이 생성한 raw URL-safe base64 envelope를 포함해야 한다.
// Scan은 작업 전에 기존 plaintext를 지운다.
func (c *EncryptedStringColumn) Scan(src any) error {
	if c == nil {
		return newColumnError(ErrInvalidColumnValue, "scan encrypted string", nil)
	}
	c.Data, c.Valid = "", false
	if src == nil {
		return nil
	}
	ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "scan encrypted string ciphertext limit")
	if err != nil {
		return err
	}
	plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "scan encrypted string plaintext limit")
	if err != nil {
		return err
	}
	raw, present, err := boundedCopiedColumnSource(src, ciphertextLimit, "scan encrypted string ciphertext")
	if err != nil || !present {
		return err
	}
	plaintext, err := c.config.encryptor.DecryptString(string(raw), c.config.associatedData)
	if err != nil {
		return newColumnError(ErrInvalidColumnValue, "scan encrypted string", err)
	}
	if len(plaintext) > plaintextLimit {
		return newColumnError(ErrColumnValueTooLarge, "scan encrypted string plaintext", nil)
	}
	c.Data, c.Valid = plaintext, true
	return nil
}

// Value는 Data를 raw URL-safe base64 text로 암호화하거나 Valid가 false이면 nil을 반환한다.
//
// Value는 non-NULL 값에 대해 string을 반환한다. 반복 호출은 독립적인 random nonce를 사용하므로
// 같은 plaintext라도 다른 ciphertext를 반환할 수 있다.
func (c EncryptedStringColumn) Value() (value driver.Value, err error) {
	if !c.Valid {
		return nil, nil
	}
	defer recoverColumnPanic("encrypt string", &err)
	plaintextLimit, err := effectiveColumnLimit(c.MaxPlaintextBytes, DefaultEncryptedColumnMaxPlaintextBytes, "encrypt string plaintext limit")
	if err != nil {
		return nil, err
	}
	if len(c.Data) > plaintextLimit {
		return nil, newColumnError(ErrColumnValueTooLarge, "encrypt string plaintext", nil)
	}
	ciphertextLimit, err := effectiveColumnLimit(c.MaxCiphertextBytes, DefaultEncryptedColumnMaxCiphertextBytes, "encrypt string ciphertext limit")
	if err != nil {
		return nil, err
	}
	ciphertext, err := c.config.encryptor.EncryptString(c.Data, c.config.associatedData)
	if err != nil {
		return nil, newColumnError(ErrInvalidColumnValue, "encrypt string", err)
	}
	if len(ciphertext) > ciphertextLimit {
		return nil, newColumnError(ErrColumnValueTooLarge, "encrypt string ciphertext", nil)
	}
	return ciphertext, nil
}
