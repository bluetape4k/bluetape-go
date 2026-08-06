# Issue #49 Step 3-R Plan Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-30
Worktree: `.worktrees/issue-49-graph-io`
브랜치: `feat/issue-49-graph-io`
Baseline: `da6005c`

## 판정

P0: 0
P1: 0

The Step 3 implementation plan is approved for TDD implementation after
targeted spec and plan corrections.

## Initial Review

| Perspective | P0 | P1 | P2 | P3 | Resolution |
| --- | ---: | ---: | ---: | ---: | --- |
| Performance | 0 | 1 | 2 | 1 | Required CSV record byte limits before `encoding/csv.Reader` allocation and adversarial oversized quoted-record tests. |
| Stability | 0 | 1 | 0 | 0 | Added direct CSV terminal-state tests for double close, post-close operations, stable report reuse, and no count mutation. |
| Security | 0 | 1 | 0 | 0 | Changed CSV write default to `CSVFormulaEscape`; `CSVFormulaRaw` is explicit lossless opt-in. |
| Operator/Ops | 0 | 0 | 0 | 0 | Existing docs, make gates, CI feasibility, and release bookkeeping plan accepted. |
| Developer/API | 0 | 2 | 0 | 0 | Added exported `CSVReader`/`CSVWriter` streaming contract and removed public `FormatGraphML`. |
| User/Caller | 0 | 1 | 0 | 0 | Added direct `CSVReader`/`CSVWriter` tests and examples for header setup, read sequencing, EOF, close, and reports. |

## 적용한 수정

- CSV writes now default to `CSVFormulaEscape`; `CSVFormulaRaw` is an explicit
  lossless graph-interchange opt-in.
- CSV reads must enforce `MaxRecordBytes` before `encoding/csv.Reader` can
  allocate arbitrarily large quoted fields or logical records.
- Adversarial oversized quoted field/record tests are required.
- Public formats are limited to `FormatNDJSON` and `FormatCSV`; GraphML is
  documentation-only in #49 with no public constant or helper.
- Stateful `CSVReader` and `CSVWriter` own the CSV streaming API contract.
- Direct `CSVWriter` tests must cover explicit `PropertyColumns`, missing header
  setup, close flushing, and final counts.
- Direct `CSVReader` tests must cover vertex EOF before edge reads, repeated EOF,
  missing-endpoint fail-closed behavior, and deterministic sequencing.
- CSV terminal-state tests must cover double close, post-close read/write
  `ErrStreamClosed`, stable final report reuse, and no count mutation.
- Examples must include direct `CSVReader`/`CSVWriter` usage.

## 재실행 증거

| Perspective | P0 | P1 | Evidence |
| --- | ---: | ---: | --- |
| Performance | 0 | 0 | Rerun accepted pre-allocation CSV record byte guard, streaming boundaries, and no performance claims. |
| Stability/Ops | 0 | 0 | Rerun accepted locked CSV terminal-state tests and later race/stress verification gates. |
| Security | 0 | 0 | Rerun accepted safe CSV formula default, explicit raw opt-in, redaction, bounded input, and GraphML docs-only boundary. |
| Operator/Ops | 0 | 0 | Rerun accepted reports, docs/bookkeeping, make gates, CI feasibility, and release preconditions. |
| Developer/API | 0 | 0 | Rerun accepted exported stateful CSV contract and removal of public `FormatGraphML`. |
| User/Caller | 0 | 0 | Focused rerun accepted direct CSVReader/CSVWriter tests and examples for caller usability. |
| Main Integration | 0 | 0 | Current on-disk spec and plan have no remaining P0/P1 contradictions across review lanes. |

## 후속 게이트

- Step 4 must write the planned failing tests before implementation code.
- Step 6-R must verify the actual implementation against the corrected plan,
  including CSV pre-allocation limits, formula policy defaults, direct CSV
  stateful API behavior, and GraphML docs-only deferral.
- PR creation remains blocked until Step 6-R records P0=0/P1=0.
