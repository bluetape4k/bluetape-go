Closes #224.

## 요약

- 통합 recipe 범위: batch, workflow, cache, resilience, id, JWT, Redis lock/leader,
  Testcontainers Redis를 아우르는 compile-checked recipe를 제공하는
  `examples/integration`을 추가한다.
- service-free, race, Docker 기반 smoke 명령을 English 및 Korean package
  README에 문서화한다.
- 새 example package를 root English 및 Korean README에서 연결한다.

## 검증

- PASS `go test -count=1 ./examples/integration`
- PASS `BLUETAPE_INTEGRATION_RECIPE_SMOKE=1 go test -p 1 -count=1 ./examples/integration`
- PASS `go test -race -count=1 ./examples/integration`
- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `make lint`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## 검토

- Step 6-R: P0=0 P1=0, seven-lane 분리를 적용한 main integration fallback.
- Step 7-R: PENDING

## DoD Status

- PASS #224 실행 가능한 integration recipe.
- PASS English/Korean docs 동기화.
- PASS 로컬 validation 게이트.
- PENDING #224와의 PR metadata parity.
