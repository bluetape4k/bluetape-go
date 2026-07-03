# collections

[English](README.md) | [한국어](README.ko.md)

`collections` provides focused generic helpers for common slice and map
transformations such as chunking, sliding windows, grouping, distinct-by-key,
counting, safe slicing, index pairing, padding, and error-aware map/filter
pipelines. It also includes small collection primitives for bounded stacks,
ring buffers, page metadata, and lazy permutations.

![collections transform pipeline](../docs/images/readme-diagrams/collections-transform-pipeline.png)

## Import

```go
import "github.com/bluetape4k/bluetape-go/collections"
```

## Usage

```go
groups, err := collections.GroupBy([]string{"api", "app", "job"}, func(value string) byte {
    return value[0]
})
if err != nil {
    return err
}

chunks, err := collections.Chunk([]int{1, 2, 3, 4, 5}, 2)
if err != nil {
    return err
}

windows, err := collections.Sliding([]int{1, 2, 3, 4}, 3, true)
if err != nil {
    return err
}

indexed := collections.ZipWithIndex([]string{"api", "worker"})

_ = groups
_ = chunks
_ = windows
_ = indexed

stack, err := collections.NewBoundedStack[string](2)
if err != nil {
    return err
}
stack.PushAll("old", "new", "latest")
_ = stack.Values() // []string{"latest", "new"}
```

## Behavior

- Validation errors from helper inputs wrap `ErrInvalidArgument`.
- `Chunk` rejects non-positive sizes.
- `Sliding` rejects non-positive sizes. It includes trailing partial windows
  only when `partialWindows` is true.
- `PadTo` rejects negative target sizes and pads nil input into a new slice when
  a positive size is requested.
- `SafeSubslice` clamps indexes instead of panicking. Reversed ranges return an
  empty slice at the clamped start.
- `Count` and `ZipWithIndex` preserve nil input as nil and empty non-nil input
  as an empty non-nil result.
- `ChunkBy`, `DistinctBy`, `GroupBy`, and `CountBy` reject nil key or predicate
  functions.
- `MapErr`, `ForEachErr`, `FilterErr`, and `FilterMap` reject nil mapper,
  action, or predicate
  functions.
- `Distinct` preserves first-seen order for comparable values.
- `BoundedStack` keeps the most recent values and returns snapshots
  top-to-bottom. `RingBuffer` overwrites the oldest values and returns snapshots
  oldest-to-newest.
- `PageOf` uses 0-based page numbers, snapshots items, and returns shallow
  copies from `Items`.
- `Permutations` is lazy, copies input when called, yields fresh shallow
  snapshots, and grows factorially with input size. Stop iteration early for
  large inputs.
- Mutable containers are not goroutine-safe and are not blocking queues.
- Kotlin/JVM collection extension parity, Java streams, synchronized container
  facades, primitive-array conversion DSLs, and broad sequence DSLs are
  intentionally excluded.

## Test

```bash
go test -count=1 ./collections
```
