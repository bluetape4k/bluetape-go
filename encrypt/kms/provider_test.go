package kms

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/bluetape4k/bluetape-go/encrypt"
)

const testKeyID = "arn:aws:kms:ap-northeast-2:123456789012:key/demo"

type fakeClient struct {
	mu sync.Mutex

	generatePlaintext []byte
	generateBlob      []byte
	generateErr       error
	generateBlock     <-chan struct{}
	decryptPlaintext  []byte
	decryptErr        error
	decryptBlock      <-chan struct{}

	generateCount          int
	decryptCount           int
	lastGenerate           *kms.GenerateDataKeyInput
	lastDecrypt            *kms.DecryptInput
	lastDecryptBlob        []byte
	lastGeneratedPlaintext []byte
	lastGeneratedBlob      []byte
	lastDecryptedPlaintext []byte
}

func newFakeClient(plaintext, blob []byte) *fakeClient {
	return &fakeClient{
		generatePlaintext: append([]byte(nil), plaintext...),
		generateBlob:      append([]byte(nil), blob...),
		decryptPlaintext:  append([]byte(nil), plaintext...),
	}
}

func (f *fakeClient) GenerateDataKey(ctx context.Context, input *kms.GenerateDataKeyInput, _ ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	f.mu.Lock()
	f.generateCount++
	f.lastGenerate = cloneGenerateInput(input)
	plaintext := append([]byte(nil), f.generatePlaintext...)
	blob := append([]byte(nil), f.generateBlob...)
	err := f.generateErr
	block := f.generateBlock
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	out := &kms.GenerateDataKeyOutput{Plaintext: plaintext, CiphertextBlob: blob}
	f.mu.Lock()
	f.lastGeneratedPlaintext = out.Plaintext
	f.lastGeneratedBlob = out.CiphertextBlob
	f.mu.Unlock()
	return out, err
}

func (f *fakeClient) Decrypt(ctx context.Context, input *kms.DecryptInput, _ ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	f.mu.Lock()
	f.decryptCount++
	f.lastDecrypt = cloneDecryptInput(input)
	f.lastDecryptBlob = input.CiphertextBlob
	plaintext := append([]byte(nil), f.decryptPlaintext...)
	err := f.decryptErr
	block := f.decryptBlock
	f.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	out := &kms.DecryptOutput{Plaintext: plaintext}
	f.mu.Lock()
	f.lastDecryptedPlaintext = out.Plaintext
	f.mu.Unlock()
	return out, err
}

func (f *fakeClient) counts() (generate, decrypt int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.generateCount, f.decryptCount
}

func (f *fakeClient) resetCounts() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.generateCount = 0
	f.decryptCount = 0
	f.lastGenerate = nil
	f.lastDecrypt = nil
	f.lastDecryptBlob = nil
}

func (f *fakeClient) returnedGeneratePlaintext() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.lastGeneratedPlaintext...)
}

func (f *fakeClient) returnedGenerateBlob() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.lastGeneratedBlob...)
}

func (f *fakeClient) returnedDecryptPlaintext() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]byte(nil), f.lastDecryptedPlaintext...)
}

func (f *fakeClient) generatedInput() *kms.GenerateDataKeyInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneGenerateInput(f.lastGenerate)
}

func (f *fakeClient) decryptedInput() *kms.DecryptInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return cloneDecryptInput(f.lastDecrypt)
}

func (f *fakeClient) decryptedBlobReference() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastDecryptBlob
}

func cloneGenerateInput(input *kms.GenerateDataKeyInput) *kms.GenerateDataKeyInput {
	if input == nil {
		return nil
	}
	var keyID *string
	if input.KeyId != nil {
		value := *input.KeyId
		keyID = &value
	}
	contextCopy := make(map[string]string, len(input.EncryptionContext))
	for key, value := range input.EncryptionContext {
		contextCopy[key] = value
	}
	return &kms.GenerateDataKeyInput{KeyId: keyID, KeySpec: input.KeySpec, EncryptionContext: contextCopy}
}

func cloneDecryptInput(input *kms.DecryptInput) *kms.DecryptInput {
	if input == nil {
		return nil
	}
	var keyID *string
	if input.KeyId != nil {
		value := *input.KeyId
		keyID = &value
	}
	contextCopy := make(map[string]string, len(input.EncryptionContext))
	for key, value := range input.EncryptionContext {
		contextCopy[key] = value
	}
	return &kms.DecryptInput{KeyId: keyID, CiphertextBlob: append([]byte(nil), input.CiphertextBlob...), EncryptionContext: contextCopy}
}

