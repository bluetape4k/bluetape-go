# Cancellation Assertion 교훈

## 변경된 점

Issue #213은 Go context cancellation contract를 assert하는 test-only helper를 추가했다.

- direct `context.Canceled` propagation
- direct `context.DeadlineExceeded` propagation
- blocked waiter release after cancellation
- resource cleanup observation after cancellation

## 예상과 달랐던 점

`golangci-lint`가 삭제된 sibling worktree의 stale cache entry를 재사용해 더 이상 존재하지
않는 path에 대한 diagnostics를 보고했다. cache를 지우고 lint를 다시 실행하자 stale
cache noise와 실제 current-worktree `errorlint` finding이 분리됐다.

native subagent cleanup도 hour-scale wait로 막혔다. 작은 Type B Go 작업에서는 7-tier
gate를 bounded하게 유지한다. native lane이 stale이거나 unresponsive하면 fallback을
기록하고 main session에서 여섯 review lens를 완료한다.

## 반복할 규칙

- new public assertion helper는 RED test로 시작해 implementation 전에 diagnostics를
  고정한다.
- probe가 concrete error를 반환하면 diagnostic helper에서 wrapped error를 보존한다.
  error가 반환되지 않으면 명시적 `<nil>` diagnostics를 사용한다.
- cancellation helper가 cooperative하다는 점을 문서화한다. Go는 `ctx.Done()`을 영원히
  무시하는 goroutine을 안전하게 멈출 수 없다.
- deleted-worktree lint path를 current finding으로 보기 전에 `golangci-lint cache clean`
  을 실행한다.

## 검증

- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `make fmt-check && make vet && make lint`: PASS
- `make test`: PASS
- `make race`: PASS
