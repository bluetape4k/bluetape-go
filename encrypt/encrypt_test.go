package encrypt_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/encrypt"
	concurrencytest "github.com/bluetape4k/bluetape-go/testing/concurrency"
)

func TestEncryptDecryptBytesRoundTrip(t *testing.T) {
	key := testKey(32, 1)
	associatedData := []byte("tenant=blue:entity=invoice:column=payload")
	plaintext := []byte("secret payload")

	ciphertext, err := encrypt.Encrypt(key, plaintext, associatedData)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}

	decrypted, err := encrypt.Decrypt(key, ciphertext, associatedData)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptorSupportsAESKeySizes(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{name: "aes128", size: 16},
		{name: "aes192", size: 24},
		{name: "aes256", size: 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encryptor, err := encrypt.New(testKey(tt.size, byte(tt.size)))
			if err != nil {
				t.Fatalf("New(%d byte key) error = %v", tt.size, err)
			}
			ciphertext, err := encryptor.Encrypt([]byte("payload"), nil)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}
			plaintext, err := encryptor.Decrypt(ciphertext, nil)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}
			if string(plaintext) != "payload" {
				t.Fatalf("Decrypt() = %q, want payload", plaintext)
			}
		})
	}
}

func TestStringRoundTripUsesURLSafeEnvelope(t *testing.T) {
	key := testKey(32, 2)
	encryptor, err := encrypt.New(key)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	associatedData := []byte("tenant=blue")
	ciphertext, err := encryptor.EncryptString("안녕 bluetape-go", associatedData)
	if err != nil {
		t.Fatalf("EncryptString() error = %v", err)
	}
	if strings.Contains(ciphertext, "+") || strings.Contains(ciphertext, "/") || strings.Contains(ciphertext, "=") {
		t.Fatalf("ciphertext string is not raw URL-safe base64: %q", ciphertext)
	}
	if _, err := base64.RawURLEncoding.DecodeString(ciphertext); err != nil {
		t.Fatalf("ciphertext is not raw URL-safe base64: %v", err)
	}

	plaintext, err := encryptor.DecryptString(ciphertext, associatedData)
	if err != nil {
		t.Fatalf("DecryptString() error = %v", err)
	}
	if plaintext != "안녕 bluetape-go" {
		t.Fatalf("DecryptString() = %q", plaintext)
	}
}