func mustProvider(t *testing.T, client Client, keyID string, contextValues map[string]string) *Provider {
	t.Helper()
	provider, err := New(client, keyID, WithEncryptionContext(contextValues))
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func mustEnvelopeBytes(t *testing.T, envelope Envelope) []byte {
	t.Helper()
	data, err := envelope.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testEnvelope() Envelope {
	return Envelope{
		Version:           EnvelopeVersion,
		Algorithm:         AlgorithmAES256GCM,
		KeyID:             testKeyID,
		EncryptedDataKey:  bytes.Repeat([]byte{1}, 32),
		EncryptionContext: map[string]string{"tenant": "blue"},
		Nonce:             bytes.Repeat([]byte{2}, 12),
		Ciphertext:        bytes.Repeat([]byte{3}, 16),
	}
}

func assertAllZero(t *testing.T, value []byte) {
	t.Helper()
	if !bytes.Equal(value, make([]byte, len(value))) {
		t.Fatalf("buffer was not zeroed: %x", value)
	}
}

type nilMapClient map[string]string

func (nilMapClient) GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	return nil, nil
}

func (nilMapClient) Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	return nil, nil
}

type nilFuncClient func()

func (nilFuncClient) GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	return nil, nil
}

func (nilFuncClient) Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	return nil, nil
}

type nilSliceClient []byte

func (nilSliceClient) GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	return nil, nil
}

func (nilSliceClient) Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	return nil, nil
}

type nilChanClient chan struct{}

func (nilChanClient) GenerateDataKey(context.Context, *kms.GenerateDataKeyInput, ...func(*kms.Options)) (*kms.GenerateDataKeyOutput, error) {
	return nil, nil
}

func (nilChanClient) Decrypt(context.Context, *kms.DecryptInput, ...func(*kms.Options)) (*kms.DecryptOutput, error) {
	return nil, nil
}

type checkpointContext struct {
	context.Context
	mu       sync.Mutex
	calls    int
	cancelAt int
	err      error
}

func (ctx *checkpointContext) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.calls++
	if ctx.cancelAt > 0 && ctx.calls >= ctx.cancelAt {
		return ctx.err
	}
	return nil
}

func TestParseEnvelopeRejectsMissingFields(t *testing.T) {
	_, err := ParseEnvelope([]byte("BTKMS{}"))
	if !errors.Is(err, ErrMalformedEnvelope) {
		t.Fatalf("ParseEnvelope() error = %v, want ErrMalformedEnvelope", err)
	}
}

func TestEnvelopeFixturesUseIndependentBytes(t *testing.T) {
	envelope := Envelope{
		Version:           EnvelopeVersion,
		Algorithm:         AlgorithmAES256GCM,
		KeyID:             "arn:aws:kms:ap-northeast-2:123456789012:key/demo",
		EncryptedDataKey:  bytes.Repeat([]byte{1}, 32),
		EncryptionContext: map[string]string{"tenant": "blue"},
		Nonce:             bytes.Repeat([]byte{2}, 12),
		Ciphertext:        bytes.Repeat([]byte{3}, 16),
	}
	wire, err := envelope.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseEnvelope(wire)
	if err != nil {
		t.Fatal(err)
	}
	parsed.EncryptedDataKey[0] = 9
	parsed.Nonce[0] = 9
	parsed.EncryptionContext["tenant"] = "green"
	if envelope.EncryptedDataKey[0] != 1 || envelope.Nonce[0] != 2 || envelope.EncryptionContext["tenant"] != "blue" {
		t.Fatal("parsed envelope shares mutable state")
	}
}

func TestProviderRoundTripUsesExactMetadataAndZeroesKeys(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	fake := newFakeClient(key, []byte("encrypted-data-key"))
	provider := mustProvider(t, fake, testKeyID, map[string]string{"tenant": "blue", "Tenant": "case-sensitive"})

	envelopeBytes, err := provider.Encrypt(context.Background(), []byte("payload"), []byte("record-v1"))
	if err != nil {
		t.Fatal(err)
	}
	if generate, decrypt := fake.counts(); generate != 1 || decrypt != 0 {
		t.Fatalf("KMS calls after Encrypt = %d/%d, want 1/0", generate, decrypt)
	}
	generated := fake.generatedInput()
	if generated == nil || generated.KeyId == nil || *generated.KeyId != testKeyID || generated.KeySpec != types.DataKeySpecAes256 {
		t.Fatalf("GenerateDataKey input = %#v", generated)
	}
	if !reflect.DeepEqual(generated.EncryptionContext, map[string]string{"tenant": "blue", "Tenant": "case-sensitive"}) {
		t.Fatalf("GenerateDataKey context = %#v", generated.EncryptionContext)
	}
	assertAllZero(t, fake.returnedGeneratePlaintext())
	assertAllZero(t, fake.returnedGenerateBlob())

	plaintext, err := provider.Decrypt(context.Background(), envelopeBytes, []byte("record-v1"))
	if err != nil || string(plaintext) != "payload" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	if generate, decrypt := fake.counts(); generate != 1 || decrypt != 1 {
		t.Fatalf("KMS calls after Decrypt = %d/%d, want 1/1", generate, decrypt)
	}
	assertAllZero(t, fake.returnedDecryptPlaintext())
	assertAllZero(t, fake.decryptedBlobReference())
	decrypted := fake.decryptedInput()
	if decrypted == nil || decrypted.KeyId == nil || *decrypted.KeyId != testKeyID {
		t.Fatalf("Decrypt input = %#v", decrypted)
	}
	if !reflect.DeepEqual(decrypted.EncryptionContext, map[string]string{"tenant": "blue", "Tenant": "case-sensitive"}) {
		t.Fatalf("Decrypt context = %#v", decrypted.EncryptionContext)
	}
}

