# Issue #216 Testcontainers Contract Hardening 교훈

날짜: 2026-06-23
Issue: #216

## 변경된 점

- 기존 Testcontainers wrapper에는 이미 bounded cleanup이 있었지만, public failure text가
  caller와 CI operator에게 너무 generic했다.
- connection detail name은 return value로만 암시됐다. downstream example에는 map,
  report, fixture metadata에 사용할 stable key name이 없었다.
- Docker smoke test가 `t.Parallel()`을 사용했고, 이는 milestone 0.6.5의 serial
  resource-containment rule과 충돌했다.

## 해결을 증명한 증거

- 변경 전에 `go test -p 1 -count=1 ./testcontainers/...`가 통과해 wrapper가 happy
  path에서 동작함을 증명했다.
- new synthetic start-error test는 local Docker daemon/image failure를 강제하지 않고
  원하는 diagnostic category를 증명했다.
- cleanup test는 이제 skipped subtest와 repeated terminate call을 포함한다.
- Testcontainers `t.Parallel()` call 제거 뒤 repo-wide `make test`와 `make race`가
  모두 통과했다.

## 다음 규칙

- Testcontainers helper issue에서는 synthetic diagnostic test를 먼저 추가하고, real
  Docker는 happy-path smoke와 race gate에만 사용한다.
- README example을 실제 API key 및 serial test command와 맞춘다.
- `golangci-lint`가 제거된 worktree path를 보고하면 current-branch failure로 보기 전에
  cache를 정리한다.
