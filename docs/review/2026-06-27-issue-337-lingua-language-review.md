# Issue 337 - Lingua Language Adapter Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

Baseline: `c6820df05e2dc435ec54436ed43d7bc3a57ef124`

Reviewed changes:

- `textsearch/language` optional Lingua-Go adapter, examples, tests, and README.
- Root and `textsearch` README links to the optional adapter.
- `go.mod` / `go.sum` Lingua-Go dependency additions.

## 7-Tier 검토

| Lane | Verdict | Evidence |
|---|---|---|
| Performance | PASS | API encourages detector reuse and caller-selected subsets; lazy/preloaded and low-accuracy modes are exposed explicitly. |
| Stability | PASS | Blank/too-long input uses sentinel errors; unknown detection returns `Detected=false`; mixed-language byte spans are validated before returning sections. |
| Security | PASS | README states language detection is not a security, moderation, compliance, or authorization boundary. |
| Operator/Ops | PASS | README documents model memory behavior, lazy vs preloaded loading, low accuracy mode, and reuse lifecycle. |
| Developer/API | PASS | API is narrow and Go-shaped: constructors, options, result structs, confidence values, mixed sections, and small Unicode script helpers. |
| User/Caller | PASS | Tests cover subset detection, blank/unknown input, confidence ranking, mixed sections, script helpers, and concurrent detector reuse. |
| Integration | PASS | Core `textsearch` dependency boundary is verified with `go list -deps ./textsearch` showing no Lingua output. |

P0=0 P1=0

## 메모

- `GoroutineStressTester` covers shared detector reuse.
- AsyncJobTester is not applicable because the adapter starts no async work and
  Lingua's detector API is synchronous.
