Resolves #64.

## 요약

- package code, example, direct SDK 사용, 보류 범위에 대한 DynamoDB helper
  decision matrix를 기록했다.
- 좁은 `BatchWriteItem` chunking과 `UnprocessedItems` retry를 다루는 #270만
  bluetape-go 구현 후속으로 선택했다.
- conditional write와 optimistic locking은 기존 workshop scenario issue인
  bluetape-go-workshop#61로 연결했다.
- Go core repo에 광범위한 repository, mapper, expression, DAX, Spring/Ktor,
  generic client wrapper를 추가하는 방안을 거부했다.

## 검토

- Step 2-R 및 Step 6-R 7-tier review 산출물이 `docs/superpowers/reviews/`
  아래에 포함되어 있다.
- Step 6-R 검토 결과: P0=0, P1=0.
- Go stress requirement: docs/research-only diff이므로 해당 없음. 구현이
  shared state, goroutine, worker lifecycle, goroutine-safe public claim을
  도입하면 #270에서 stress 필요성을 다시 평가해야 한다.

## 검증

- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PENDING GitHub CI

## DoD Status

- [x] 이슈 #64 DynamoDB 후보를 평가했다.
- [x] Helper, example, direct SDK 간 결정을 기록했다.
- [x] 후속 helper issue #270을 생성했다.
- [x] 기존 workshop conditional repository example을 연결했다.
- [x] P0=0 P1=0으로 7-tier review를 완료했다.
- [x] 로컬 validation을 완료했다.
- [ ] GitHub CI 대기.
