package s3example_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func Example_putGetMetadataAndPresign() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := aws.Config{} // Load with config.LoadDefaultConfig in application code.

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true // Required for Floci and most local S3 endpoints.
	})
	presigner := s3.NewPresignClient(client)

	bucket := "example-bucket"
	key := "docs/hello.txt"
	body := "hello from bluetape-go"
	contentType := contentTypeForKey(key, []byte(body))

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(body),
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"source": "bluetape-go",
		},
	}); err != nil {
		return
	}

	if _, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return
	}

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return
	}
	if out != nil && out.Body != nil {
		defer func() {
			_ = out.Body.Close()
		}()
		if _, err := io.ReadAll(out.Body); err != nil {
			return
		}
	}

	if _, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(options *s3.PresignOptions) {
		options.Expires = 15 * time.Minute
	}); err != nil {
		return
	}
}

func Example_streamingUploadDownload() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := aws.Config{}

	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true
	})

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("example-bucket"),
		Key:    aws.String("streamed.txt"),
		Body:   strings.NewReader("chunk-1\nchunk-2\n"),
	}); err != nil {
		return
	}

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String("example-bucket"),
		Key:    aws.String("streamed.txt"),
	})
	if err != nil {
		return
	}
	if out != nil && out.Body != nil {
		defer func() {
			_ = out.Body.Close()
		}()
		var downloaded bytes.Buffer
		if _, err := io.Copy(&downloaded, out.Body); err != nil {
			return
		}
	}
}

func Example_errorMapping() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := getMissingS3Object(ctx, nil, "example-bucket", "missing.txt")
	if isS3NotFound(err) {
		fmt.Println("not found")
		return
	}
	if err != nil {
		fmt.Println("other error")
	}
}

func TestS3ExampleSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_S3_EXAMPLE_SMOKE") != "1" {
		t.Skip("set BLUETAPE_S3_EXAMPLE_SMOKE=1 to run the Docker-backed Floci S3 example smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t, flocitestcontainer.WithS3Config(flocitestcontainer.DefaultS3Config()))
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true
	})
	presigner := s3.NewPresignClient(client)

	bucket := "bluetape-s3-example-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	key := "docs/hello.txt"
	body := []byte("hello from bluetape-go s3 examples")
	contentType := contentTypeForKey(key, body)
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"source": "bluetape-go",
			"issue":  "62",
		},
	}); err != nil {
		t.Fatalf("put object: %v", err)
	}

	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("head object: %v", err)
	}
	if got := aws.ToString(head.ContentType); got != "text/plain; charset=utf-8" {
		t.Fatalf("content type = %q, want text/plain; charset=utf-8", got)
	}
	if got := head.Metadata["source"]; got != "bluetape-go" {
		t.Fatalf("metadata source = %q, want bluetape-go", got)
	}

	got, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	payload, err := io.ReadAll(got.Body)
	if closeErr := got.Body.Close(); closeErr != nil {
		t.Fatalf("close object body: %v", closeErr)
	}
	if err != nil {
		t.Fatalf("read object body: %v", err)
	}
	if !bytes.Equal(payload, body) {
		t.Fatalf("object body = %q, want %q", payload, body)
	}

	streamKey := "docs/streamed.txt"
	if err := putStreamingObject(ctx, client, bucket, streamKey); err != nil {
		t.Fatalf("put streaming object: %v", err)
	}
	var streamed bytes.Buffer
	if err := downloadObject(ctx, client, bucket, streamKey, &streamed); err != nil {
		t.Fatalf("download streaming object: %v", err)
	}
	if got := streamed.String(); got != "chunk-1\nchunk-2\n" {
		t.Fatalf("streamed body = %q, want chunks", got)
	}

	getURL, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(options *s3.PresignOptions) {
		options.Expires = 15 * time.Minute
	})
	if err != nil {
		t.Fatalf("presign get object: %v", err)
	}
	assertPresignedURL(t, getURL.Method, getURL.URL, http.MethodGet)

	putURL, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String("docs/presigned-put.txt"),
		ContentType: aws.String("text/plain; charset=utf-8"),
	}, func(options *s3.PresignOptions) {
		options.Expires = 15 * time.Minute
	})
	if err != nil {
		t.Fatalf("presign put object: %v", err)
	}
	assertPresignedURL(t, putURL.Method, putURL.URL, http.MethodPut)

	if err := getMissingS3Object(ctx, client, bucket, "docs/missing.txt"); !isS3NotFound(err) {
		t.Fatalf("missing object error = %T %[1]v, want S3 not found", err)
	}
}

func TestContentTypeForKey(t *testing.T) {
	cases := []struct {
		name string
		key  string
		body []byte
		want string
	}{
		{name: "extension", key: "docs/hello.txt", body: []byte("hello"), want: "text/plain; charset=utf-8"},
		{name: "payload sample", key: "docs/data", body: []byte(`{"ok":true}`), want: "text/plain; charset=utf-8"},
		{name: "empty payload", key: "docs/data", body: nil, want: "application/octet-stream"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := contentTypeForKey(tc.key, tc.body); got != tc.want {
				t.Fatalf("contentTypeForKey(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func contentTypeForKey(key string, sample []byte) string {
	if typ := mime.TypeByExtension(filepath.Ext(key)); typ != "" {
		return typ
	}
	if len(sample) == 0 {
		return "application/octet-stream"
	}
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return http.DetectContentType(sample)
}

func putStreamingObject(ctx context.Context, client *s3.Client, bucket, key string) error {
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader("chunk-1\nchunk-2\n"),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func downloadObject(ctx context.Context, client *s3.Client, bucket, key string, dst io.Writer) error {
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}

	_, copyErr := io.Copy(dst, out.Body)
	closeErr := out.Body.Close()
	if copyErr != nil {
		return fmt.Errorf("copy object body: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close object body: %w", closeErr)
	}
	return nil
}

func getMissingS3Object(ctx context.Context, client *s3.Client, bucket, key string) error {
	if client == nil {
		return &s3types.NoSuchKey{}
	}
	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	if out.Body != nil {
		_ = out.Body.Close()
	}
	return nil
}

func isS3NotFound(err error) bool {
	if err == nil {
		return false
	}
	var noSuchKey *s3types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr interface{ ErrorCode() string }
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey"
}

func assertPresignedURL(t *testing.T, method, rawURL, wantMethod string) {
	t.Helper()

	if method != wantMethod {
		t.Fatalf("presign method = %q, want %q", method, wantMethod)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		t.Fatalf("presigned URL must include scheme and host: %q", rawURL)
	}
}
