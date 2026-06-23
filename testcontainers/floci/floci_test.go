package flocitestcontainer_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestStartFlociS3Smoke(t *testing.T) {
	if os.Getenv("BLUETAPE_FLOCI_SMOKE") != "1" {
		t.Skip("set BLUETAPE_FLOCI_SMOKE=1 to run the Docker-backed Floci S3 smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t)
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true
	})

	bucket := fmt.Sprintf("bluetape-floci-%d", time.Now().UnixNano())
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	const key = "hello.txt"
	const body = "hello from bluetape floci"
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	}); err != nil {
		t.Fatalf("put object: %v", err)
	}

	got, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	t.Cleanup(func() {
		if err := got.Body.Close(); err != nil {
			t.Fatalf("close object body: %v", err)
		}
	})
	payload, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatalf("read object body: %v", err)
	}
	if string(payload) != body {
		t.Fatalf("object body = %q, want %q", payload, body)
	}
}

func TestDetailsConnectionDetails(t *testing.T) {
	details := flocitestcontainer.Details{
		Endpoint:             "http://localhost:4566",
		Region:               "ap-northeast-2",
		AccessKeyID:          "test",
		SecretAccessKey:      "test",
		AccountID:            "000000000000",
		AvailabilityZone:     "ap-northeast-2a",
		DedicatedNetworkName: "floci-network",
	}

	connectionDetails := details.ConnectionDetails()
	cases := map[string]string{
		flocitestcontainer.EndpointKey:             details.Endpoint,
		flocitestcontainer.RegionKey:               details.Region,
		flocitestcontainer.AccessKeyIDKey:          details.AccessKeyID,
		flocitestcontainer.SecretAccessKeyKey:      details.SecretAccessKey,
		flocitestcontainer.AccountIDKey:            details.AccountID,
		flocitestcontainer.AvailabilityZoneKey:     details.AvailabilityZone,
		flocitestcontainer.DedicatedNetworkNameKey: details.DedicatedNetworkName,
	}
	for key, want := range cases {
		got, err := connectionDetails.Require(key)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}
