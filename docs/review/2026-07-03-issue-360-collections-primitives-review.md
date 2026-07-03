# Issue #360 Collections Primitive Review

Date: 2026-07-03

Scope:

- `collections/slices.go`
- `collections/slices_test.go`
- `collections/collections_example_test.go`
- `collections/doc.go`
- `collections/README.md`
- `collections/README.ko.md`

## Evidence

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

## 7-Tier Lanes

| Lane | Verdict | Notes |
|---|---|---|
| Performance | Pass | Helpers are O(n) slice/map operations. `Sliding` returns subslices instead of copying each window, matching lightweight helper expectations. |
| Stability | Pass | Invalid sizes wrap `ErrInvalidArgument`; nil and empty input contracts are covered by table-style tests. |
| Security | Pass | No trust boundary, deserialization, path, credential, or external input sink changed. |
| Operator/Ops | Pass | No runtime configuration, logging, goroutine, or external service behavior changed. |
| Developer/API | Pass | New APIs are narrow and Go-shaped. Broad Kotlin parity, synchronized container facades, Java streams, and primitive-array DSLs remain non-goals. |
| User/Caller | Pass | README pairs and examples document common usage, nil/empty behavior, and partial-window behavior. |
| Integration | Pass | `go test -count=1 ./core ./collections`, `make test`, and `make race` passed. |

## Validation

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

## Findings

- P0: 0
- P1: 0

## Residual Risk

The new helpers intentionally do not rewrite downstream packages in this PR.
Future replacements should be made only where the helper removes duplicated
production logic without changing caller-visible behavior.
