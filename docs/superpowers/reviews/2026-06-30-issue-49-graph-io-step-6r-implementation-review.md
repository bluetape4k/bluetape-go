# Issue #49 Step 6-R Implementation Review

> 한국어 리뷰 경계: 이 문서는 리뷰 판정과 근거를 한국어 독자가 추적할 수 있도록 정리한다. 심각도 토큰, 판정 토큰, 파일 경로, 라인 번호, 이슈/PR 번호, 명령, 코드 식별자는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-30
Milestone: 0.10.0
범위: `graph/graphio` NDJSON and paired CSV graph I/O helpers

## 판정

Final gate: P0=0 P1=0

The six independent lanes found two P1 issues during review. Both were fixed
before this verdict:

- Performance/stability: CSV imports did not enforce `ReadOptions.MaxRecords`.
  Fixed by counting CSV vertex and edge data rows across the reader and adding a
  regression test.
- Developer/API: NDJSON writers discarded invalid `WriteOptions`.
  Fixed by preserving setup errors and returning `ErrInvalidOptions` on write,
  with a regression test.

## 관점 요약

| Lane | Initial P0 | Initial P1 | Resolution |
|---|---:|---:|---|
| Performance | 0 | 1 | CSV `MaxRecords` enforced before appending more records. |
| Stability/reliability | 0 | 2 | CSV `MaxRecords` fixed; blocking in-flight `io.Reader`/`io.Writer` cancellation documented as caller-owned close/deadline behavior. |
| Security | 0 | 0 | NDJSON `MaxRecordBytes` added before JSON unmarshal; strict unknown-field rejection documented as deferred. |
| Operator/Ops | 0 | 0 | Parent graph README race command and rollback wording aligned. |
| Developer/API | 0 | 1 | NDJSON invalid `WriteOptions` now fail consistently with CSV. |
| User/caller | 0 | 0 | CSV reader example, raw interchange option, unknown-field wording, and elapsed reports improved. |
| Main integration | 0 | 0 | Re-ran targeted tests, race, vet, lint, docs, and diff checks after fixes. |

## 증거

- `go test -count=1 ./graph/graphio`
- `go test -count=1 ./graph/...`
- `go test -race -count=1 ./graph/graphio`
- `go vet ./graph/...`
- `golangci-lint run ./graph/...`
- `go doc ./graph/graphio`
- `git diff --check`

## Residual P2

Strict rejection of duplicate/unknown NDJSON fields and duplicate/unknown CSV
columns remains deferred. The package README now states that unknown NDJSON
fields and non-reserved/non-property CSV columns are ignored in this first
package slice, so callers do not infer a strict schema validator contract from
the graph value package documentation.
