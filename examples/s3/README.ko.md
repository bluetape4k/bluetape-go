# S3 Examples

[English](README.md) | [한국어](README.ko.md)

이 package는 AWS SDK for Go v2와 `testcontainers/floci` fixture를 사용하는
compile-checked S3 예제입니다. bluetape S3 client wrapper를 제공하지 않습니다.
예제는 `*s3.Client`와 `*s3.PresignClient`를 caller-owned로 유지해서 application
code가 AWS SDK request/response type을 직접 사용할 수 있게 합니다.

## 범위

- `PutObject`와 `GetObject`
- object metadata와 content type
- `io.Reader` / `io.Writer` 기반 streaming upload/download
- caller-owned client를 사용하는 AWS SDK transfer-manager upload/download
- multipart upload 완료와 cancellation cleanup
- presigned GET/PUT URL
- request checksum과 SSE-KMS option
- modeled error와 Smithy API error code 기반 S3 error mapping
- path-style S3 addressing을 사용하는 로컬 Floci endpoint 설정

Client-side encryption과 bluetape KMS/envelope provider는 이 package 범위가
아닙니다. 예제는 SSE-KMS에 필요한 AWS SDK request field만 보여주며, key policy,
KMS permission, production encryption 결정은 caller-owned로 유지합니다.

## Diagram

![S3 example contract map](../../docs/images/readme-diagrams/examples-s3-contract-map.png)

Contract map은 예제가 AWS SDK client를 caller-owned로 유지하고, content type
detection, streaming, Floci local endpoint, error mapping helper 경계만 문서화한다는
점을 보여줍니다.

![S3 object operation sequence](../../docs/images/readme-diagrams/examples-s3-object-sequence.png)

Sequence는 local endpoint 설정에서 upload, metadata 검증, download, presigned URL,
modeled not-found 처리까지 이어지는 smoke/example 흐름을 보여줍니다.

## Local Endpoint

Floci와 다른 로컬 S3-compatible endpoint에는 path-style addressing이 필요합니다.

```go
client := s3.NewFromConfig(cfg, func(options *s3.Options) {
    options.UsePathStyle = true
})
```

실제 AWS S3에서는 local endpoint override를 생략하고 application이 필요한 option만
설정하세요.

## Transfer Manager와 Multipart Cleanup

AWS SDK transfer manager는 caller-owned utility입니다. compile-checked
`Example_transferManagerUploadDownload`는 `transfermanager.New(client, ...)`를
사용해 bluetape S3 client wrapper 없이 multipart-aware upload와 병렬 download를
보여줍니다. request context에는 bounded timeout을 사용하고,
`DownloadObject`에는 `transfermanagertypes.NewWriteAtBuffer` 또는 caller-owned
`io.WriterAt`을 전달하세요.

직접 multipart flow를 구성할 때는 `CompleteMultipartUpload`가 성공할 때까지
upload ID를 보관하세요. part 또는 complete가 실패하면
`context.WithoutCancel`로 취소의 영향을 받지 않는 짧은 cleanup context를 만들고
`AbortMultipartUpload`를 호출해야 업로드된 part가 남지 않습니다. cleanup error는
별도 operational failure로 취급하고 bucket, key, upload ID를 operator remediation
정보로 남기세요. compile-checked `Example_multipartCleanup`는 기본 test 실행에서
AWS에 접속하지 않고 이 계약을 보여줍니다.

## Checksum과 SSE-KMS Request Option

request에 SDK가 checksum을 계산하도록 하려면 `PutObjectInput.ChecksumAlgorithm`
(또는 transfer-manager의 대응 field)을 지정하세요. read에서 checksum validation을
요청하려면 `GetObjectInput.ChecksumMode = s3types.ChecksumModeEnabled`를 사용하고
반환된 checksum field를 확인하세요. server validation과 실제 bucket policy 동작은
live 환경에서 별도로 검증해야 하며 compile-checked example만으로는 증명되지
않습니다.

SSE-KMS에는 `ServerSideEncryption`을
`s3types.ServerSideEncryptionAwsKms`로 지정하고 `SSEKMSKeyId`에 KMS key ID/ARN을
전달하세요. 필요하면 `BucketKeyEnabled`와 base64-encoded
`SSEKMSEncryptionContext`도 지정합니다. caller AWS identity에 필요한 S3/KMS
permission이 있어야 하고 KMS key policy도 operation을 허용해야 합니다.
Floci/default CI는 live KMS semantics를 제공하지 않으므로 opt-in AWS 환경에서
이 option을 확인하세요.

## Content Type

예제는 bluetape helper 대신 standard library content type 처리를 사용합니다.

1. object key에 알려진 extension이 있으면 `mime.TypeByExtension`.
2. extension이 없거나 알 수 없으면 payload sample에서 `http.DetectContentType`.

## Test

예제 compile-check:

```bash
go test -count=1 ./examples/s3
go test -run Example -count=1 ./examples/s3
```

Docker-backed Floci smoke test:

```bash
BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/s3
```

Docker resource가 공유될 때는 Testcontainers-backed package를 serial로 실행하세요.
