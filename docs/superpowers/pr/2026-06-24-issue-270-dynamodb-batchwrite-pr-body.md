Resolves #270.

## 요약

- `dynamodb/batchwrite`는 좁은 AWS SDK for Go v2 `BatchWriteItem` helper로
  25-item chunking과 `UnprocessedItems` retry를 지원한다.
- API 입력 및 소유권: caller-owned client, `context.Context`, `map[string][]types.WriteRequest`
  input을 사용하는 SDK-native API를 유지했다.
- chunking, retry, cancellation, error preservation, 실제 DynamoDB 호환
  실행을 검증하는 unit, race, opt-in Floci smoke coverage를 추가했다.
- English 및 Korean package docs와 root package table을 갱신했다.

## 검토

- Step 2-R, Step 3-R, Step 6-R 7-tier review 산출물이
  `docs/superpowers/reviews/` 아래에 포함되어 있다.
- Step 6-R 검토 결과: P0=0, P1=0.
- Go stress: `GoroutineStressTester`와 `AsyncJobTester`는 해당 없음. helper가
  goroutine이나 shared worker state를 소유하지 않는다. cancellation 및 race
  동작은 대상 test로 검증했다.

## 검증

- PASS `go test -count=1 ./dynamodb/batchwrite`
- PASS `go test -race -count=1 ./dynamodb/batchwrite`
- PASS `BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -p 1 -count=1 ./dynamodb/batchwrite`
- PASS `BLUETAPE_DYNAMODB_BATCHWRITE_SMOKE=1 go test -race -p 1 -count=1 ./dynamodb/batchwrite`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## DoD Status

- [x] `BatchWriteItem` 25-item chunking 구현.
- [x] `UnprocessedItems` retry 및 retry-exhaustion error 구현.
- [x] SDK-native caller-owned client contract 보존.
- [x] Unit, race, Floci smoke test 추가.
- [x] English 및 Korean docs 갱신.
- [x] P0=0 P1=0으로 7-tier review 완료.
- [x] 최종 로컬 게이트 완료.
- [ ] GitHub CI 대기.
