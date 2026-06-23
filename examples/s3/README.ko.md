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
- presigned GET/PUT URL
- modeled error와 Smithy API error code 기반 S3 error mapping
- path-style S3 addressing을 사용하는 로컬 Floci endpoint 설정

KMS와 client-side encryption은 이 package 범위가 아닙니다. 실제 Go consumer가
envelope encryption, key policy, metadata compatibility를 필요로 할 때 별도
KMS/encryption issue로 다룹니다.

## Local Endpoint

Floci와 다른 로컬 S3-compatible endpoint에는 path-style addressing이 필요합니다.

```go
client := s3.NewFromConfig(cfg, func(options *s3.Options) {
    options.UsePathStyle = true
})
```

실제 AWS S3에서는 local endpoint override를 생략하고 application이 필요한 option만
설정하세요.

## Content Type

예제는 bluetape helper 대신 standard library content type 처리를 사용합니다.

1. object key에 알려진 extension이 있으면 `mime.TypeByExtension`.
2. extension이 없거나 알 수 없으면 payload sample에서 `http.DetectContentType`.

## Test

예제 compile-check:

```bash
go test -count=1 ./examples/s3
```

Docker-backed Floci smoke test:

```bash
BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/s3
```

Docker resource가 공유될 때는 Testcontainers-backed package를 serial로 실행하세요.
