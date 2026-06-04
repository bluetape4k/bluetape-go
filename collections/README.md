# collections

`collections` provides focused generic helpers for common slice and map
transformations such as chunking, grouping, distinct-by-key, counting, and
error-aware map/filter pipelines.

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

_ = groups
_ = chunks
```

## Behavior

- `Chunk` rejects non-positive sizes.
- `ChunkBy`, `DistinctBy`, `GroupBy`, and `CountBy` reject nil key functions.
- `MapErr` and `FilterErr` stop at the first mapper or predicate error.
- `Distinct` preserves first-seen order for comparable values.

## Test

```bash
go test -count=1 ./collections
```