func TestProviderConstructorRejectsNilAndCopiesConfiguration(t *testing.T) {
	if _, err := New(nil, testKeyID); !errors.Is(err, ErrNilClient) {
		t.Fatalf("nil client error = %v", err)
	}
	if _, err := New(&fakeClient{}, " "); !errors.Is(err, ErrInvalidKeyID) {
		t.Fatalf("blank key ID error = %v", err)
	}
	if _, err := New(&fakeClient{}, string([]byte{0xff})); !errors.Is(err, ErrInvalidKeyID) {
		t.Fatalf("invalid UTF-8 key ID error = %v", err)
	}
	if _, err := New(&fakeClient{}, strings.Repeat("k", MaxKeyIDSize+1)); !errors.Is(err, ErrInvalidKeyID) {
		t.Fatalf("oversized key ID error = %v", err)
	}
	if _, err := New(&fakeClient{}, testKeyID, nil); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("nil option error = %v", err)
	}

	values := map[string]string{"tenant": "blue"}
	provider, err := New(&fakeClient{}, testKeyID, WithEncryptionContext(values))
	if err != nil {
		t.Fatal(err)
	}
	values["tenant"] = "changed"
	values["new"] = "value"
	if !reflect.DeepEqual(provider.encryptionContext, map[string]string{"tenant": "blue"}) {
		t.Fatalf("provider context = %#v", provider.encryptionContext)
	}
	if _, err := New(&fakeClient{}, testKeyID, WithEncryptionContext(map[string]string{"tenant": "blue"}), WithEncryptionContext(map[string]string{"tenant": "green"})); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("duplicate context option error = %v", err)
	}
	if _, err := New(&fakeClient{}, testKeyID, WithEncryptionContext(map[string]string{"": "bad"})); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("empty context key error = %v", err)
	}
	if _, err := New(&fakeClient{}, testKeyID, WithEncryptionContext(map[string]string{"bad": string([]byte{0xff})})); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("invalid context value error = %v", err)
	}
	if _, err := New(&fakeClient{}, testKeyID, WithEncryptionContext(map[string]string{"k": strings.Repeat("v", MaxContextSize-1)})); err != nil {
		t.Fatalf("context exact byte boundary error = %v", err)
	}
	if _, err := New(&fakeClient{}, testKeyID, WithEncryptionContext(map[string]string{"k": strings.Repeat("v", MaxContextSize+1)})); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("context over byte boundary error = %v", err)
	}
	tooMany := make(map[string]string, MaxContextEntries+1)
	for i := 0; i < MaxContextEntries+1; i++ {
		tooMany[fmt.Sprintf("key-%d", i)] = "value"
	}
	if _, err := New(&fakeClient{}, testKeyID, WithEncryptionContext(tooMany)); !errors.Is(err, ErrInvalidOptions) {
		t.Fatalf("context entry boundary error = %v", err)
	}
}

func TestProviderZeroValueAndTypedNilClientsFailWithoutCalls(t *testing.T) {
	var provider Provider
	if _, err := provider.Encrypt(context.Background(), nil, nil); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("zero-value Encrypt() error = %v", err)
	}
	if _, err := provider.Decrypt(context.Background(), nil, nil); !errors.Is(err, ErrInvalidProvider) {
		t.Fatalf("zero-value Decrypt() error = %v", err)
	}

	var typedNil *fakeClient
	if _, err := New(typedNil, testKeyID); !errors.Is(err, ErrNilClient) {
		t.Fatalf("typed-nil pointer error = %v", err)
	}
	for _, client := range []Client{
		nilMapClient(nil), nilFuncClient(nil), nilSliceClient(nil), nilChanClient(nil),
	} {
		if _, err := New(client, testKeyID); !errors.Is(err, ErrNilClient) {
			t.Fatalf("typed-nil %T error = %v", client, err)
		}
	}
}

