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

	// EndpointKey is the documented key for the Floci AWS endpoint URI.
	EndpointKey = "floci.endpoint"
	// RegionKey is the documented key for the Floci AWS region.
	RegionKey = "floci.region"
	// AccessKeyIDKey is the documented key for the Floci test access key ID.
	AccessKeyIDKey = "floci.access_key_id"
	// SecretAccessKeyKey is the documented key for the Floci test secret access key.
	SecretAccessKeyKey = "floci.secret_access_key"
	// AccountIDKey is the documented key for the Floci test AWS account ID.
	AccountIDKey = "floci.account_id"
	// AvailabilityZoneKey is the documented key for the Floci availability zone.
	AvailabilityZoneKey = "floci.availability_zone"
	// DedicatedNetworkNameKey is the documented key for the optional Floci Docker network.
	DedicatedNetworkNameKey = "floci.dedicated_network_name"
)

// ContainerOption customizes the upstream Floci container builder before start.
type ContainerOption func(*upstreamfloci.FlociContainer)

// Details contains the connection values needed by AWS SDK for Go v2 clients.
type Details struct {
	Endpoint             string
	Region               string
	AccessKeyID          string
	SecretAccessKey      string
	AccountID            string
	AvailabilityZone     string
	DedicatedNetworkName string
}

// Start launches a Floci test container and returns its AWS SDK details.
func Start(ctx context.Context, tb testing.TB, opts ...ContainerOption) Details {
	tb.Helper()

	return DetailsFromContainer(tb, StartContainer(ctx, tb, opts...))
}

// StartContainer launches a Floci test container and returns the upstream container.
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

// DetailsFromContainer extracts AWS SDK details from an upstream Floci container.
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

// ConnectionDetails returns the shared connection-detail map for env export.
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

// LoadConfig returns an AWS SDK for Go v2 config for the Floci endpoint.
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
