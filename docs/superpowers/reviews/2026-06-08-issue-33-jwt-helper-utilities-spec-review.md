# Issue #33 JWT Helper Utilities Spec Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

Review type: Step 2-R spec review
Spec: `docs/superpowers/specs/2026-06-08-issue-33-jwt-helper-utilities-spec.md`
이슈: #33
Milestone: 0.6.0
검토일: 2026-06-08

## 입력

- GitHub issue #33: assignee `debop`, milestone `0.6.0`, labels
  `type: task`, `priority: p1`, `area: utilities`.
- Kotlin parity source: `bluetape4k-projects/utils/jwt`.
- Follow-up issues created during scoping:
  - #173: distributed JWT KeyChain repositories.
  - #174: safe JWT compression and JOSE dependency scope.
  - #175: optional JWT provider cache adapters.
- Dependency evidence:
  - `github.com/golang-jwt/jwt/v5` selected for #33 core.
  - `lestrrat-go/jwx` and `go-jose/go-jose` deferred to #174.

## 서브에이전트 결과

| Reviewer | Role | Initial P0 | Initial P1 | Initial P2 | Initial P3 | Verdict |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| Epicurus | test-engineer | 0 | 1 | 2 | 0 | Failed until deterministic clock/entropy/key-generation tests were required. |
| Godel | dependency-expert | 0 | 0 | 2 | 1 | Passed, with security hardening improvements requested. |
| Kierkegaard | architect | 0 | 1 | 2 | 0 | Failed until repository context/future distributed API boundary was fixed. |
| Herschel | critic closure | 0 | 0 | 0 | 0 | Passed after P1/P2 closure. |

## 차단 항목 종료

| Finding | Severity | Resolution |
| --- | --- | --- |
| Deterministic clock/entropy/key-generation tests were not required. | P1 | Spec now requires test-local entropy without global mutation at lines 232-233, no package-global entropy/clock/parser mutation at line 278, deterministic clock assertions at lines 357-358, and deterministic entropy/failure assertions at lines 359-361. |
| `KeyChainRepository` looked like a future public distributed repository API but had no `context.Context`. | P1 | Spec now uses unexported `keyChainRepository` at lines 239-246 and explicitly scopes it to #33 in-memory only at lines 249-252. #173 was updated to require a context-aware distributed repository contract. |

## 비차단 수정 적용

| Finding | Severity | Resolution |
| --- | --- | --- |
| `Reader.Token()` exposed the raw bearer token. | P2 | Removed from the proposed reader API; lines 199-201 now forbid exposing the original bearer token as stable API. |
| `Provider` interface was too broad. | P2 | Replaced with narrow `Signer`, `Parser`, and `Rotator` interfaces at lines 138-152; concrete providers may implement multiple interfaces. |
| `zip` and JOSE control headers were not reserved. | P2 | Lines 284-288 reject `alg`, `kid`, `zip`, `crit`, `jku`, `jwk`, `x5u`, and `x5c` as caller-controlled headers. |
| Retained key expiry behavior was underspecified. | P2 | Lines 265-270 require previous retained keys to be non-expired and define expired/evicted `kid` failure. Lines 377-378 require tests. |
| Error non-leakage tests were only implied. | P3 | Lines 371-373 now require representative error text to exclude tokens, private keys, and symmetric secrets. |
| Stress invariant was too loose. | P2 | Lines 379-384 now name concurrent rotate/parse/eviction/reader mutation invariants. |
| `AsyncJobTester` N/A path was too weak. | P2 | Lines 385-390 require verifier evidence when #33 has no context-aware cancellation boundary. |

## Gate

Final Step 2-R counts: P0=0, P1=0, P2=0, P3=0.

Gate verdict: PASS. The spec may advance to plan.

## 검증

```bash
git diff --check
rg -n "type Signer|type Parser|type Rotator|type keyChainRepository|context-aware|zip|crit|deterministic injected|AsyncJobTester N/A|expired retained" docs/superpowers/specs/2026-06-08-issue-33-jwt-helper-utilities-spec.md
gh issue view 173 --json body --jq .body
```
