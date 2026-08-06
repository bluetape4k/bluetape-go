# Issue 182 PR Review - Redis Probabilistic Filters

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-13
PR: https://github.com/bluetape4k/bluetape-go/pull/229
기준: `develop`
Head: `issue-182-redis-probabilistic-filters`
범위: PR #229 live diff plus local Step 7-R follow-up fixes before final push.

## 게이트 형태

Step 7-R used the required 7-Tier shape:

1. Performance subagent lane
2. Stability subagent lane
3. Security subagent lane
4. Operator/Ops subagent lane
5. Developer/API subagent lane
6. User/Caller subagent lane
7. Main-session integration review

No seventh integration subagent was used. The main Codex session owned
deduplication, severity normalization, fallback review, and final P0/P1 verdict.

## 관점별 결과

| Lane | Result | P0 | P1 | P2 | Notes |
|---|---:|---:|---:|---:|---|
| Performance | PASS | 0 | 0 | 0 | Redis Lua hot paths, command-count tests, and benchmark matrix reviewed. |
| Stability | PASS | 0 | 0 | 0 | Race, cancellation, config mismatch, external deletion, and goroutine stress evidence reviewed. |
| Security | PASS with fallback | 0 | 0 | 0 | Initial P2s fixed. Final rerun lane shut down; main integration fallback performed. |
| Operator/Ops | PASS | 0 | 0 | 0 | Runbook, eviction/no-false-negative caveats, ACL/TLS commands, README/CHANGELOG/WIP consistency reviewed. |
| Developer/API | PASS | 0 | 0 | 0 | External examples, Redis-free offset benchmark, and portable SVG/font output verified. |
| User/Caller | PASS | 0 | 0 | 0 | README quickstart, WIP wording, and caller-followable examples reviewed. |
| Main integration | PASS | 0 | 0 | 0 | Final diff, docs, diagram, PR body, CI status, and timeout fallback evidence integrated. |

Final Step 7-R gate: `P0=0 P1=0`.

## Step 7-R 중 수정한 발견 사항

- Security P2: namespace and hasher sensitive-marker validation was too narrow.
  Fixed by centralizing marker checks and rejecting token, secret, password,
  credential, email-like values, and API key variants including `api-key`,
  `api_key`, `api.key`, `api:key`, and `apikey`.
- Developer/API P2: examples did not compile-check the external import path.
  Fixed by moving Redis Bloom examples to `package redisbloom_test` and importing
  `github.com/bluetape4k/bluetape-go/probabilistic/redis`.
- Developer/API P2: `BenchmarkRedisBloomOffsets` unnecessarily required Redis.
  Fixed by constructing the benchmark filter state in memory.
- Developer/API P2: generated Redis Bloom SVG embedded local font paths. Fixed
  by removing `@font-face` absolute path generation and regenerating the SVG/PNG.
- User/Caller P2: README quickstart used an undefined `value`. Fixed in English
  and Korean README snippets.
- User/Caller P2: WIP wording implied Redis Bloom remained unfinished. Fixed to
  say Bloom is delivered by #182 / PR #229, with Cuckoo/HyperLogLog left as
  follow-up scope.

## 타임아웃/대체 처리 메모

The final Security rerun subagent did not return a completed finding after the
bounded wait and was closed. This lane is recorded as:

`lane timed out; main integration fallback performed`

Main integration fallback reviewed:

- `probabilistic/redis/options.go`
- `probabilistic/redis/options_test.go`
- changed examples, docs, benchmark, and diagram generator for security
  regressions

Fallback evidence:

- `go test -p 1 -race -count=1 ./probabilistic/redis -run 'TestValidateNamespaceRejectsUnsafeNames|TestHasherKeyRejectsSensitiveOrUnsafeNames'`
- `gopls check probabilistic/redis/options.go probabilistic/redis/options_test.go probabilistic/redis/example_test.go probabilistic/redis/filter_benchmark_test.go`
- `rg -n "/Users/debop|font-face|BLUETAPE_.*FONT|Library/Fonts" scripts/generate-redis-bloom-diagram.mjs docs/images/readme-diagrams/redis-bloom-key-layout-01.svg || true`

No P0/P1/P2 findings remain after fallback review.

## 검증 증거

- `gopls check probabilistic/redis/options.go probabilistic/redis/options_test.go probabilistic/redis/example_test.go probabilistic/redis/filter_benchmark_test.go`
- `go test -p 1 -count=1 ./probabilistic/redis`
- `go test -p 1 -race -count=1 ./probabilistic/redis`
- `go test -count=1 ./probabilistic ./probabilistic/internal/bloomhash`
- `go test -race -count=1 ./probabilistic ./probabilistic/internal/bloomhash`
- `go test -p 1 -run '^$' -bench 'BenchmarkRedisBloom(Put|MightContain|Offsets)' -benchtime=100ms -benchmem ./probabilistic/redis`
- `node scripts/generate-redis-bloom-diagram.mjs`
- `node --check scripts/generate-redis-bloom-diagram.mjs`
- `xmllint --noout docs/images/readme-diagrams/redis-bloom-key-layout-01.svg`
- `file docs/images/readme-diagrams/redis-bloom-key-layout-01.png docs/images/readme-diagrams/redis-bloom-key-layout-01-graphviz.png`
- `git diff --check`
- `make ci`

Remote PR evidence before final follow-up push:

- PR body final `##` heading: `## DoD Status`
- GitHub merge state: `CLEAN`
- GitHub check `ci`: `SUCCESS`

## 최종 판정

Step 7-R PASS.

P0=0 P1=0
