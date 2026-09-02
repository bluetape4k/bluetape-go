package kms

import (
	"context"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/bluetape4k/bluetape-go/encrypt"
)

// Client - Provider가 사용하는 AWS KMS SDK method의 최소 집합이다.
// client의 credentials, retry, transport, lifecycle은 호출자가 소유한다.
type Client interface {
	GenerateDataKey(context.Context, *awskms.GenerateDataKeyInput, ...func(*awskms.Options)) (*awskms.GenerateDataKeyOutput, error)
	Decrypt(context.Context, *awskms.DecryptInput, ...func(*awskms.Options)) (*awskms.DecryptOutput, error)
}

// AWS SDK의 concrete client가 의도한 interface를 계속 만족하는지 compile-time에 확인한다.
var _ Client = (*awskms.Client)(nil)

// Option - Provider 생성 설정을 적용한다.
type Option func(*providerConfig) error

type providerConfig struct {
	encryptionContext map[string]string
}

// Provider - caller-owned KMS client와 복사된 envelope metadata 설정을 조합한다.
// 생성 후 설정은 불변이며 client가 동시 호출을 지원하면 Provider도 동시 사용이 안전하다.
type Provider struct {
	client            Client
	keyID             string
	encryptionContext map[string]string
}

// WithEncryptionContext - KMS encryption context를 Provider 설정에 추가한다.
// context는 복사되며 여러 option에서 같은 key를 다시 지정하면 오류가 된다.
func WithEncryptionContext(values map[string]string) Option {
	copied := cloneContext(values)
	return func(config *providerConfig) error {
		if config == nil {
			return errorWith(ErrInvalidOptions, "apply encryption context", nil)
		}
		if err := validateContext(copied); err != nil {
			return err
		}
		if config.encryptionContext == nil {
			config.encryptionContext = make(map[string]string)
		}
		if len(config.encryptionContext)+len(copied) > MaxContextEntries {
			return errorWith(ErrInvalidOptions, "apply encryption context", nil)
		}
		for key, value := range copied {
			if _, exists := config.encryptionContext[key]; exists {
				return errorWith(ErrInvalidOptions, "apply encryption context", nil)
			}
			config.encryptionContext[key] = value
		}
		if err := validateContext(config.encryptionContext); err != nil {
			return errorWith(ErrInvalidOptions, "apply encryption context", err)
		}
		return nil
	}
}

// New - caller-owned KMS client와 key ID로 immutable Provider를 생성한다.
func New(client Client, keyID string, options ...Option) (*Provider, error) {
	if isNilClient(client) {
		return nil, ErrNilClient
	}
	if !validKeyID(keyID) {
		return nil, ErrInvalidKeyID
	}

	config := providerConfig{encryptionContext: make(map[string]string)}
	for _, option := range options {
		if option == nil {
			return nil, errorWith(ErrInvalidOptions, "apply option", nil)
		}
		if err := option(&config); err != nil {
			return nil, errorWith(ErrInvalidOptions, "apply option", err)
		}
	}

	return &Provider{
		client:            client,
		keyID:             keyID,
		encryptionContext: cloneContext(config.encryptionContext),
	}, nil
}

