# S3 Examples

[English](README.md) | [한국어](README.ko.md)

This package contains compile-checked S3 examples for AWS SDK for Go v2 and the
`testcontainers/floci` fixture. It intentionally does not provide a bluetape S3
client wrapper. The examples keep `*s3.Client` and `*s3.PresignClient`
caller-owned so application code can use AWS SDK request and response types
directly.

## Scope

- `PutObject` and `GetObject`
- object metadata and content type
- streaming upload and streaming download with `io.Reader` / `io.Writer`
- AWS SDK transfer-manager upload/download with caller-owned clients
- multipart upload completion and cancellation cleanup
- presigned GET and PUT URLs
- request checksum and SSE-KMS options
- S3 error mapping with modeled errors and Smithy API error codes
- local Floci endpoint configuration with path-style S3 addressing

Client-side encryption and a bluetape KMS/envelope provider are out of scope for
this package. The examples only show the AWS SDK request fields for SSE-KMS;
key policy, KMS permissions, and production encryption decisions remain
caller-owned.

## Diagram

![S3 example contract map](../../docs/images/readme-diagrams/examples-s3-contract-map.png)

The contract map shows that the examples keep AWS SDK clients caller-owned and
only document helper boundaries for content type detection, streaming, Floci
local endpoints, and error mapping.

![S3 object operation sequence](../../docs/images/readme-diagrams/examples-s3-object-sequence.png)

The sequence follows the smoke/example flow from local endpoint configuration
through upload, metadata verification, download, presigned URLs, and modeled
not-found handling.

## Local Endpoint

Floci and other local S3-compatible endpoints need path-style addressing:

```go
client := s3.NewFromConfig(cfg, func(options *s3.Options) {
    options.UsePathStyle = true
})
```

For real AWS S3, omit the local endpoint override and only set options required
by the application.

## Transfer Manager and Multipart Cleanup

The AWS SDK transfer manager is a caller-owned utility. `Example_transferManagerUploadDownload`
uses `transfermanager.New(client, ...)` for multipart-aware upload and parallel
download without adding a bluetape S3 client wrapper. Keep the request context
bounded and use `transfermanagertypes.NewWriteAtBuffer` (or another caller-owned
`io.WriterAt`) for `DownloadObject`.

For a direct multipart flow, retain the upload ID until `CompleteMultipartUpload`
succeeds. On a part or completion error, call `AbortMultipartUpload` with a
short, bounded cleanup context derived with `context.WithoutCancel`; otherwise a
canceled request can leave uploaded parts behind. Treat cleanup errors as a
separate operational failure and record the bucket, key, and upload ID for
operator remediation. The compile-checked `Example_multipartCleanup` shows this
contract without contacting AWS during the default test run.

## Checksums and SSE-KMS Request Options

Set `PutObjectInput.ChecksumAlgorithm` (or the corresponding transfer-manager
field) when the request needs an SDK-computed checksum. To request checksum
validation on a read, use `GetObjectInput.ChecksumMode =
s3types.ChecksumModeEnabled` and inspect the returned checksum fields. The
server still validates the request; a compile-checked example is not evidence of
live bucket policy or checksum behavior.

For SSE-KMS, set `ServerSideEncryption` to `s3types.ServerSideEncryptionAwsKms`,
provide the KMS key ID/ARN with `SSEKMSKeyId`, and optionally set
`BucketKeyEnabled` and a base64-encoded `SSEKMSEncryptionContext`. The caller's
AWS identity needs the matching S3 and KMS permissions, and the KMS key policy
must allow the operation. Floci/default CI does not provide live KMS semantics;
verify these options against an opt-in AWS environment.

## Content Type

The examples use standard-library content type handling instead of a bluetape
helper:

1. `mime.TypeByExtension` when the object key has a known extension.
2. `http.DetectContentType` from a payload sample when the extension is absent
   or unknown.

## Test

Compile-check the examples:

```bash
go test -count=1 ./examples/s3
go test -run Example -count=1 ./examples/s3
```

Run the Docker-backed Floci smoke test:

```bash
BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/s3
```

Run Testcontainers-backed packages serially when Docker resources are shared.
