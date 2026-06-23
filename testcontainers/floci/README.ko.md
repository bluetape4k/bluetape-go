# testcontainers/floci

[English](README.md) | [한국어](README.ko.md)

`testcontainers/floci`는 로컬 AWS 통합 테스트를 위해 Floci 컨테이너를
시작하고 AWS SDK for Go v2 client에 필요한 endpoint, region, 테스트
credential, account, availability-zone 정보를 반환합니다.

## Import

```go
import flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
```

## Usage

```go
ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
t.Cleanup(cancel)

details := flocitestcontainer.Start(ctx, t)
cfg := flocitestcontainer.LoadConfig(ctx, t, details)

client := s3.NewFromConfig(cfg, func(options *s3.Options) {
    options.UsePathStyle = true
})
```

로컬 endpoint를 사용하는 S3 client에는 `UsePathStyle`이 필요합니다.
서비스별 AWS client 동작은 해당 서비스 테스트나 package에 남기고, 이
helper는 Floci 시작과 공통 AWS config 생성만 담당합니다.

## Connection Details

`Details.ConnectionDetails()`는 `testcontainers/server` env export helper와
함께 쓸 수 있는 공유 map을 반환합니다.

```go
details := flocitestcontainer.Start(ctx, t)
if err := tcserver.ExportEnv(t, details.ConnectionDetails(), map[string]string{
    flocitestcontainer.EndpointKey:        "BLUETAPE_FLOCI_ENDPOINT",
    flocitestcontainer.RegionKey:          "BLUETAPE_FLOCI_REGION",
    flocitestcontainer.AccessKeyIDKey:     "BLUETAPE_FLOCI_ACCESS_KEY_ID",
    flocitestcontainer.SecretAccessKeyKey: "BLUETAPE_FLOCI_SECRET_ACCESS_KEY",
}); err != nil {
    t.Fatal(err)
}
```

`tcserver.ExportEnv`는 `testing.TB.Setenv`를 사용합니다. `t.Parallel`을
사용하거나 parallel ancestor가 있는 테스트에서는 호출하지 마세요.

## Behavior

- upstream `github.com/floci-io/testcontainers-floci-go`를 사용합니다.
- upstream 기본 이미지 `floci/floci:latest`를 사용합니다.
- Floci readiness가 확인된 뒤 반환합니다.
- `t.Cleanup`에 bounded `Stop` cleanup을 등록합니다.
- 다음 key를 노출합니다.
  - `floci.endpoint`
  - `floci.region`
  - `floci.access_key_id`
  - `floci.secret_access_key`
  - `floci.account_id`
  - `floci.availability_zone`
  - `floci.dedicated_network_name`
- access key와 secret key는 Floci 테스트 credential 전용입니다.

## Scope

이 작업은 #220의 첫 slice이며 #61의 기반 fixture입니다. 기본
`go test ./...` 안정성을 위해 S3 smoke test는 opt-in으로 두고, local 및 CI
Docker lane에서 명시적으로 emulator contract를 검증할 수 있게 합니다.

- S3 예제 확장은 #62에 남깁니다.
- SQS/SNS producer-consumer 예제는 #63에 남깁니다.
- DynamoDB repository 및 conditional-write 판단은 #64에 남깁니다.
- Graph database fixture는 #50/#44 이후에 진행합니다.
- Infrastructure/security/observability fixture는 구체적인 consumer issue가
  생긴 뒤 구현합니다.

## Operational Boundaries

- Docker 또는 Testcontainers 호환 runtime이 필요합니다.
- 첫 실행 시 `floci/floci:latest` 이미지를 pull할 수 있습니다.
- 기본은 dynamic host port mapping입니다. 컨테이너 시작 후
  `Details.Endpoint`를 읽으세요.
- Docker-backed Testcontainers package는 resource나 port가 공유될 때
  serial로 실행하세요.
- Docker가 없는 CI job은 `./testcontainers/...`를 skip하고, 이 helper를
  검증하는 CI job은 `-p 1`로 package를 실행해야 합니다.
- upstream Floci module은 기본적으로 넓은 service를 활성화합니다. 이
  package는 serial로 실행하고, Floci service selection을 바꾸면 인접 Docker
  fixture를 다시 검증하세요.

## Test

```bash
go test -p 1 -count=1 ./testcontainers/floci
BLUETAPE_FLOCI_SMOKE=1 go test -p 1 -count=1 ./testcontainers/floci
```
