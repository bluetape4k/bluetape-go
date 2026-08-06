# Issue 9 Collections Inventory

## 맥락

Issue #9는 `bluetape4k/core`의 유용한 collection support를 idiomatic Go로 옮긴다. Kotlin source에는 많은
extension helper와 mutable data structure가 있으므로, 첫 Go slice는 standard library가 이미 덮지 않는
transformation helper로 좁힌다.

## Source Inventory

관찰한 `bluetape4k-projects/bluetape4k/core` collection 영역:

- `IterableSupport.kt`: iterator conversion, counting, primitive array conversion, catching map/for-each variant,
  sliding window.
- `ListSupport.kt`: range list construction.
- `CollectionSupport.kt`: prepend/append/swap, padding, element count, chunk-by-predicate, safe sub-list,
  zip-with-index.
- `MapEntrySupport.kt`: pair/map-entry conversion.
- `BoundedStack.kt`, `RingBuffer.kt`, `QueueSupport.kt`, `PaginatedList.kt`: reusable data structure와 pagination model.
- `permutations/*`, `graph/*`, Eclipse Collections adapter, Java stream support: specialized 또는 JVM-specific 영역.

## 지금 구현

- `Chunk`: error-returning validation을 가진 fixed-size chunking.
- `ChunkBy`: predicate-started chunking.
- `Distinct` 및 `DistinctBy`: 순서를 보존하는 duplicate removal.
- `GroupBy` 및 `CountBy`: derived key 기준 grouping/counting.
- `MapErr` 및 `FilterErr`: 첫 error에서 멈추는 transformation.
- `FilterMap`: map한 뒤 성공 또는 선택된 값만 유지.

## Standard Library 채택

- 단순 contains/index/delete/sort/reverse operation은 Go `slices` package를 직접 쓴다.
- 순서가 중요하지 않은 map key/value iteration은 Go `maps` package 또는 직접 `for range`를 쓴다.
- pair/map-entry conversion은 Go map iteration이 이미 key/value pair를 노출한다.
- iterator-to-slice conversion은 input이 slice가 아니라 iterator일 때 Go iterator helper를 쓴다.

## 보류

- bounded stack, queue, ring buffer, pagination, permutation helper는 public package가 되기 전에 별도 API 결정과 example이
  필요하다.
- sliding window는 실제 use case가 생기면 뒤따를 수 있다. 현재 0.1.0 필요는 `Chunk`와 `ChunkBy`가 덮는다.
- primitive array conversion은 JVM-specific이며 Go에서 유용하지 않다.
- Eclipse Collections와 Java stream adapter는 Go로 portable하지 않다.

## 결정

작은 public `collections` package를 만든다. helper는 deterministic하고 allocation-conscious해야 하며 nil slice/map과 empty
slice/map의 차이를 명시적으로 다룬다. bluetape-specific value 없이 standard `slices` 또는 `maps` package를 감싸지 않는다.
