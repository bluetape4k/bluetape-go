# Issue #201 Test Gate 교훈

## 변경된 점

- Testcontainers cleanup에는 caller cancellation과 독립적인 bounded context가 필요하다.
  test failure 중 parent cancellation이 container termination을 건너뛰게 해서는 안 된다.
- Redis-backed test는 여러 package가 한 package run에서 많은 Redis container를 만들었기
  때문에 flaky했다. package-shared Redis fixture와 per-test `FlushDB`가 isolation을
  보존하면서 Docker/port churn을 줄였다.
- Testcontainers Redis는 이미 listening-port wait strategy를 제공한다. 이를 log-only
  wait로 덮어쓰면 readiness가 약해지고 HTTP/SSH byte가 Redis reply로 parsing되는
  잘못된 endpoint 증상을 허용한다.
- 이 repository에는 Testcontainers-backed package가 많으므로 repo-wide `make test`,
  `make race`, `make coverage`는 Go package를 `-p 1`로 실행해야 한다.

## 검증 증거

- `make ci`: serial package scheduling과 Redis fixture stabilization 뒤 PASS.
- `go test -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `go test -race -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `git diff --check`: PASS.

## 다음에 확인할 규칙

Go Testcontainers task에서는 다음을 확인한다.

- direct `container.Terminate(context.Background())`;
- custom wait strategies that replace module defaults;
- package test suites that create one external service container per test;
- full-suite `make ci`, not only targeted package tests;
- stress path가 있으면 explicit GoroutineStressTester 증거와 race 증거를 함께 둔다.