func TestCiphertextEnvelopeIsVersioned(t *testing.T) {
	plaintext := []byte("payload")
	ciphertext, err := encrypt.Encrypt(testKey(32, 3), plaintext, nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if len(ciphertext) < 7 {
		t.Fatalf("ciphertext too short: %d", len(ciphertext))
	}
	if got := string(ciphertext[:5]); got != "BTENC" {
		t.Fatalf("magic = %q, want BTENC", got)
	}
	if ciphertext[5] != 1 {
		t.Fatalf("version = %d, want 1", ciphertext[5])
	}
	if ciphertext[6] != 1 {
		t.Fatalf("algorithm = %d, want 1", ciphertext[6])
	}
	if want := len([]byte("BTENC")) + 2 + 28 + len(plaintext); len(ciphertext) != want {
		t.Fatalf("ciphertext length = %d, want %d", len(ciphertext), want)
	}
}

func TestDecryptRejectsWrongKeyAssociatedDataAndTamper(t *testing.T) {
	key := testKey(32, 4)
	ciphertext, err := encrypt.Encrypt(key, []byte("payload"), []byte("ad"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	tests := []struct {
		name           string
		key            []byte
		ciphertext     []byte
		associatedData []byte
	}{
		{name: "wrong key", key: testKey(32, 5), ciphertext: ciphertext, associatedData: []byte("ad")},
		{name: "wrong associated data", key: key, ciphertext: ciphertext, associatedData: []byte("other")},
		{name: "tamper", key: key, ciphertext: tamper(ciphertext), associatedData: []byte("ad")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encrypt.Decrypt(tt.key, tt.ciphertext, tt.associatedData)
			if !errors.Is(err, encrypt.ErrAuthenticationFailed) {
				t.Fatalf("Decrypt() error = %v, want ErrAuthenticationFailed", err)
			}
		})
	}
}

func TestDecryptRejectsMalformedEnvelope(t *testing.T) {
	key := testKey(32, 6)
	valid, err := encrypt.Encrypt(key, []byte("payload"), nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	tests := []struct {
		name       string
		ciphertext []byte
	}{
		{name: "empty", ciphertext: nil},
		{name: "short", ciphertext: []byte("BTENC")},
		{name: "wrong magic", ciphertext: append([]byte("XXXXX"), valid[5:]...)},
		{name: "unsupported version", ciphertext: withByte(valid, 5, 2)},
		{name: "unsupported algorithm", ciphertext: withByte(valid, 6, 2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := encrypt.Decrypt(key, tt.ciphertext, nil)
			if !errors.Is(err, encrypt.ErrMalformedCiphertext) {
				t.Fatalf("Decrypt() error = %v, want ErrMalformedCiphertext", err)
			}
		})
	}
}

func TestDecryptStringRejectsMalformedBase64(t *testing.T) {
	encryptor, err := encrypt.New(testKey(32, 7))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = encryptor.DecryptString("not raw base64!!!", nil)
	if !errors.Is(err, encrypt.ErrMalformedCiphertext) {
		t.Fatalf("DecryptString() error = %v, want ErrMalformedCiphertext", err)
	}
}

func TestStringHelpersRejectInvalidUTF8(t *testing.T) {
	key := testKey(32, 7)
	encryptor, err := encrypt.New(key)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := encryptor.EncryptString(string([]byte{0xff}), nil); !errors.Is(err, encrypt.ErrInvalidOptions) {
		t.Fatalf("EncryptString() error = %v, want ErrInvalidOptions", err)
	}

	ciphertext, err := encrypt.Encrypt(key, []byte{0xff}, nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(ciphertext)
	if _, err := encryptor.DecryptString(token, nil); !errors.Is(err, encrypt.ErrMalformedCiphertext) {
		t.Fatalf("DecryptString() error = %v, want ErrMalformedCiphertext", err)
	}
}

func TestNewRejectsInvalidKeysAndOptions(t *testing.T) {
	for _, key := range [][]byte{nil, {}, testKey(15, 1), testKey(17, 1), testKey(33, 1)} {
		if _, err := encrypt.New(key); !errors.Is(err, encrypt.ErrInvalidKey) {
			t.Fatalf("New(%d byte key) error = %v, want ErrInvalidKey", len(key), err)
		}
	}

	if _, err := encrypt.New(testKey(32, 8), nil); !errors.Is(err, encrypt.ErrInvalidOptions) {
		t.Fatalf("New(nil option) error = %v, want ErrInvalidOptions", err)
	}
}

func TestNewCopiesKeyMaterial(t *testing.T) {
	key := testKey(32, 9)
	encryptor, err := encrypt.New(key)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	for i := range key {
		key[i] = 0
	}

	ciphertext, err := encryptor.Encrypt([]byte("payload"), nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	plaintext, err := encryptor.Decrypt(ciphertext, nil)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if string(plaintext) != "payload" {
		t.Fatalf("Decrypt() = %q, want payload", plaintext)
	}
}

func TestZeroValueEncryptorFailsSafely(t *testing.T) {
	var encryptor encrypt.Encryptor
	if _, err := encryptor.Encrypt([]byte("payload"), nil); !errors.Is(err, encrypt.ErrInvalidKey) {
		t.Fatalf("Encrypt() error = %v, want ErrInvalidKey", err)
	}
	if _, err := encryptor.Decrypt([]byte("ciphertext"), nil); !errors.Is(err, encrypt.ErrInvalidKey) {
		t.Fatalf("Decrypt() error = %v, want ErrInvalidKey", err)
	}
}

func TestErrorsDoNotExposeSensitiveMaterial(t *testing.T) {
	secretKey := []byte("secret-key")
	secretCiphertext := []byte("BTENC-secret-ciphertext")
	secretAD := []byte("secret-associated-data")
	secretPlaintext := []byte("secret-plaintext")

	_, err := encrypt.Encrypt(secretKey, secretPlaintext, secretAD)
	assertSafeError(t, err, secretKey, secretPlaintext, secretAD)

	_, err = encrypt.Decrypt(testKey(32, 10), secretCiphertext, secretAD)
	assertSafeError(t, err, secretCiphertext, secretAD)
}

func TestSharedEncryptorConcurrencyStress(t *testing.T) {
	encryptor, err := encrypt.New(testKey(32, 11))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tester := concurrencytest.NewGoroutineStressTester(concurrencytest.Options{
		Workers:       8,
		RoundsPerTask: 64,
		Timeout:       5 * time.Second,
	})

	tester.RunT(t, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		associatedData := []byte("tenant=blue")
		ciphertext, err := encryptor.Encrypt([]byte("payload"), associatedData)
		if err != nil {
			return err
		}
		plaintext, err := encryptor.Decrypt(ciphertext, associatedData)
		if err != nil {
			return err
		}
		if string(plaintext) != "payload" {
			return errors.New("unexpected plaintext")
		}
		return nil
	})
}

func TestAsyncJobTesterEncryptionJobs(t *testing.T) {
	encryptor, err := encrypt.New(testKey(32, 12))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	tester := concurrencytest.NewAsyncJobTester(concurrencytest.Options{
		Workers:       4,
		RoundsPerTask: 32,
		Timeout:       5 * time.Second,
	})

	tester.RunT(t, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ciphertext, err := encryptor.EncryptString("payload", []byte("async-job"))
		if err != nil {
			return err
		}
		plaintext, err := encryptor.DecryptString(ciphertext, []byte("async-job"))
		if err != nil {
			return err
		}
		if plaintext != "payload" {
			return errors.New("unexpected plaintext")
		}
		return nil
	})
}

func testKey(size int, seed byte) []byte {
	key := make([]byte, size)
	for i := range key {
		key[i] = seed + byte(i)
	}
	return key
}

func tamper(value []byte) []byte {
	out := append([]byte(nil), value...)
	out[len(out)-1] ^= 0x01
	return out
}

func withByte(value []byte, index int, b byte) []byte {
	out := append([]byte(nil), value...)
	out[index] = b
	return out
}

func assertSafeError(t *testing.T, err error, secrets ...[]byte) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	message := err.Error()
	for _, secret := range secrets {
		if len(secret) > 0 && strings.Contains(message, string(secret)) {
			t.Fatalf("error %q exposes secret %q", message, string(secret))
		}
	}
}
