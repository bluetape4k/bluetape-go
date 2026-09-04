Resolves #63.

## 요약

- 직접 AWS SDK for Go v2 SQS/SNS 사용을 보여 주는 compile-checked example인
  `examples/sqs-sns`를 추가했다.
- 다루는 기능: SQS send, long-poll receive, manual ack/delete, visibility extension, redrive
  policy JSON, SNS to SQS fanout을 다룬다.
- retry, DLQ, visibility timeout, production SNS-to-SQS queue policy의
  주의사항을 문서화했다.
- root README package index를 English 및 Korean으로 갱신했다.

## 검토

- Step 2-R, Step 3-R, Step 6-R 7-tier review 산출물이
  `docs/superpowers/reviews/` 아래에 포함되어 있다.
- Step 6-R 검토 결과: P0=0, P1=0.
- Go stress requirement: example-only package이며 shared mutable state, worker
  lifecycle, goroutine-safe public contract를 추가하지 않으므로 해당 없음.
  대상 race, smoke race, 직렬 전체 race가 통과했다.

## 검증

- PASS `go test -count=1 ./examples/sqs-sns`
- PASS `go test -race -count=1 ./examples/sqs-sns`
- PASS `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -p 1 -count=1 ./examples/sqs-sns`
- PASS `BLUETAPE_SQS_SNS_EXAMPLE_SMOKE=1 go test -race -p 1 -count=1 ./examples/sqs-sns`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `go test -p 1 -count=1 ./...`
- PASS `go test -race -p 1 -count=1 ./...`
- PASS `git diff --check`

## DoD Status

- [x] 직접 AWS SDK example으로 이슈 #63 범위를 구현했다.
- [x] public package 동작에 대해 README와 README.ko.md를 동기화했다.
- [x] Docker 기반 Floci smoke test는 opt-in이며 문서화되어 있다.
- [x] 필요한 경우 main integration fallback을 사용하여 7-tier review를
      완료했다.
- [ ] GitHub CI 대기.
