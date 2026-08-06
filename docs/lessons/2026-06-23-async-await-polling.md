# Async Await Polling Helper 교훈

## 변경된 점

Issue #211은 root `testing` package에 context-aware await/polling helper를 추가했다.

- `CheckAwait` and `RequireAwait`
- `CheckAwaitValue` and `RequireAwaitValue`
- `CheckAwaitError` and `RequireAwaitError`
- `AwaitResult`, `AwaitStatus`, `AwaitProbe`, and `AwaitCheck`

이 helper들은 기존 Gomega-backed `Eventually`, `Consistently` wrapper를 대체하지 않고
보완한다.

## 예상과 달랐던 점

repository lint rule은 `Require*` test helper에서도 `context.Context`가 첫 번째
argument여야 한다고 요구한다. 초기 `RequireAwait(tb, ctx, ...)` 형태가 test assertion에는
더 익숙했지만 revive가 거부했다. 최종 API는 `RequireAwait(ctx, tb, ...)`이고 README
example도 이 순서를 따른다.

## 반복할 규칙

- await/polling helper는 cooperative하고 synchronous하게 유지한다. context를 무시하는
  probe는 여전히 자기 test를 block할 수 있다. helper는 이를 timeout 뒤의 goroutine
  leak로 숨기지 말고 문서화해야 한다.
- caller-owned `context.Canceled` 또는 `context.DeadlineExceeded`는 retry하지 않는다.
- probe 내부 logging 없이도 timeout diagnostics가 유용하도록 최종 observed value/error와
  attempt count를 반환한다.
- 반복 bounded goroutine stress에는 `testing/concurrency`를 사용하고, root `testing`
  helper는 작고 assertion-focused하게 유지한다.

## 검증

- `go test -count=1 ./testing`: PASS
- `go test -race -count=1 ./testing`: PASS
- `go test -count=1 ./testing/...`: PASS
- `make fmt-check && make vet && make lint`: PASS
- `make test`: PASS
- `make race`: PASS
