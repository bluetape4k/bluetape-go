# Issue #592 Probabilistic Redis Key Builder Code Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-10 KST
게이트: Step 6 / Step 6-R
Baseline: `068df42615303090be1f57e03a494b596962e8e7`
Reviewed production/test diff:

- `probabilistic/redis/keys.go`
- `probabilistic/redis/options_test.go`

Native reviewer spawning is not exposed in this session. The changed slice is
small and tightly coupled, so this gate uses independent local-equivalent
six-perspective passes plus main-session integration.

## 발견 사항

| Perspective | P0 | P1 | P2 | P3 | Evidence and verdict |
|---|---:|---:|---:|---:|---|
| Performance | 0 | 0 | 0 | 0 | `keys.go:28-65` only builds the same local key values before existing Redis calls. No command, script, command count, algorithm, contention, or benchmark claim changes. |
| Stability | 0 | 0 | 0 | 0 | `keys.go:68-92` validates before shared construction and hides impossible builder failures; serial normal/race Testcontainers tests passed. No context, goroutine, timer, client lifecycle, retry, or cleanup path changed. |
| Security | 0 | 0 | 0 | 0 | `keys.go:68-80` retains package namespace validation; `keys.go:95-98` retains the short local redacted ID. `options_test.go:31-49,73-97` asserts no shared validation/error or marker key leak. |
| Operator/Ops | 0 | 0 | 0 | 0 | Exact Cluster hash-tag bytes are asserted. No key migration, state cleanup, config, telemetry, or runbook change; rollback is a commit revert. |
| Developer/API | 0 | 0 | 0 | 0 | Private adapter is confined to `keys.go`; no exported API or `RedisError`/metadata sentinel source changed. Production concurrency quick scan of changed files returned zero hits. |
| User/Caller | 0 | 0 | 0 | 0 | Valid colon namespaces and invalid namespaces retain their contracts. No caller-visible behavior changed, so README/README.ko, changelog, diagram, and release-note changes are N/A. |

## 통합

P0=0 P1=0 P2=0 P3=0

The diff is deliberately narrow: shared `KeyBuilder` is adopted only after the
provider's validation boundary and only its `Value` is used. The provider keeps
the existing `redactedRedisKeyID`, public `RedisError`, Bloom metadata sentinel
mapping, Lua scripts, and HyperLogLog command behavior. Focused formatter,
tidy, vet, serial normal tests, serial race tests, and `git diff --check` are
all fresh PASS evidence.

## 유예/거절

| Item | Decision | Rationale |
|---|---|---|
| Shared `Key.RedactedID` | Reject | It is a different 24-hex public diagnostic format. |
| Shared validation errors | Reject | Caller-visible probabilistic namespace policy remains local. |
| Benchmark | N/A | No performance claim or command/algorithm change; #560 owns the table, chart, and analysis. |
| Diagram | N/A | This is a private construction refactor with no system/data-flow topology change. |
