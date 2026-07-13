package sqlkit_test

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/encrypt"
	"github.com/bluetape4k/bluetape-go/sqlkit"
)

var _ sql.Scanner = (*sqlkit.EncryptedBytesColumn)(nil)
var _ driver.Valuer = sqlkit.EncryptedBytesColumn{}

func TestEncryptedBytesColumnRoundTripNullAndStorageType(t *testing.T) {
	encryptor := newTestEncryptor(t)
	aad := []byte("tenant=blue:column=payload")
	for _, plaintext := range [][]byte{[]byte("secret payload"), {}, nil} {
		input := sqlkit.NewEncryptedBytesColumn(encryptor, aad)
		input.Data = plaintext
		input.Valid = true

		value, err := input.Value()
		if err != nil {
			t.Fatalf("Value(%q) failed: %v", plaintext, err)
		}
		ciphertext, ok := value.([]byte)
		if !ok {
			t.Fatalf("Value type = %T, want []byte", value)
		}

		output := sqlkit.NewEncryptedBytesColumn(encryptor, aad)
		if err := output.Scan(ciphertext); err != nil {
			t.Fatalf("Scan(%q) failed: %v", plaintext, err)
		}
		if !output.Valid || !bytes.Equal(output.Data, plaintext) {
			t.Fatalf("output = %#v, want valid %q", output, plaintext)
		}

		if err := output.Scan(nil); err != nil {
			t.Fatalf("Scan(nil) failed: %v", err)
		}
		if output.Valid || output.Data != nil {
			t.Fatalf("SQL NULL output = %#v, want invalid nil data", output)
		}
	}
}

func TestEncryptedBytesColumnUsesRandomCiphertext(t *testing.T) {
	encryptor := newTestEncryptor(t)
	column := sqlkit.NewEncryptedBytesColumn(encryptor, []byte("column=payload"))
	column.Data = []byte("same plaintext")
	column.Valid = true

	firstValue, err := column.Value()
	if err != nil {
		t.Fatalf("first Value failed: %v", err)
	}
	secondValue, err := column.Value()
	if err != nil {
		t.Fatalf("second Value failed: %v", err)
	}
	first := firstValue.([]byte)
	second := secondValue.([]byte)
	if bytes.Equal(first, second) {
		t.Fatal("repeated Value calls returned identical ciphertext")
	}
	for _, ciphertext := range [][]byte{first, second} {
		plaintext, err := encryptor.Decrypt(ciphertext, []byte("column=payload"))
		if err != nil || string(plaintext) != "same plaintext" {
			t.Fatalf("Decrypt = %q, %v", plaintext, err)
		}
	}
}

func TestEncryptedBytesColumnPreservesEncryptErrorsAndClearsPlaintext(t *testing.T) {
	encryptor := newTestEncryptor(t)
	aad := []byte("column=payload")
	sourceColumn := sqlkit.NewEncryptedBytesColumn(encryptor, aad)
	sourceColumn.Data = []byte("secret payload")
	sourceColumn.Valid = true
	value, err := sourceColumn.Value()
	if err != nil {
		t.Fatalf("source Value failed: %v", err)
	}
	ciphertext := value.([]byte)
	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0xff
	wrongEncryptor := newEncryptorFromByte(t, 0x24)

	tests := []struct {
		name      string
		column    sqlkit.EncryptedBytesColumn
		src       any
		wantCause error
	}{
		{name: "malformed", column: sqlkit.NewEncryptedBytesColumn(encryptor, aad), src: []byte("bad"), wantCause: encrypt.ErrMalformedCiphertext},
		{name: "tampered", column: sqlkit.NewEncryptedBytesColumn(encryptor, aad), src: tampered, wantCause: encrypt.ErrAuthenticationFailed},
		{name: "wrong key", column: sqlkit.NewEncryptedBytesColumn(wrongEncryptor, aad), src: ciphertext, wantCause: encrypt.ErrAuthenticationFailed},
		{name: "wrong AAD", column: sqlkit.NewEncryptedBytesColumn(encryptor, []byte("column=other")), src: ciphertext, wantCause: encrypt.ErrAuthenticationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.column.Data = []byte("stale plaintext")
			tt.column.Valid = true
			err := tt.column.Scan(tt.src)
			if !errors.Is(err, sqlkit.ErrInvalidColumnValue) || !errors.Is(err, tt.wantCause) {
				t.Fatalf("Scan error = %v, want ErrInvalidColumnValue and %v", err, tt.wantCause)
			}
			if tt.column.Valid || tt.column.Data != nil {
				t.Fatalf("failed Scan retained plaintext: %#v", tt.column)
			}
		})
	}

	column := sqlkit.NewEncryptedBytesColumn(encryptor, aad)
	column.Data = []byte("stale plaintext")
	column.Valid = true
	if err := column.Scan(int64(7)); !errors.Is(err, sqlkit.ErrInvalidColumnValue) {
		t.Fatalf("unsupported Scan error = %v", err)
	}
	if column.Valid || column.Data != nil {
		t.Fatalf("unsupported Scan retained plaintext: %#v", column)
	}
}

