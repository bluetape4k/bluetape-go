# Issue #175 Step 7-R PR Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

이슈: #175
PR: #230
날짜: 2026-06-14
브랜치: `issue-175-jwt-provider-cache-adapters`
게이트: 7-Tier = 6 independent lanes + main integration review

## 검토 범위

- PR diff against `develop` for #230.
- Follow-up patch after Step 7-R comments:
  - `jwt/cache_key.go`
  - `jwt/cache_key_test.go`
  - `jwt/README.md`
  - `jwt/README.ko.md`
  - `scripts/generate-jwt-provider-cache-adapter-diagram.mjs`
  - `docs/images/readme-diagrams/jwt-provider-cache-adapter-flow.{png,svg}`
  - `docs/lessons/2026-06-14-jwt-provider-cache-adapters.md`

## 초기 관점 결과

| Tier | Perspective | Result | P0 | P1 | P2 | P3 | Notes |
|---|---|---:|---:|---:|---:|---:|---|
| 1 | Performance | COMMENT | 0 | 0 | 3 | 0 | Cache-key allocation, cold-miss benchmark setup, Redis command-count metric follow-ups. |
| 2 | Stability | COMMENT | 0 | 0 | 1 | 0 | Distributed warm-hit revalidation can evict valid entries on transient repository errors. |
| 3 | Security | COMMENT | 0 | 0 | 1 | 0 | Cache namespace prefix/scope fields needed length framing. |
| 4 | Operator/Ops | COMMENT | 0 | 0 | 0 | 1 | Lesson/PR body metadata drift. |
| 5 | Developer/API | COMMENT | 0 | 0 | 0 | 1 | Distributed README snippet used unbounded context. |
| 6 | User/Caller | COMMENT | 0 | 0 | 1 | 2 | Warm-hit caveat, diagram key-expiry label, and Redis snippet ergonomics. |
| Main | Integration | REQUEST_CHANGES | 0 | 0 | 3 | 4 | All blockers were P2/P3 only; small local fixes were applied before final verdict. |

## Follow-Up Fixes

| Finding | Severity | Resolution |
|---|---:|---|
| Cache namespace fields concatenated without framing. | P2 | Reworked top-level cache-key fields to use length-framed `appendField`/`appendHexField`; added delimiter-bearing prefix/scope regression test. |
| Warm-hit docs implied broader validation than implementation performs. | P2 | Documented that warm-hit revalidation checks live `kid`/algorithm/expiry but does not rerun signature validation or fingerprint key material; cache safety depends on non-reused `kid` values. |
| Distributed README snippet used `context.Background()`. | P3 | Replaced with `context.WithTimeout` and `defer cancelOp()` in EN/KO snippets. |
| Diagram bounded lifetime card omitted key expiry. | P3 | Regenerated diagram with "entry cannot outlive claim or key expiry". |
| Lesson artifact still had pending PR metadata. | P3 | Updated related PR to #230. |

## Affected Rerun And Fallback

| Perspective | Method | Result | P0 | P1 | P2 | P3 | Notes |
|---|---|---:|---:|---:|---:|---:|---|
| User/Developer | Affected rerun lane | APPROVE | 0 | 0 | 0 | 0 | Confirmed bounded context, `kid` caveat, key-expiry diagram label, lesson PR number, and cache-key regression. |
| Security | Main-role fallback + late affected rerun | APPROVE | 0 | 0 | 0 | 0 | Main session switched into Security role after native-agent instability; late Security attempt 2 also returned APPROVE with P0=0 P1=0. |

Subagent note: after the user directed that subagents are unstable for this
session, the remaining affected Security confirmation was completed by the main
session in a read-only Security role. The later Security affected-rerun result
matched the main fallback verdict. No further subagent result is required for the
final gate.

## Main Security Fallback Evidence

- `jwt/cache_key.go` now length-frames `prefix`, `scope`, `alg`, `token`, and
  `profile` fields, so delimiter-bearing caller input cannot collapse top-level
  cache-key namespaces.
- `jwt/cache_key_test.go` includes
  `TestCacheProfileKeyFramesPrefixAndScope`, proving delimiter-bearing
  prefix/scope pairs do not collide.
- `jwt/README.md` and `jwt/README.ko.md` explicitly state that warm-hit
  revalidation does not rerun signature validation or fingerprint key material,
  and relies on non-reused `kid` values.
- Targeted JWT cache tests and race checks passed after the follow-up patch.

## 검증 증거

- `git diff --check`
  - PASS
- `go test -count=1 ./jwt -run 'CacheProfile|CachedProvider|CachedDistributedProvider|TTL|Stale|ParseFailure|AsyncCancellation|Wrong|Algorithm|Malformed|FailureDoesNotCache'`
  - PASS, `ok github.com/bluetape4k/bluetape-go/jwt`
- `go test -race -count=1 ./jwt ./cache ./testing/concurrency`
  - PASS, `jwt`, `cache`, and `testing/concurrency`
- `make ci`
  - PASS after follow-up patch; includes `vet`, `lint` with `0 issues`, and
    repository package tests including Testcontainers-backed packages.
- `node scripts/generate-jwt-provider-cache-adapter-diagram.mjs`
  - PASS: `nodes=11 routes=9 segments=18 badEndpointAngle=0 badBends=0 interiorCrossings=0 nodeOverlaps=0 laneClearance=0 marginImbalance=0 margins=L48/R48/T48/B48 titleGap=58`
- `xmllint --noout` for generated SVG assets
  - PASS
- GitHub PR #230 CI before the follow-up commit
  - PASS, merge state `CLEAN`

## Final Verdict

APPROVE.

Final gate:

- P0 = 0
- P1 = 0

Remaining P2/P3 performance and transient distributed backend-efficiency notes
are non-blocking follow-ups. The Step 7-R blocker gate is closed.
