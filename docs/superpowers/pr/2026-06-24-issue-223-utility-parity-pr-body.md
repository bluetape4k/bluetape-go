Resolves #223.

## 요약

- logging, time, math, measured value, money, geo, science 후보에 대한
  Go-native utility parity 경계를 문서화했다.
- 이번 milestone에서 확인한 반복 helper 수요는 `core`, `measure`, `money`가
  이미 충족하므로 현재 public API는 변경하지 않았다.
- #223 closure milestone 범위를 넘는 대형 domain에 대해 후속 research
  issue를 등록했다.

## 후속 작업

- #275는 `slog`/observability hook 경계를 평가한다.
- #276은 geo 및 coordinate utility 범위를 평가한다.
- #277은 집중 statistics 및 math utility 범위를 평가한다.

## 검토

- Step 6-R 7-tier review 산출물이 `docs/superpowers/reviews/` 아래에
  포함되어 있다.
- Step 6-R 검토 결과: P0=0, P1=0.
- Go stress: 이 PR은 Go code, goroutine, shared state, public concurrency
  claim을 추가하지 않으므로 package-specific stress helper는 해당 없음.

## 검증

- PASS `git diff --check`
- PASS `make fmt-check`
- PASS `make tidy-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `make test`
- PASS `make race`
- PENDING GitHub CI

## DoD Status

- [x] 현재 `origin/develop`에서 Worktree를 생성했다.
- [x] sibling bluetape4k module을 기준으로 #223 후보 inventory를 완료했다.
- [x] 기존 Go package coverage를 평가했다.
- [x] Kotlin/JVM-specific non-goal을 문서화했다.
- [x] 과도하게 큰 observability, geo/science, math 범위에 대해 후속 issue를
      등록했다.
- [x] P0=0 P1=0으로 Step 6-R 7-tier review를 완료했다.
- [x] 로컬 validation 게이트를 완료했다.
- [ ] GitHub CI 완료 확인.
