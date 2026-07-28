# Issue 569 Redis Foundation 교훈

## Context

Issue #569는 package name `btredis`를 쓰는 public
`github.com/bluetape4k/bluetape-go/redis` package를 추가했다. 이 package는 foundation
slice일 뿐이다. Redis key builder, owner token, lease script, TTL validation,
redacted operation error를 제공한다. 기존 Redis-backed package는 #570이 old/new
parity와 benchmark impact를 증명할 때까지 migration하지 않는다.

## Lessons

- Public Redis foundation helper는 external Redis I/O를 dispatch하기 전에 nil
  context, pre-canceled context, nil client, typed nil client, invalid lease,
  invalid TTL을 거절해야 한다.
- Owner token은 diagnostic value가 아니라 credential이다. `String`, `GoString`,
  `slog.LogValuer`는 기본 redacted로 유지하고, raw value는 명시적으로 sensitive한
  Redis argument method를 통해서만 노출한다.
- Structural Redis key part와 caller-owned logical key에는 별도 API가 필요하다.
  structural part는 delimiter와 brace를 거절할 수 있지만, caller가 key namespace를
  소유하는 logical key는 caller byte를 verbatim 보존해야 한다.
- Redis script cancellation에는 split boundary가 있다. pre-dispatch cancellation은
  caller-owned context error로 반환할 수 있지만, post-dispatch cancellation은 commit
  state가 불확정이므로 runbook concern으로 문서화해야 한다.
- Redis Cluster hash tag는 compatibility surface다. `WithHashTag`는 empty/braced
  tag를 거절해야 하지만 colon-bearing tag는 보존해야 한다. 기존
  `probabilistic/redis` namespace가 그 형태에 의존하기 때문이다.
- 새 shared Redis primitive는 migration 전에 도입한다. Migration PR은 package-local
  helper를 조용히 교체하지 말고 old/new key parity test와 provider benchmark
  evidence를 포함해야 한다.

## Verification Notes

- 각 production slice 전에 TDD red command를 실행했다.
  `go test -count=1 ./redis -run 'OwnerToken|NewOwnerToken|ParseOwnerToken'`,
  `go test -count=1 ./redis -run 'Key|TTL|OpError|Redacted'`, and
  `go test -count=1 ./redis -run 'Lease|CompareAnd'`.
- 실제 Redis script behavior는 serial package execution 아래 repo-local
  `testcontainers/redis` fixture로 검증했다.
- 이 branch의 final verification에는 `go test -p 1 -count=1 ./redis`,
  `go test -p 1 -race -count=1 ./redis`, `go test -count=1 ./redis -run Example`,
  `git diff --check`, no-migration import scan, `make ci`가 포함되었다.
