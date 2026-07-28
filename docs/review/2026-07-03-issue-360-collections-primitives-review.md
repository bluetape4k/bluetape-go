# Issue #360 Collections Primitive Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-07-03

Scope:

- `collections/slices.go`
- `collections/slices_test.go`
- `collections/collections_example_test.go`
- `collections/doc.go`
- `collections/README.md`
- `collections/README.ko.md`

## 증거

- Issue #360 requires Go-native collection helpers only when they are clearer
  than `slices`, `maps`, or short local loops.
- Kotlin source references reviewed:
  - `collections/CollectionSupport.kt`
  - `collections/IterableSupport.kt`
  - `collections/SequenceSupport.kt`
  - `collections/BoundedStack.kt`
  - `collections/RingBuffer.kt`
  - `collections/PaginatedList.kt`
- Current Go container contracts reviewed:
  - `BoundedStack` keeps top-to-bottom snapshots and explicitly remains not
    goroutine-safe.
  - `RingBuffer` keeps oldest-to-newest snapshots and explicitly remains not a
    blocking queue.
  - `PageOf` validates metadata and snapshots items.
  - `Permutations` is lazy and yields fresh shallow snapshots.
- Repo scan found no package-local production callsites where replacing custom
  chunk/sliding/count logic was safer than leaving behavior unchanged in this
  PR.

## 7-Tier 관점

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Helpers are O(n) slice/map operations. `Sliding` returns subslices instead of copying each window, matching lightweight helper expectations. |
| Stability | Pass | Invalid sizes wrap `ErrInvalidArgument`; nil and empty input contracts are covered by table-style tests. |
| Security | Pass | No trust boundary, deserialization, path, credential, or external input sink changed. |
| Operator/Ops | Pass | No runtime configuration, logging, goroutine, or external service behavior changed. |
| Developer/API | Pass | New APIs are narrow and Go-shaped. Broad Kotlin parity, synchronized container facades, Java streams, and primitive-array DSLs remain non-goals. |
| User/Caller | Pass | README pairs and examples document common usage, nil/empty behavior, and partial-window behavior. |
| Integration | Pass | `go test -count=1 ./core ./collections`, `make test`, and `make race` passed. |

## 검증

- `git diff --check`: PASS
- `go test -count=1 ./collections`: PASS
- `go test -race -count=1 ./collections`: PASS
- `go test -count=1 ./core ./collections`: PASS
- `make fmt-check`: PASS
- `make tidy-check`: PASS
- `make vet`: PASS
- `make lint`: PASS
- `make test`: PASS
- `make race`: PASS

## 발견 사항

- P0: 0
- P1: 0

## 잔여 위험

The new helpers intentionally do not rewrite downstream packages in this PR.
Future replacements should be made only where the helper removes duplicated
production logic without changing caller-visible behavior.
