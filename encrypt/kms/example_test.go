package kms_test

import (
	"context"
	"fmt"
	"time"

	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/bluetape4k/bluetape-go/encrypt/kms"
)

type exampleClient struct {
	key  []byte
	blob []byte
}

func (c *exampleClient) GenerateDataKey(context.Context, *awskms.GenerateDataKeyInput, ...func(*awskms.Options)) (*awskms.GenerateDataKeyOutput, error) {
	return &awskms.GenerateDataKeyOutput{
		Plaintext:      append([]byte(nil), c.key...),
		CiphertextBlob: append([]byte(nil), c.blob...),
	}, nil
}

func (c *exampleClient) Decrypt(context.Context, *awskms.DecryptInput, ...func(*awskms.Options)) (*awskms.DecryptOutput, error) {
	return &awskms.DecryptOutput{Plaintext: append([]byte(nil), c.key...)}, nil
}

func ExampleProvider() {
	client := &exampleClient{
		key:  []byte("01234567890123456789012345678901"),
		blob: []byte("fake-encrypted-data-key"),
	}
	provider, err := kms.New(client, "alias/example", kms.WithEncryptionContext(map[string]string{
		"service": "example",
	}))
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	envelope, err := provider.Encrypt(ctx, []byte("payload"), []byte("record:v1"))
	if err != nil {
		panic(err)
	}
	plaintext, err := provider.Decrypt(ctx, envelope, []byte("record:v1"))
	if err != nil {
		panic(err)
	}
	fmt.Println(string(plaintext))
	// Output: payload
}
