# Issue #542 HTTP middleware 적합성 구현 lesson

## 문맥

Issue [#542](https://github.com/bluetape4k/bluetape-go/issues/542)는
Epic [#540](https://github.com/bluetape4k/bluetape-go/issues/540)의 후속 작업으로,
`web`, `resilience`, `ratelimit`의 `net/http` 경계를 같은 입력과 관찰 계약으로
검증하는 test support를 요구했다. 승인한 설계와 계획에 따라
`feat/web-api-542` worktree에서 구현했으며, Gin/Echo/Fiber adapter와 새
production middleware는 범위에 넣지 않았다.

## 선택과 결과

- 루트 `webtest` 패키지에 표준 라이브러리만 사용하는 `Adapter`, `Scenario`,
  `Observation`, `Run`을 추가했다. 각 scenario는 새 request, recorder,
  observer를 사용하고 `Observation`의 header/body는 복사한다.
- `Run`은 기본 2초 timeout을 사용하고, timeout 뒤 request context를 취소한
  다음 같은 상한으로 bounded cleanup을 기다린다. 결과 channel은 buffered라서
  늦게 반환한 handler가 runner를 막지 않는다.
- `CloseTracker`는 retryable `RoundTripper` response body의 close 소유권을
  확인하는 fixture로만 공유했다. Handler runner와 transport runner를 하나의
  추상화로 합치지 않았다.
- `net/http` 적합성 case는 problem 응답, trusted proxy context, resilience
  error/timeout/panic 경계, rate-limit key/rejection/backend error, retry body
  close, synchronous hook과 case isolation을 고정한다.
- 루트와 패키지 English/Korean README에 test-only 경계와 비목표를 같은 의미로
  기록했다. `webtest`는 runtime policy 목록이 아니라 test support 목록에만
  둔다.

## 실제 검증과 예상 밖의 점

- 구현 전 `go test -count=1 ./webtest`는 package 구현과 public symbol이 없어
  실패했고, harness를 추가한 뒤 green으로 전환했다.
- `go test -count=1 ./webtest`, `go test -race -count=1 ./webtest`,
  `go test -run '^Example' -count=1 ./webtest`, 관련 네 package의 normal/race
  test, 전체 `go test -p 1 -count=1 ./...`, `make race`, `make ci`가 통과했다.
- 처음 전체 `make lint`는 삭제된 이전 worktree 경로를 참조하는 stale
  `golangci-lint` cache 때문에 실패했다. 현재 worktree 소스의 문제로 단정하지
  않고 `golangci-lint cache clean` 후 전체 lint를 다시 실행해 `0 issues`를
  확인했다.
- harness 자체의 `t.Fatal` 입력 검증과 timeout 실패는 부모 test process가
  실패를 관찰해야 하므로 child process test로 고정했다. 각 child process에는
  5초 상한을 두어 잘못된 검증이 hang으로 바뀌지 않게 했다.
- 계획의 사전 취소 계약을 최종 검토에서 다시 대조해
  `TestRunPreservesPreCancelledRequest`를 추가했다. in-flight 취소와 함께
  request context가 이미 취소된 경우도 보장한다.

## 다음 adapter를 위한 guard

1. Gin/Echo/Fiber adapter는 `webtest.Adapter` 경계를 사용하고,
   `Observation`이 제공하는 status/body/context/next 호출만으로 같은 case를
   연결한다. framework-specific global state를 harness에 넣지 않는다.
2. timeout을 늘려 flaky test를 숨기지 말고, handler가 cancellation을
   관찰하고 bounded cleanup으로 반환하는지를 먼저 확인한다.
3. retryable response body처럼 caller가 소유한 resource만
   `CloseTracker`로 검증한다. server incoming request body의 수명을
   middleware contract로 잘못 확장하지 않는다.
4. proxy forwarding header는 명시적인 trust predicate 없이는 신뢰하지 않고,
   event/log callback은 각 case가 소유하는 recorder로 관찰한다.
5. benchmark 수치와 adapter별 성능 결론은 Issue #560 범위로 남긴다.

## SPW-01~05 기록

- **SPW-01 — PASS:** 독자, Issue/Epic, worktree, 구현 범위와 중지 조건을
  고정했다.
- **SPW-02 — PASS:** 선택, 결과, 검증, 예상 밖의 점, 다음 adapter guard를
  단계와 실제 파일/API에 연결했다.
- **SPW-03 — PASS:** 한국어 자연스러움 checklist를 적용해 구체 동사와
  일관된 `harness`/`scenario`/`소유권` 용어를 사용했다.
- **SPW-04 — PASS:** 설계/계획의 timeout, cancellation, body close, trust,
  event isolation 계약을 소스와 테스트 결과로 대조했다.
- **SPW-05 — PASS:** 저장 후 제목, 링크, 명령, issue 범위, locale parity,
  P0/P1 판정을 다시 읽고 현재 구현과 불일치가 없음을 확인한다.
