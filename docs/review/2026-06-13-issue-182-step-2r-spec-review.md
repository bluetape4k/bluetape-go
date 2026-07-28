# Issue #182 Step 2-R Spec Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-13
범위: `docs/superpowers/specs/2026-06-13-issue-182-redis-probabilistic-filters-design.md`
게이트: Step 2-R, six independent subagent lanes plus main integration review

## 판정

PASS. Latest integrated blocker count is `P0=0 P1=0`.

## 관점별 결과

| Tier | Perspective | Initial P0/P1 | Latest P0/P1 | Evidence |
|---|---|---:|---:|---|
| Tier 1 | Performance | P0=0 P1=1 | P0=0 P1=0 | Static Lua/EVALSHA and one-round-trip requirements resolved the simple-command ambiguity. |
| Tier 2 | Stability | P0=0 P1=2 | P0=0 P1=0 | `Put`, `MightContain`, `Clear`, and `BitCount` now validate config and bitmap operation in one script; Testcontainers commands use `-p 1`. |
| Tier 3 | Security | P0=0 P1=0 | P0=0 P1=0 | P2 findings were addressed by static Lua source, `KEYS`/`ARGV` only, ACL command scope, and redacted key errors. |
| Tier 4 | Operator/Ops | P0=0 P1=1 | P0=0 P1=0 | Redis Cluster multi-key script risk resolved with package-owned hash-tagged slot key layout. |
| Tier 5 | Developer/API | P0=0 P1=0 | P0=0 P1=0 | P2/P3 findings were addressed by `package redisbloom` and nil context normalization. |
| Tier 6 | User/Caller | P0=0 P1=0 | P0=0 P1=0 | P2/P3 findings were addressed by `Put(false)` caveat, `Clear` admin-path guidance, Kotlin migration notes, and config error actions. |

## 통합 변경

- Redis key layout now uses `bluetape:probabilistic:bloom:v1:{namespace}` so `{key}:bits` and `{key}:config` share a Redis Cluster hash slot.
- Constructor metadata initialization uses a static/versioned Lua script loaded via `redis.NewScript.Run` or equivalent `EVALSHA` cached path.
- `Put`, `MightContain`, `Clear`, and `BitCount` validate the stored fingerprint and perform bitmap work in one static Lua script.
- Lua source must be static; all dynamic data goes through `KEYS` and `ARGV`.
- Default errors use operation plus redacted key id or short key digest, not inserted values, full hasher keys, or full logical Redis keys.
- Test requirements now include command-count/recorder tests, static-script tests, external bitmap deletion behavior, serial Testcontainers commands, race tests, stress tests, and hot-path benchmarks.
- Documentation requirements now include `Put(false)` semantics, `Clear` misuse resistance, Redis ACL/TLS command scope, no TTL/no eviction guidance, Kotlin migration limits, and config mismatch recovery actions.

## 유예한 P2/P3 항목

No P2/P3 item remains unaddressed in the spec. Implementation and Step 3 plan must carry the added test, benchmark, documentation, and diagram requirements forward.

## 메인 통합 검토

No internal contradiction remains after the spec repair. The accepted design still matches the user-approved Approach 1: Redis-backed Bloom only, plain Redis bitmap commands, `go-redis/v9` through `redis.Cmdable`, no RedisBloom module, and no Cuckoo/HLL in this PR.

P0=0 P1=0
