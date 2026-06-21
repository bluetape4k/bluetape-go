# Issue #201 Test Gate Lessons

## What Changed

- Testcontainers cleanup needs a bounded context that is independent from caller cancellation. Parent cancellation during test failure should not skip container termination.
- Redis-backed tests were flaky because several packages created many Redis containers in one package run. Package-shared Redis fixtures plus per-test `FlushDB` preserved isolation while reducing Docker/port churn.
- Testcontainers Redis already provides a listening-port wait strategy. Overriding it with a log-only wait weakened readiness and allowed wrong endpoint symptoms such as HTTP/SSH bytes being parsed as Redis replies.
- Repo-wide `make test`, `make race`, and `make coverage` must run Go packages with `-p 1` because this repository includes many Testcontainers-backed packages.

## Validation Evidence

- `make ci`: PASS after serial package scheduling and Redis fixture stabilization.
- `go test -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `go test -race -count=1 ./testing/concurrency ./cache/rediscoord ./jwt ./leader/redis ./probabilistic/redis -run 'GoroutineStressTester|Stress'`: PASS.
- `git diff --check`: PASS.

## Next-Time Rule

For Go Testcontainers tasks, check for:

- direct `container.Terminate(context.Background())`;
- custom wait strategies that replace module defaults;
- package test suites that create one external service container per test;
- full-suite `make ci`, not only targeted package tests;
- explicit GoroutineStressTester evidence plus race evidence when stress paths exist.
