package flocitestcontainer

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/bluetape4k/bluetape-go/internal/testcleanup"
	tcserver "github.com/bluetape4k/bluetape-go/testcontainers/server"
	upstreamfloci "github.com/floci-io/testcontainers-floci-go"
)

const (
	defaultImage = "floci/floci:latest"

	// EndpointKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	EndpointKey = "floci.endpoint"
	// RegionKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	RegionKey = "floci.region"
	// AccessKeyIDKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	AccessKeyIDKey = "floci.access_key_id"
	// SecretAccessKeyKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	SecretAccessKeyKey = "floci.secret_access_key"
	// AccountIDKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	AccountIDKey = "floci.account_id"
	// AvailabilityZoneKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	AvailabilityZoneKey = "floci.availability_zone"
	// DedicatedNetworkNameKey Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
	DedicatedNetworkNameKey = "floci.dedicated_network_name"
)

// ContainerOption Floci test container 생성 옵션을 조정한다.
type ContainerOption func(*upstreamfloci.FlociContainer)

// S3Config Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
type S3Config = upstreamfloci.S3Config

// SQSConfig Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
type SQSConfig = upstreamfloci.SqsConfig

// SNSConfig Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
type SNSConfig = upstreamfloci.SnsConfig

// DynamoDBConfig Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
type DynamoDBConfig = upstreamfloci.DynamoDbConfig

// Details Testcontainers fixture에서 caller-visible 상태와 의미를 설명한다.
type Details struct {
	Endpoint             string
	Region               string
	AccessKeyID          string
	SecretAccessKey      string
	AccountID            string
	AvailabilityZone     string
	DedicatedNetworkName string
}

// DefaultS3Config Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func DefaultS3Config() S3Config {
	return upstreamfloci.DefaultS3Config()
}

// DefaultSQSConfig Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func DefaultSQSConfig() SQSConfig {
	return upstreamfloci.DefaultSqsConfig()
}

// DefaultSNSConfig Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func DefaultSNSConfig() SNSConfig {
	return upstreamfloci.DefaultSnsConfig()
}

// DefaultDynamoDBConfig Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func DefaultDynamoDBConfig() DynamoDBConfig {
	return upstreamfloci.DefaultDynamoDbConfig()
}

// WithS3Config Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
func WithS3Config(cfg S3Config) ContainerOption {
	return func(container *upstreamfloci.FlociContainer) {
		container.WithS3Config(cfg)
	}
}

// WithSQSConfig Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
func WithSQSConfig(cfg SQSConfig) ContainerOption {
	return func(container *upstreamfloci.FlociContainer) {
		container.WithSqsConfig(cfg)
	}
}

// WithSNSConfig Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
func WithSNSConfig(cfg SNSConfig) ContainerOption {
	return func(container *upstreamfloci.FlociContainer) {
		container.WithSnsConfig(cfg)
	}
}

// WithDynamoDBConfig Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
func WithDynamoDBConfig(cfg DynamoDBConfig) ContainerOption {
	return func(container *upstreamfloci.FlociContainer) {
		container.WithDynamoDbConfig(cfg)
	}
}

// Start Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func Start(ctx context.Context, tb testing.TB, opts ...ContainerOption) Details {
	tb.Helper()

	return DetailsFromContainer(tb, StartContainer(ctx, tb, opts...))
}

// StartContainer Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func StartContainer(ctx context.Context, tb testing.TB, opts ...ContainerOption) *upstreamfloci.StartedFlociContainer {
	tb.Helper()

	builder := upstreamfloci.NewFlociContainer()
	for _, opt := range opts {
		if opt != nil {
			opt(builder)
		}
	}

	container, err := builder.Start(ctx)
	if err != nil {
		tb.Fatal(testcleanup.FormatStartError("floci", defaultImage, err))
	}
	tb.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), testcleanup.DefaultTerminateTimeout)
		defer cancel()
		if err := container.Stop(cleanupCtx); err != nil {
			tb.Fatalf("terminate floci: %v", err)
		}
	})
	return container
}

// DetailsFromContainer Testcontainers fixture에서 동작과 caller-visible 계약을 설명한다.
func DetailsFromContainer(tb testing.TB, container *upstreamfloci.StartedFlociContainer) Details {
	tb.Helper()
	if container == nil {
		tb.Fatal("floci container must not be nil")
	}
	return Details{
		Endpoint:             container.GetEndpoint(),
		Region:               container.GetRegion(),
		AccessKeyID:          container.GetAccessKey(),
		SecretAccessKey:      container.GetSecretKey(),
		AccountID:            container.GetAccountID(),
		AvailabilityZone:     container.GetAvailabilityZone(),
		DedicatedNetworkName: container.GetDedicatedNetworkName(),
	}
}

// ConnectionDetails Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func (d Details) ConnectionDetails() tcserver.ConnectionDetails {
	return tcserver.ConnectionDetails{
		EndpointKey:             d.Endpoint,
		RegionKey:               d.Region,
		AccessKeyIDKey:          d.AccessKeyID,
		SecretAccessKeyKey:      d.SecretAccessKey,
		AccountIDKey:            d.AccountID,
		AvailabilityZoneKey:     d.AvailabilityZone,
		DedicatedNetworkNameKey: d.DedicatedNetworkName,
	}
}

// LoadConfig Testcontainers fixture에서 반환값과 오류 의미를 설명한다.
func LoadConfig(ctx context.Context, tb testing.TB, details Details, opts ...func(*config.LoadOptions) error) aws.Config {
	tb.Helper()
	requireDetail(tb, EndpointKey, details.Endpoint)
	requireDetail(tb, RegionKey, details.Region)
	requireDetail(tb, AccessKeyIDKey, details.AccessKeyID)
	requireDetail(tb, SecretAccessKeyKey, details.SecretAccessKey)

	loadOptions := []func(*config.LoadOptions) error{
		config.WithRegion(details.Region),
		config.WithBaseEndpoint(details.Endpoint),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(details.AccessKeyID, details.SecretAccessKey, "")),
	}
	loadOptions = append(loadOptions, opts...)

	cfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		tb.Fatalf("floci aws config: %v", err)
	}
	return cfg
}

func requireDetail(tb testing.TB, key, value string) {
	tb.Helper()
	if value == "" {
		tb.Fatalf("%s must not be empty", key)
	}
}
