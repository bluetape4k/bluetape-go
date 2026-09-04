Resolves #222.

## 요약

- 추가한 항목: table test, package-local builder, golden file, deterministic random data,
  cancellation assertion을 대상으로 한 compile-checked `testing` example을
  추가했다.
- Go-native testing 형태를 English 및 Korean README에 문서화했다.
- assertion DSL, faker dependency, JUnit 방식 parameter-source API는 포함하지
  않고 범위를 의도적으로 좁게 유지했다.

## 검토

- Step 6-R 7-tier review 산출물이 `docs/superpowers/reviews/` 아래에
  포함되어 있다.
- Step 6-R 검토 결과: P0=0, P1=0.
- Go stress: `GoroutineStressTester`와 `AsyncJobTester`는 해당 없음. example은
  shared state, goroutine lifecycle, goroutine-safe public claim을 추가하지
  않는다.

## 검증

- PASS `go test -count=1 ./testing`
- PASS `go test -race -count=1 ./testing`
- PASS `make fmt-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `git diff --check`
- PASS staged `make tidy-check`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## DoD Status

- [x] Table-test example을 추가했다.
- [x] Package-local fixture builder example을 추가했다.
- [x] package `testdata` 아래에 golden-file example을 추가했다.
- [x] Deterministic random data example을 추가했다.
- [x] Cancellation assertion example을 추가했다.
- [x] English 및 Korean docs를 갱신했다.
- [x] P0=0 P1=0으로 7-tier review를 완료했다.
- [x] 최종 로컬 게이트를 완료했다.
- [ ] GitHub CI 대기.
