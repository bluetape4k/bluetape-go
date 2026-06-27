# Issue 336 - Kagome Japanese Adapter Review

## Scope

Baseline: `e9ed319ce603678e8898655ff9857f58a0e8ad11`

Reviewed changes:

- `textsearch/japanese` optional Kagome adapter, examples, tests, and README.
- Root and `textsearch` README links to the optional adapter.
- `go.mod` / `go.sum` Kagome dependency additions.

## 7-Tier Review

| Lane | Verdict | Evidence |
|---|---|---|
| Performance | PASS | Adapter constructs immutable Kagome tokenizer once; tokenization allocates only response tokens/metadata. Kagome dictionary cost is optional and isolated to `textsearch/japanese`. |
| Stability | PASS | Request validation reuses `textsearch.NewTokenizeRequest`; span validation returns errors if Kagome emits inconsistent byte ranges. |
| Security | PASS | README states tokenization/blockword helpers are not security classifiers or policy boundaries. No auth/deserialization/command surface added. |
| Operator/Ops | PASS | README documents dictionary size/deployment cost and keeps UniDic outside default runtime boundary. |
| Developer/API | PASS | API is narrow: `NewTokenizer`, options, mode aliases, metadata constants, noun/verb filters. Core `textsearch` imports remain dependency-free. |
| User/Caller | PASS | Tests and examples cover Japanese tokenization, byte spans, POS metadata, blockword composition, dictionary options, normalization, and concurrent reads. |
| Integration | PASS | Main-session integration review found no P0/P1 blockers after local validation. |

P0=0 P1=0

## Notes

- `GoroutineStressTester` is the concurrency helper used for reusable tokenizer
  access.
- AsyncJobTester is not applicable because the adapter does not start async
  work or accept caller-owned contexts.