func TestEncryptedBytesColumnEnforcesLimits(t *testing.T) {
	if sqlkit.DefaultEncryptedColumnMaxPlaintextBytes != 1<<20 {
		t.Fatalf("plaintext default = %d", sqlkit.DefaultEncryptedColumnMaxPlaintextBytes)
	}
	if sqlkit.DefaultEncryptedColumnMaxCiphertextBytes != 2<<20 {
		t.Fatalf("ciphertext default = %d", sqlkit.DefaultEncryptedColumnMaxCiphertextBytes)
	}

	encryptor := newTestEncryptor(t)
	aad := []byte("column=payload")
	source := sqlkit.NewEncryptedBytesColumn(encryptor, aad)
	source.Data = []byte("payload")
	source.Valid = true
	value, err := source.Value()
	if err != nil {
		t.Fatalf("source Value failed: %v", err)
	}
	ciphertext := value.([]byte)

	scanTests := []struct {
		name   string
		plain  int
		cipher int
		want   error
	}{
		{name: "negative plaintext", plain: -1, want: sqlkit.ErrInvalidColumnValue},
		{name: "negative ciphertext", cipher: -1, want: sqlkit.ErrInvalidColumnValue},
		{name: "oversized plaintext", plain: len("payload") - 1, want: sqlkit.ErrColumnValueTooLarge},
		{name: "oversized ciphertext", cipher: len(ciphertext) - 1, want: sqlkit.ErrColumnValueTooLarge},
	}
	for _, tt := range scanTests {
		t.Run("scan "+tt.name, func(t *testing.T) {
			column := sqlkit.NewEncryptedBytesColumn(encryptor, aad)
			column.Data = []byte("stale")
			column.Valid = true
			column.MaxPlaintextBytes = tt.plain
			column.MaxCiphertextBytes = tt.cipher
			err := column.Scan(ciphertext)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Scan error = %v, want %v", err, tt.want)
			}
			if column.Valid || column.Data != nil {
				t.Fatalf("failed Scan retained plaintext: %#v", column)
			}
		})
	}

	valueTests := []struct {
		name   string
		plain  int
		cipher int
		want   error
	}{
		{name: "negative plaintext", plain: -1, want: sqlkit.ErrInvalidColumnValue},
		{name: "negative ciphertext", cipher: -1, want: sqlkit.ErrInvalidColumnValue},
		{name: "oversized plaintext", plain: len("payload") - 1, want: sqlkit.ErrColumnValueTooLarge},
		{name: "oversized ciphertext", cipher: 1, want: sqlkit.ErrColumnValueTooLarge},
	}
	for _, tt := range valueTests {
		t.Run("value "+tt.name, func(t *testing.T) {
			column := sqlkit.NewEncryptedBytesColumn(encryptor, aad)
			column.Data = []byte("payload")
			column.Valid = true
			column.MaxPlaintextBytes = tt.plain
			column.MaxCiphertextBytes = tt.cipher
			if _, err := column.Value(); !errors.Is(err, tt.want) {
				t.Fatalf("Value error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestEncryptedBytesColumnCopiesAADAndSource(t *testing.T) {
	encryptor := newTestEncryptor(t)
	originalAAD := []byte("column=payload")
	constructorAAD := append([]byte(nil), originalAAD...)
	input := sqlkit.NewEncryptedBytesColumn(encryptor, constructorAAD)
	for i := range constructorAAD {
		constructorAAD[i] = 'X'
	}
	input.Data = []byte("secret payload")
	input.Valid = true
	value, err := input.Value()
	if err != nil {
		t.Fatalf("Value after AAD mutation failed: %v", err)
	}
	ciphertext := value.([]byte)

	output := sqlkit.NewEncryptedBytesColumn(encryptor, originalAAD)
	if err := output.Scan(ciphertext); err != nil {
		t.Fatalf("Scan with original AAD failed: %v", err)
	}
	for i := range ciphertext {
		ciphertext[i] = 0
	}
	if string(output.Data) != "secret payload" {
		t.Fatalf("plaintext aliases ciphertext source: %q", output.Data)
	}
}

func TestEncryptedBytesColumnRedactsErrors(t *testing.T) {
	markers := []string{"plaintext-secret", "ciphertext-secret", "key-secret", "aad-secret"}
	encryptor := newTestEncryptor(t)
	column := sqlkit.NewEncryptedBytesColumn(encryptor, []byte(markers[3]))
	column.Data = []byte("stale")
	column.Valid = true
	err := column.Scan([]byte(markers[1]))
	if err == nil {
		t.Fatal("Scan error = nil")
	}
	for _, marker := range markers {
		if strings.Contains(err.Error(), marker) {
			t.Fatalf("error exposes %q: %v", marker, err)
		}
	}
}

func TestEncryptedBytesColumnZeroValue(t *testing.T) {
	var column sqlkit.EncryptedBytesColumn
	value, err := column.Value()
	if err != nil || value != nil {
		t.Fatalf("zero invalid Value = %v, %v", value, err)
	}

	column.Data = []byte("secret")
	column.Valid = true
	if _, err := column.Value(); !errors.Is(err, sqlkit.ErrInvalidColumnValue) || !errors.Is(err, encrypt.ErrInvalidKey) {
		t.Fatalf("unconfigured Value error = %v", err)
	}

	var scanned sqlkit.EncryptedBytesColumn
	if err := scanned.Scan([]byte("ciphertext")); !errors.Is(err, sqlkit.ErrInvalidColumnValue) || !errors.Is(err, encrypt.ErrInvalidKey) {
		t.Fatalf("unconfigured Scan error = %v", err)
	}
}

func TestEncryptedBytesColumnNilScanner(t *testing.T) {
	var column *sqlkit.EncryptedBytesColumn
	if err := column.Scan([]byte("ciphertext")); !errors.Is(err, sqlkit.ErrInvalidColumnValue) {
		t.Fatalf("nil Scan error = %v", err)
	}
}

func newTestEncryptor(t *testing.T) encrypt.Encryptor {
	t.Helper()
	return newEncryptorFromByte(t, 0x42)
}

func newEncryptorFromByte(t *testing.T, value byte) encrypt.Encryptor {
	t.Helper()
	encryptor, err := encrypt.New(bytes.Repeat([]byte{value}, 32))
	if err != nil {
		t.Fatalf("encrypt.New failed: %v", err)
	}
	return encryptor
}
