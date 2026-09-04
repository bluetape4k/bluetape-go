Resolves #62.

Stacked on #267 / `issue-60-aws-helper-boundaries`.

## 요약

- 직접 AWS SDK for Go v2 S3 사용을 보여 주는 compile-checked example인
  `examples/s3`를 추가했다.
- 다루는 기능: put/get, metadata, content type detection, streaming upload/download,
  presigned GET/PUT URL, not-found error mapping을 다룬다.
- 구체적인 Go consumer가 필요로 할 때까지 Floci path-style endpoint 설정과
  KMS/client-side encryption을 범위에서 제외한다고 문서화했다.
- root README package index를 English 및 Korean으로 갱신했다.

## 검토

- Step 2-R, Step 3-R, Step 6-R 7-tier review 산출물이
  `docs/superpowers/reviews/` 아래에 포함되어 있다.
- Step 6-R 검토 결과: P0=0, P1=0.
- Go stress requirement: example-only package이며 shared mutable state나 public
  concurrency primitive를 추가하지 않으므로 해당 없음. smoke 및 race
  게이트가 Docker 기반 example 경로를 다룬다.

## 검증

- PASS `go test -count=1 ./examples/s3`
- PASS `go test -race -count=1 ./examples/s3`
- PASS `BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/s3`
- PASS `BLUETAPE_S3_EXAMPLE_SMOKE=1 go test -race -p 1 -count=1 ./examples/s3`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `make test`
- PASS `make race`
- PASS `git diff --check`

## DoD Status

- [x] 직접 AWS SDK example으로 이슈 #62 범위를 구현했다.
- [x] public package 동작에 대해 README와 README.ko.md를 동기화했다.
- [x] Docker 기반 Floci smoke test는 opt-in이며 문서화되어 있다.
- [x] 필요한 경우 main integration fallback을 사용하여 7-tier review를
      완료했다.
- [ ] GitHub CI 대기.