func TestProviderPreflightBoundsAvoidKMSCalls(t *testing.T) {
	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, nil)
	for _, test := range []struct {
		name string
		call func() error
	}{
		{name: "plaintext", call: func() error {
			_, err := provider.Encrypt(context.Background(), make([]byte, MaxPlaintextSize+1), nil)
			return err
		}},
		{name: "associated data encrypt", call: func() error {
			_, err := provider.Encrypt(context.Background(), nil, make([]byte, MaxAssociatedDataSize+1))
			return err
		}},
		{name: "associated data decrypt", call: func() error {
			_, err := provider.Decrypt(context.Background(), []byte("not-parsed"), make([]byte, MaxAssociatedDataSize+1))
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); !errors.Is(err, ErrInputTooLarge) {
				t.Fatalf("error = %v, want ErrInputTooLarge", err)
			}
			if generate, decrypt := fake.counts(); generate != 0 || decrypt != 0 {
				t.Fatalf("KMS calls = %d/%d, want 0/0", generate, decrypt)
			}
		})
	}
}

func TestProviderRejectsOversizedCiphertextBeforeKMS(t *testing.T) {
	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, nil)
	wire := wireEnvelope{
		Version:           EnvelopeVersion,
		Algorithm:         AlgorithmAES256GCM,
		KeyID:             testKeyID,
		EncryptedDataKey:  base64.StdEncoding.EncodeToString([]byte("encrypted-data-key")),
		EncryptionContext: []wireContextEntry{},
		Nonce:             base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, gcmNonceSize)),
		Ciphertext:        strings.Repeat("A", base64.StdEncoding.EncodedLen(MaxPlaintextSize+gcmOverhead)+4),
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(envelopeMagic), encoded...)
	if _, err := provider.Decrypt(context.Background(), data, nil); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("Decrypt() error = %v, want ErrInputTooLarge", err)
	}
	if _, decrypt := fake.counts(); decrypt != 0 {
		t.Fatalf("KMS Decrypt calls = %d, want 0", decrypt)
	}
}

func TestProviderTreatsNilContextAsBackground(t *testing.T) {
	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, nil)
	//nolint:staticcheck // nil context normalization is an explicit provider contract.
	if _, err := provider.Encrypt(nil, []byte("payload"), nil); err != nil {
		t.Fatalf("Encrypt(nil context) error = %v", err)
	}
}

func TestProviderBoundaryPlaintextSucceeds(t *testing.T) {
	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, nil)
	maximum := make([]byte, MaxPlaintextSize)
	wire, err := provider.Encrypt(context.Background(), maximum, nil)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseEnvelope(wire)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(parsed.Ciphertext) - gcmOverhead; got != MaxPlaintextSize {
		t.Fatalf("ciphertext-derived plaintext length = %d, want %d", got, MaxPlaintextSize)
	}
}

