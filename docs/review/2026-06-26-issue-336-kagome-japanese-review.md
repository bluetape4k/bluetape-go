# Issue 336 - Kagome Japanese Adapter Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

## 범위

Baseline: `e9ed319ce603678e8898655ff9857f58a0e8ad11`

Reviewed changes:

- `textsearch/japanese` optional Kagome adapter, examples, tests, and README.
- Root and `textsearch` README links to the optional adapter.
- `go.mod` / `go.sum` Kagome dependency additions.

## 7-Tier 검토

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

## 메모

- `GoroutineStressTester` is the concurrency helper used for reusable tokenizer
  access.
- AsyncJobTester is not applicable because the adapter does not start async
  work or accept caller-owned contexts.
