# Issue 19 Circuit Breaker and Bulkhead Plan

## 분류

- 작업 유형: Type A - Full Feature.
- 근거: 새 공개 resilience API, 동시성 동작, 오류 계약, docs/spec/plan 산출물, deterministic tests를 포함한다.
- 진행 라인: issue worktree `feat/issue-19-circuit-breaker-bulkhead`.

## 순서

1. `develop` 직접 push가 보호되어 있으므로 local `.gitignore` cleanup commit을 PR branch에 포함한다.
2. 구현 전에 issue #19 research, spec, plan을 추가한다.
3. circuit breaker와 bulkhead를 위해 resilience events와 errors를 확장한다.
4. mutex-protected state, counters, injected time source, bounded half-open probes를 갖춘 circuit breaker를 구현한다.
5. first-party permit accounting, optional waiting, cancellation-safe acquisition을 갖춘 bulkhead를 구현한다.
6. circuit breaker와 bulkhead의 deterministic unit tests와 examples를 추가한다.
7. 공개 상태가 retry/timeout only에서 retry/timeout/circuit breaker/bulkhead로 바뀌면 README locale package descriptions를 갱신한다.
8. focused tests, race tests, repo tests, vet, raw golangci-lint, format, tidy-check, diff-check, CodeGraph/code-review-graph review, 7-tier review를 실행한다.
9. milestone `0.2.0`, assignee `debop`, issue links, problem context, solution summary, validation, final DoD status가 포함된 PR을 연다.

## 리뷰 게이트

다음 항목을 확인한다.

- 외부 runtime resilience dependency 또는 wrapper가 없는지 확인한다.
- 기존 `Policy[T]`와의 조합이 자연스러운지 확인한다.
- 동시성 아래 state transition이 올바른지 확인한다.
- half-open admission이 deterministic한지 확인한다.
- context cancellation과 permit release가 누락되지 않는지 확인한다.
- error sentinel과 typed error 동작을 확인한다.
- #21 event hook 호환성을 확인한다.
- 테스트가 deterministic하고 race-safe한지 확인한다.

## 검증 게이트

완료 전에 다음을 실행한다.

- `go test -count=1 ./resilience`
- `go test -race -count=1 ./resilience`
- `go test -count=1 ./...`
- `go vet ./...`
- `golangci-lint run ./...`
- `make fmt-check`
- `go mod tidy && git diff --exit-code -- go.mod go.sum`
- `git diff --check`
- CodeGraph status와 code-review-graph review context