func TestProviderKMSFailuresAndOutputValidation(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeClient)
		want      error
		wantCause error
	}{
		{name: "generate error", configure: func(fake *fakeClient) { fake.generateErr = fmt.Errorf("secret AWS request 123") }, want: ErrKMSOperation},
		{name: "nil output", configure: func(fake *fakeClient) { fake.generatePlaintext = nil; fake.generateBlob = nil }, want: ErrInvalidDataKey},
		{name: "wrong plaintext", configure: func(fake *fakeClient) { fake.generatePlaintext = bytes.Repeat([]byte{1}, 31) }, want: ErrInvalidDataKey},
		{name: "empty blob", configure: func(fake *fakeClient) { fake.generateBlob = nil }, want: ErrInvalidDataKey},
		{name: "oversized blob", configure: func(fake *fakeClient) { fake.generateBlob = bytes.Repeat([]byte{1}, MaxEncryptedDataKeySize+1) }, want: ErrInvalidDataKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
			tt.configure(fake)
			provider := mustProvider(t, fake, testKeyID, nil)
			_, err := provider.Encrypt(context.Background(), []byte("payload"), nil)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Encrypt() error = %v, want %v", err, tt.want)
			}
			if generate, _ := fake.counts(); generate != 1 {
				t.Fatalf("GenerateDataKey calls = %d, want 1", generate)
			}
			if tt.name != "nil output" {
				assertAllZero(t, fake.returnedGeneratePlaintext())
				assertAllZero(t, fake.returnedGenerateBlob())
			}
		})
	}

	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	fake.decryptErr = fmt.Errorf("secret InvalidCiphertext request")
	provider := mustProvider(t, fake, testKeyID, nil)
	envelope, err := provider.Encrypt(context.Background(), []byte("payload"), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Decrypt(context.Background(), envelope, nil)
	if !errors.Is(err, ErrKMSOperation) {
		t.Fatalf("Decrypt() error = %v, want ErrKMSOperation", err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(fmt.Sprintf("%+v", err), "InvalidCiphertext") {
		t.Fatalf("error leaked SDK detail: %v", err)
	}
	assertAllZero(t, fake.returnedDecryptPlaintext())
}

func TestProviderMetadataMismatchAndMalformedEnvelopeAvoidKMS(t *testing.T) {
	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, map[string]string{"tenant": "blue"})
	other := mustProvider(t, fake, "alias/other", map[string]string{"tenant": "blue"})
	envelope, err := provider.Encrypt(context.Background(), []byte("payload"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Decrypt(context.Background(), envelope, nil); !errors.Is(err, ErrMetadataMismatch) {
		t.Fatalf("key mismatch error = %v", err)
	}
	if _, err := provider.Decrypt(context.Background(), []byte("BTKMS{}"), nil); !errors.Is(err, ErrMalformedEnvelope) {
		t.Fatalf("malformed error = %v", err)
	}
	if _, err := provider.Decrypt(context.Background(), envelope, nil); err != nil {
		t.Fatal(err)
	}
	if generate, decrypt := fake.counts(); generate != 1 || decrypt != 1 {
		t.Fatalf("KMS calls = %d/%d, want 1/1", generate, decrypt)
	}

	emptyValueFake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	emptyProvider := mustProvider(t, emptyValueFake, testKeyID, map[string]string{"a": ""})
	emptyOther := mustProvider(t, emptyValueFake, testKeyID, map[string]string{"b": ""})
	emptyEnvelope, err := emptyProvider.Encrypt(context.Background(), []byte("payload"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := emptyOther.Decrypt(context.Background(), emptyEnvelope, nil); !errors.Is(err, ErrMetadataMismatch) {
		t.Fatalf("empty-value context mismatch error = %v", err)
	}
	if _, decrypt := emptyValueFake.counts(); decrypt != 0 {
		t.Fatalf("empty-value mismatch Decrypt calls = %d, want 0", decrypt)
	}
}

func TestProviderDecryptAuthenticationAndOutputValidation(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	fake := newFakeClient(key, []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, map[string]string{"tenant": "blue"})
	envelopeBytes, err := provider.Encrypt(context.Background(), []byte("payload"), []byte("record-v1"))
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := ParseEnvelope(envelopeBytes)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*Envelope)
		ad     []byte
		want   error
	}{
		{name: "ciphertext", mutate: func(value *Envelope) { value.Ciphertext[len(value.Ciphertext)-1] ^= 1 }, ad: []byte("record-v1"), want: encrypt.ErrAuthenticationFailed},
		{name: "nonce", mutate: func(value *Envelope) { value.Nonce[0] ^= 1 }, ad: []byte("record-v1"), want: encrypt.ErrAuthenticationFailed},
		{name: "encrypted data key", mutate: func(value *Envelope) { value.EncryptedDataKey[0] ^= 1 }, ad: []byte("record-v1"), want: encrypt.ErrAuthenticationFailed},
		{name: "associated data", mutate: func(_ *Envelope) {}, ad: []byte("other"), want: encrypt.ErrAuthenticationFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := envelope
			candidate.EncryptedDataKey = append([]byte(nil), envelope.EncryptedDataKey...)
			candidate.Nonce = append([]byte(nil), envelope.Nonce...)
			candidate.Ciphertext = append([]byte(nil), envelope.Ciphertext...)
			test.mutate(&candidate)
			wire, err := candidate.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Decrypt(context.Background(), wire, test.ad); !errors.Is(err, test.want) {
				t.Fatalf("Decrypt() error = %v, want %v", err, test.want)
			}
		})
	}

	for _, test := range []struct {
		name      string
		plaintext []byte
		want      error
	}{
		{name: "nil plaintext", plaintext: nil, want: ErrInvalidDataKey},
		{name: "short plaintext", plaintext: bytes.Repeat([]byte{1}, 31), want: ErrInvalidDataKey},
		{name: "long plaintext", plaintext: bytes.Repeat([]byte{1}, 33), want: ErrInvalidDataKey},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake.mu.Lock()
			fake.decryptPlaintext = append([]byte(nil), test.plaintext...)
			fake.mu.Unlock()
			if _, err := provider.Decrypt(context.Background(), envelopeBytes, []byte("record-v1")); !errors.Is(err, test.want) {
				t.Fatalf("Decrypt() error = %v, want %v", err, test.want)
			}
			assertAllZero(t, fake.returnedDecryptPlaintext())
		})
	}
}

