# Issue #217 Step 6-R Code Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: [#217](https://github.com/bluetape4k/bluetape-go/issues/217)
Diff Base: `origin/develop` at `625d2e2`
날짜: 2026-06-23

## 검토 범위

- New public package `testcontainers/server`
- Wrapper adaptations in `testcontainers/{postgres,mysql,redis,kafka,nats}`
- English/Korean README updates for the same wrappers
- `go.mod` direct dependency classification for `github.com/moby/moby/api`

## 6개 관점 검토

| Tier | Perspective | P0 | P1 | P2 | P3 | Evidence |
|---|---|---:|---:|---:|---:|---|
| 1 | Performance | 0 | 0 | 0 | 0 | Adapter methods delegate directly to Testcontainers (`testcontainers/server/server.go:121`, `:130`, `:139`); only small map clones occur for test fixture details (`testcontainers/server/details.go:15`). |
| 2 | Stability | 0 | 0 | 0 | 0 | Cleanup uses bounded repo helper (`testcontainers/server/server.go:156`, `:162`); wrappers terminate started containers if server construction fails before cleanup registration (`testcontainers/redis/redis.go:42`, `testcontainers/kafka/kafka.go:58`). |
| 3 | Security | 0 | 0 | 0 | 0 | Env export validates before mutation and uses test-scoped `testing.TB.Setenv` (`testcontainers/server/env.go:17`, `:29`); no package-level global export or secret persistence. |
| 4 | Operator/Ops | 0 | 0 | 0 | 0 | Errors wrap server name/operation for diagnostics (`testcontainers/server/server.go:123`, `:132`, `:141`, `:150`); README locale pairs document dynamic host ports and fixed-port collision limits. |
| 5 | Developer/API | 0 | 0 | 0 | 0 | Public API is narrow and Go-shaped (`testcontainers/server/server.go:18`, `:33`); sentinel errors are `errors.Is` compatible (`testcontainers/server/details.go:8`, `testcontainers/server/env.go:10`, `testcontainers/server/server.go:15`). |
| 6 | User/Caller | 0 | 0 | 0 | 0 | Existing `Start` helpers remain typed and delegate through `StartServer` (`testcontainers/redis/redis.go:19`, `testcontainers/kafka/kafka.go:24`); README examples cover `StartServer` and `ExportEnv`. |

## Quick Scan

Production quick scan on changed Go paths:

```bash
rg -n "context\.TODO\(|context\.Background\(|go func|time\.Tick\(|http\.ListenAndServe\(|panic\(|RealIP|X-Forwarded-For" testcontainers/server testcontainers/{postgres,mysql,redis,kafka,nats} -g '*.go'
```

결과: only expected `context.Background()` usage in wrapper tests; no new
production goroutines, `panic`, `time.Tick`, HTTP server, or proxy-header trust
paths.

## 검증 증거

- `go test -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats`
- `go test -race -p 1 -count=1 ./testcontainers/server ./testcontainers/redis ./testcontainers/postgres ./testcontainers/mysql ./testcontainers/kafka ./testcontainers/nats`
- `make fmt-check`
- `make tidy-check`
- `make vet`
- `make lint`
- `make test`
- `make race`
- `git diff --check`

## 통합 판정

P0=0 P1=0

No blocking issue remains for PR creation. P2/P3 follow-up filing is not needed
from this review because fixed host ports and new service expansion are already
owned by later 0.6.5 issues.
