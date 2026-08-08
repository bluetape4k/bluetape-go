Resolves #220.

## 요약

- 병합된 작업: Floci fixture, service smoke, S3, SQS/SNS, DynamoDB evaluation, DynamoDB
  batch helper 작업이 모두 병합된 뒤 #220 closure note를 추가했다.
- 2026-06-24 closure 결정과 PR 증거를 반영하여 #220 fixture matrix를
  갱신했다.
- 구체적인 consumer issue가 선택할 때까지 LocalStack, DynamoDB Local,
  ElasticMQ, graph DB, infrastructure, LLM/vector fixture를 보류했다.

## 검토

- Step 6-R 7-tier closure review가 `docs/superpowers/reviews/` 아래에
  포함되어 있다.
- Step 6-R 검토 결과: P0=0, P1=0.
- Go stress: docs-only closure diff이므로 해당 없음. runtime code,
  goroutine, channel, shared state를 변경하지 않았다.

## 검증

- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PENDING GitHub CI

## DoD Status

- [x] #220 완료 implementation 범위를 기록했다.
- [x] 0.9.0 AWS consumer PR 증거를 연결했다.
- [x] 무거운 fixture 후보를 구체적인 향후 consumer로 연결했다.
- [x] P0=0 P1=0으로 7-tier closure review를 완료했다.
- [x] 로컬 docs 검증을 완료했다.
- [ ] GitHub CI 대기.
