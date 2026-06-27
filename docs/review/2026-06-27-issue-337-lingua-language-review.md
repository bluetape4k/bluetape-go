# Issue 337 - Lingua Language Adapter Review

## Scope

Baseline: `c6820df05e2dc435ec54436ed43d7bc3a57ef124`

Reviewed changes:

- `textsearch/language` optional Lingua-Go adapter, examples, tests, and README.
- Root and `textsearch` README links to the optional adapter.
- `go.mod` / `go.sum` Lingua-Go dependency additions.

## 7-Tier Review

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

## Notes

- `GoroutineStressTester` covers shared detector reuse.
- AsyncJobTester is not applicable because the adapter starts no async work and
  Lingua's detector API is synchronous.