func TestProviderCancellationCheckpoints(t *testing.T) {
	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Encrypt(ctx, []byte("payload"), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancel Encrypt() error = %v", err)
	}
	if generate, _ := fake.counts(); generate != 0 {
		t.Fatalf("pre-cancel GenerateDataKey calls = %d, want 0", generate)
	}

	block := make(chan struct{})
	fake.generateBlock = block
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(block) }) }
	t.Cleanup(release)
	done := make(chan error, 1)
	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		_, err := provider.Encrypt(ctx, []byte("payload"), nil)
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-KMS Encrypt() error = %v", err)
		}
		release()
	case <-time.After(2 * time.Second):
		release()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("blocked fake goroutine did not terminate after release")
		}
		t.Fatal("blocked fake did not observe cancellation")
	}
}

func TestParseEnvelopeRejectsNonCanonicalInput(t *testing.T) {
	valid := mustEnvelopeBytes(t, testEnvelope())
	dataKeyEncoded := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	nonCanonicalDataKey := strings.TrimRight(dataKeyEncoded, "=")
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   error
	}{
		{name: "unknown field", mutate: func(data []byte) []byte {
			return append(append([]byte(nil), data[:len(data)-1]...), []byte(`,"extra":1}`)...)
		}, want: ErrMalformedEnvelope},
		{name: "duplicate field", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
		}, want: ErrMalformedEnvelope},
		{name: "case variant field", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"version":1`), []byte(`"Version":1`), 1)
		}, want: ErrMalformedEnvelope},
		{name: "trailing bytes", mutate: func(data []byte) []byte {
			return append(append([]byte(nil), data...), 'x')
		}, want: ErrMalformedEnvelope},
		{name: "top-level whitespace", mutate: func(data []byte) []byte {
			return append(append(append([]byte(nil), data[:len(envelopeMagic)]...), ' '), data[len(envelopeMagic):]...)
		}, want: ErrMalformedEnvelope},
		{name: "null required value", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"key_id":"`+testKeyID+`"`), []byte(`"key_id":null`), 1)
		}, want: ErrMalformedEnvelope},
		{name: "invalid UTF-8", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(testKeyID), append([]byte(testKeyID), 0xff), 1)
		}, want: ErrMalformedEnvelope},
		{name: "noncanonical base64", mutate: func(data []byte) []byte {
			return bytes.Replace(data, []byte(dataKeyEncoded), []byte(nonCanonicalDataKey), 1)
		}, want: ErrMalformedEnvelope},
		{name: "field reorder", mutate: func(data []byte) []byte {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data[len(envelopeMagic):], &fields); err != nil {
				panic(err)
			}
			reordered, err := json.Marshal(fields)
			if err != nil {
				panic(err)
			}
			return append([]byte(envelopeMagic), reordered...)
		}, want: ErrMalformedEnvelope},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseEnvelope(test.mutate(valid)); !errors.Is(err, test.want) {
				t.Fatalf("ParseEnvelope() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestEnvelopeValidationAndCanonicalMetadata(t *testing.T) {
	envelope := testEnvelope()
	first, err := envelope.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	second, err := (Envelope{
		Version:           envelope.Version,
		Algorithm:         envelope.Algorithm,
		KeyID:             envelope.KeyID,
		EncryptedDataKey:  append([]byte(nil), envelope.EncryptedDataKey...),
		EncryptionContext: map[string]string{"Tenant": "case-sensitive", "tenant": "blue"},
		Nonce:             append([]byte(nil), envelope.Nonce...),
		Ciphertext:        append([]byte(nil), envelope.Ciphertext...),
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("different context maps unexpectedly produced equal bytes")
	}
	metadata, err := canonicalMetadata(envelope)
	if err != nil {
		t.Fatal(err)
	}
	wantMetadata := `{"version":1,"algorithm":"AES-256-GCM","key_id":"` + testKeyID + `","encrypted_data_key":"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=","encryption_context":[{"key":"tenant","value":"blue"}]}`
	if string(metadata) != wantMetadata {
		t.Fatalf("metadata = %s, want %s", metadata, wantMetadata)
	}
	aad, err := buildAssociatedData(metadata, []byte("record-v1"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(aad, []byte("BTKMS-AAD\x01")) || !bytes.HasSuffix(aad, []byte("record-v1")) {
		t.Fatalf("unexpected AAD framing: %x", aad[:minInt(len(aad), 32)])
	}

	for name, candidate := range map[string]Envelope{
		"unsupported version":   func() Envelope { value := envelope; value.Version = 2; return value }(),
		"unsupported algorithm": func() Envelope { value := envelope; value.Algorithm = "OTHER"; return value }(),
		"invalid key":           func() Envelope { value := envelope; value.KeyID = " "; return value }(),
		"invalid nonce":         func() Envelope { value := envelope; value.Nonce = []byte{1}; return value }(),
		"short ciphertext":      func() Envelope { value := envelope; value.Ciphertext = []byte{1}; return value }(),
		"empty blob":            func() Envelope { value := envelope; value.EncryptedDataKey = nil; return value }(),
		"oversized blob": func() Envelope {
			value := envelope
			value.EncryptedDataKey = bytes.Repeat([]byte{1}, MaxEncryptedDataKeySize+1)
			return value
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := candidate.MarshalBinary(); err == nil {
				t.Fatal("MarshalBinary() unexpectedly succeeded")
			}
		})
	}
}

func TestParseEnvelopeRejectsOversizedAndUnsortedContext(t *testing.T) {
	overSized := wireEnvelope{
		Version:           EnvelopeVersion,
		Algorithm:         AlgorithmAES256GCM,
		KeyID:             testKeyID,
		EncryptedDataKey:  base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, MaxEncryptedDataKeySize+1)),
		EncryptionContext: []wireContextEntry{{Key: "tenant", Value: "blue"}},
		Nonce:             base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 12)),
		Ciphertext:        base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 16)),
	}
	encoded, err := json.Marshal(overSized)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvelope(append([]byte(envelopeMagic), encoded...)); !errors.Is(err, ErrMalformedEnvelope) {
		t.Fatalf("oversized data key error = %v", err)
	}

	unsorted := testEnvelope()
	unsorted.EncryptionContext = map[string]string{"z": "last", "a": "first"}
	wire := toWireEnvelope(unsorted)
	wire.EncryptionContext[0], wire.EncryptionContext[1] = wire.EncryptionContext[1], wire.EncryptionContext[0]
	encoded, err = json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvelope(append([]byte(envelopeMagic), encoded...)); !errors.Is(err, ErrMalformedEnvelope) {
		t.Fatalf("unsorted context error = %v", err)
	}
	if _, err := ParseEnvelope(bytes.Repeat([]byte{'x'}, MaxEnvelopeSize+1)); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("oversized envelope error = %v", err)
	}

	oversizedCiphertext := testEnvelope()
	oversizedCiphertext.Ciphertext = bytes.Repeat([]byte{'c'}, MaxPlaintextSize+gcmOverhead+1)
	encoded, err = json.Marshal(toWireEnvelope(oversizedCiphertext))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvelope(append([]byte(envelopeMagic), encoded...)); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("oversized ciphertext error = %v", err)
	}

	tooManyContext := testEnvelope()
	tooManyContext.EncryptionContext = make(map[string]string, MaxContextEntries+1)
	for index := 0; index < MaxContextEntries+1; index++ {
		tooManyContext.EncryptionContext[fmt.Sprintf("key-%02d", index)] = "value"
	}
	encoded, err = json.Marshal(toWireEnvelope(tooManyContext))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvelope(append([]byte(envelopeMagic), encoded...)); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("too many context entries error = %v", err)
	}
	tooLargeContext := testEnvelope()
	tooLargeContext.EncryptionContext = map[string]string{"key": strings.Repeat("v", MaxContextSize)}
	encoded, err = json.Marshal(toWireEnvelope(tooLargeContext))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvelope(append([]byte(envelopeMagic), encoded...)); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("oversized context error = %v", err)
	}

	for name, mutation := range map[string]func([]byte) []byte{
		"unsupported version": func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"version":1`), []byte(`"version":2`), 1)
		},
		"unsupported algorithm": func(data []byte) []byte {
			return bytes.Replace(data, []byte(`"algorithm":"AES-256-GCM"`), []byte(`"algorithm":"OTHER"`), 1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			data := mutation(mustEnvelopeBytes(t, testEnvelope()))
			want := ErrUnsupportedVersion
			if name == "unsupported algorithm" {
				want = ErrUnsupportedAlgorithm
			}
			if _, err := ParseEnvelope(data); !errors.Is(err, want) {
				t.Fatalf("ParseEnvelope() error = %v, want %v", err, want)
			}
		})
	}
}

func TestParseEnvelopeBoundsJSONStringsBeforeDecoding(t *testing.T) {
	valid := mustEnvelopeBytes(t, testEnvelope())
	oversizedKey := strings.Repeat("k", maxJSONStringBytes(MaxKeyIDSize)+1)
	keyMutation := bytes.Replace(
		valid,
		[]byte(`"key_id":"`+testKeyID+`"`),
		[]byte(`"key_id":"`+oversizedKey+`"`),
		1,
	)
	if _, err := ParseEnvelope(keyMutation); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("oversized key ID error = %v, want ErrInputTooLarge", err)
	}

	oversizedContextValue := strings.Repeat("v", maxJSONStringBytes(MaxContextSize)+1)
	contextMutation := bytes.Replace(
		valid,
		[]byte(`"value":"blue"`),
		[]byte(`"value":"`+oversizedContextValue+`"`),
		1,
	)
	if _, err := ParseEnvelope(contextMutation); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("oversized context value error = %v, want ErrInputTooLarge", err)
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func TestProviderCancellationUsesDeterministicCheckpoints(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	newProvider := func() (*Provider, *fakeClient) {
		fake := newFakeClient(key, []byte("blob"))
		return mustProvider(t, fake, testKeyID, nil), fake
	}

	t.Run("encrypt before local crypto", func(t *testing.T) {
		provider, fake := newProvider()
		ctx := &checkpointContext{Context: context.Background(), cancelAt: 2, err: context.Canceled}
		if result, err := provider.Encrypt(ctx, []byte("payload"), nil); !errors.Is(err, context.Canceled) || result != nil {
			t.Fatalf("Encrypt() = %x, %v", result, err)
		}
		if generate, _ := fake.counts(); generate != 1 {
			t.Fatalf("GenerateDataKey calls = %d, want 1", generate)
		}
		assertAllZero(t, fake.returnedGeneratePlaintext())
	})

	t.Run("encrypt before publish", func(t *testing.T) {
		provider, fake := newProvider()
		ctx := &checkpointContext{Context: context.Background(), cancelAt: 3, err: context.DeadlineExceeded}
		if result, err := provider.Encrypt(ctx, []byte("payload"), nil); !errors.Is(err, context.DeadlineExceeded) || result != nil {
			t.Fatalf("Encrypt() = %x, %v", result, err)
		}
		assertAllZero(t, fake.returnedGeneratePlaintext())
	})

	fixtureProvider, fixtureFake := newProvider()
	envelope, err := fixtureProvider.Encrypt(context.Background(), []byte("payload"), nil)
	if err != nil {
		t.Fatal(err)
	}
	fixtureFake.resetCounts()

	t.Run("decrypt before KMS", func(t *testing.T) {
		ctx := &checkpointContext{Context: context.Background(), cancelAt: 2, err: context.Canceled}
		if result, err := fixtureProvider.Decrypt(ctx, envelope, nil); !errors.Is(err, context.Canceled) || result != nil {
			t.Fatalf("Decrypt() = %x, %v", result, err)
		}
		if _, decrypt := fixtureFake.counts(); decrypt != 0 {
			t.Fatalf("Decrypt calls = %d, want 0", decrypt)
		}
	})

	t.Run("decrypt after KMS", func(t *testing.T) {
		fixtureFake.resetCounts()
		ctx := &checkpointContext{Context: context.Background(), cancelAt: 3, err: context.DeadlineExceeded}
		if result, err := fixtureProvider.Decrypt(ctx, envelope, nil); !errors.Is(err, context.DeadlineExceeded) || result != nil {
			t.Fatalf("Decrypt() = %x, %v", result, err)
		}
		if _, decrypt := fixtureFake.counts(); decrypt != 1 {
			t.Fatalf("Decrypt calls = %d, want 1", decrypt)
		}
		assertAllZero(t, fixtureFake.returnedDecryptPlaintext())
	})

	t.Run("decrypt before publish", func(t *testing.T) {
		fixtureFake.resetCounts()
		ctx := &checkpointContext{Context: context.Background(), cancelAt: 4, err: context.Canceled}
		if result, err := fixtureProvider.Decrypt(ctx, envelope, nil); !errors.Is(err, context.Canceled) || result != nil {
			t.Fatalf("Decrypt() = %x, %v", result, err)
		}
		assertAllZero(t, fixtureFake.returnedDecryptPlaintext())
	})
}

func TestProviderConcurrentRoundTrip(t *testing.T) {
	fake := newFakeClient(bytes.Repeat([]byte{7}, 32), []byte("blob"))
	provider := mustProvider(t, fake, testKeyID, map[string]string{"tenant": "blue"})
	const workers = 8
	const iterations = 20
	errorsCh := make(chan error, workers)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wait sync.WaitGroup
	wait.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				wire, err := provider.Encrypt(ctx, []byte("payload"), []byte("record-v1"))
				if err != nil {
					errorsCh <- err
					return
				}
				plaintext, err := provider.Decrypt(ctx, wire, []byte("record-v1"))
				if err != nil {
					errorsCh <- err
					return
				}
				if string(plaintext) != "payload" {
					errorsCh <- fmt.Errorf("plaintext = %q", plaintext)
					return
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(12 * time.Second):
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent round-trip workers did not terminate")
		}
		t.Fatal("concurrent round-trip exceeded bounded timeout")
	}
	close(errorsCh)
	for err := range errorsCh {
		t.Fatal(err)
	}
	if generate, decrypt := fake.counts(); generate != workers*iterations || decrypt != workers*iterations {
		t.Fatalf("KMS calls = %d/%d, want %d/%d", generate, decrypt, workers*iterations, workers*iterations)
	}
}