// Encrypt - KMS AES-256 data key로 local AES-GCM payload를 만들고 BTKMS envelope를 반환한다.
func (p *Provider) Encrypt(ctx context.Context, plaintext, associatedData []byte) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(plaintext) > MaxPlaintextSize || len(associatedData) > MaxAssociatedDataSize {
		return nil, errorWith(ErrInputTooLarge, "encrypt preflight", nil)
	}

	requestedContext := cloneContext(p.encryptionContext)
	output, callErr := p.client.GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{
		KeyId:             aws.String(p.keyID),
		KeySpec:           kmstypes.DataKeySpecAes256,
		EncryptionContext: cloneContext(requestedContext),
	})
	if output != nil {
		defer zeroBytes(output.Plaintext)
		defer zeroBytes(output.CiphertextBlob)
	}
	if callErr != nil {
		return nil, errorWith(ErrKMSOperation, "generate data key", callErr)
	}
	if output == nil || len(output.Plaintext) != 32 || len(output.CiphertextBlob) == 0 || len(output.CiphertextBlob) > MaxEncryptedDataKeySize {
		return nil, errorWith(ErrInvalidDataKey, "generate data key", nil)
	}

	encryptedDataKey := append([]byte(nil), output.CiphertextBlob...)
	localKey := append([]byte(nil), output.Plaintext...)
	defer zeroBytes(localKey)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	metadata, err := canonicalMetadata(Envelope{
		Version:           EnvelopeVersion,
		Algorithm:         AlgorithmAES256GCM,
		KeyID:             p.keyID,
		EncryptedDataKey:  encryptedDataKey,
		EncryptionContext: requestedContext,
	})
	if err != nil {
		return nil, err
	}
	aad, err := buildAssociatedData(metadata, associatedData)
	if err != nil {
		return nil, err
	}
	encryptor, err := encrypt.New(localKey)
	if err != nil {
		return nil, errorWith(ErrInvalidDataKey, "encrypt payload", err)
	}
	nonce, ciphertext, err := encryptor.EncryptDetached(plaintext, aad)
	if err != nil {
		return nil, err
	}
	envelope, err := (Envelope{
		Version:           EnvelopeVersion,
		Algorithm:         AlgorithmAES256GCM,
		KeyID:             p.keyID,
		EncryptedDataKey:  encryptedDataKey,
		EncryptionContext: requestedContext,
		Nonce:             nonce,
		Ciphertext:        ciphertext,
	}).MarshalBinary()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return envelope, nil
}

// Decrypt - BTKMS envelope의 encrypted data key를 KMS로 복호화한 뒤 local payload를 검증한다.
func (p *Provider) Decrypt(ctx context.Context, data, associatedData []byte) ([]byte, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	ctx = normalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(associatedData) > MaxAssociatedDataSize {
		return nil, errorWith(ErrInputTooLarge, "decrypt preflight", nil)
	}

	envelope, err := ParseEnvelope(data)
	if err != nil {
		return nil, err
	}
	if envelope.KeyID != p.keyID || !sameContext(envelope.EncryptionContext, p.encryptionContext) {
		return nil, errorWith(ErrMetadataMismatch, "decrypt metadata", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	output, callErr := p.client.Decrypt(ctx, &awskms.DecryptInput{
		CiphertextBlob:    append([]byte(nil), envelope.EncryptedDataKey...),
		KeyId:             aws.String(p.keyID),
		EncryptionContext: cloneContext(p.encryptionContext),
	})
	if output != nil {
		defer zeroBytes(output.Plaintext)
	}
	if callErr != nil {
		return nil, errorWith(ErrKMSOperation, "decrypt data key", callErr)
	}
	if output == nil || len(output.Plaintext) != 32 {
		return nil, errorWith(ErrInvalidDataKey, "decrypt data key", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	localKey := append([]byte(nil), output.Plaintext...)
	defer zeroBytes(localKey)
	metadata, err := canonicalMetadata(envelope)
	if err != nil {
		return nil, err
	}
	aad, err := buildAssociatedData(metadata, associatedData)
	if err != nil {
		return nil, err
	}
	encryptor, err := encrypt.New(localKey)
	if err != nil {
		return nil, errorWith(ErrInvalidDataKey, "decrypt payload", err)
	}
	plaintext, err := encryptor.DecryptDetached(envelope.Nonce, envelope.Ciphertext, aad)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		zeroBytes(plaintext)
		return nil, err
	}
	return plaintext, nil
}

func (p *Provider) validate() error {
	if p == nil || isNilClient(p.client) || !validKeyID(p.keyID) {
		return ErrInvalidProvider
	}
	if err := validateContext(p.encryptionContext); err != nil {
		return ErrInvalidProvider
	}
	return nil
}

func validKeyID(keyID string) bool {
	return utf8.ValidString(keyID) && strings.TrimSpace(keyID) != "" && len(keyID) <= MaxKeyIDSize
}

func isNilClient(client Client) bool {
	if client == nil {
		return true
	}
	value := reflect.ValueOf(client)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func sameContext(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if rightValue, ok := right[key]; !ok || rightValue != value {
			return false
		}
	}
	return true
}

func zeroBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
