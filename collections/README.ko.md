# collections

[English](README.md) | [한국어](README.ko.md)

`collections`는 chunking, grouping, distinct-by-key, counting, error-aware map/filter pipeline 같은 일반적인 slice/map 변환을 위한 focused generic helper를 제공합니다.

![collections transform pipeline](../docs/images/readme-diagrams/collections-transform-pipeline.png)

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/collections"
```

## 사용 예

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

## 동작

- `Chunk`는 0 이하 size를 거부합니다.
- `ChunkBy`, `DistinctBy`, `GroupBy`, `CountBy`는 nil key function을 거부합니다.
- `MapErr`와 `FilterErr`는 mapper 또는 predicate의 첫 error에서 중단합니다.
- `Distinct`는 comparable value에 대해 first-seen order를 보존합니다.

## 테스트

```bash
go test -count=1 ./collections
```
