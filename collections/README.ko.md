# collections

[English](README.md) | [한국어](README.ko.md)

`collections`는 chunking, sliding window, grouping, distinct-by-key, counting,
safe slicing, index pairing, padding, error-aware map/filter pipeline 같은
일반적인 slice/map 변환을 위한 focused generic helper를 제공합니다. bounded
stack, ring buffer, page metadata, lazy permutation을 위한 작은 collection
primitive도 제공합니다.

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

## 동작

- Helper 입력 validation error는 `ErrInvalidArgument`를 감쌉니다.
- `Chunk`는 0 이하 size를 거부합니다.
- `Sliding`은 0 이하 size를 거부합니다. `partialWindows`가 true일 때만 뒤쪽
  partial window를 포함합니다.
- `PadTo`는 음수 target size를 거부하고, nil input이라도 양수 size가 요청되면
  새 slice로 padding합니다.
- `SafeSubslice`는 panic 대신 index를 clamp합니다. reversed range는 clamp된
  start 위치의 empty slice를 반환합니다.
- `Count`와 `ZipWithIndex`는 nil input은 nil로, empty non-nil input은 empty
  non-nil result로 유지합니다.
- `ChunkBy`, `DistinctBy`, `GroupBy`, `CountBy`는 nil key/predicate function을
  거부합니다.
- `MapErr`, `ForEachErr`, `FilterErr`, `FilterMap`은 nil mapper/action/predicate
  function을
  거부합니다.
- `Distinct`는 comparable value에 대해 first-seen order를 보존합니다.
- `BoundedStack`은 최신 값을 유지하고 top-to-bottom snapshot을 반환합니다.
  `RingBuffer`는 가장 오래된 값을 덮어쓰고 oldest-to-newest snapshot을
  반환합니다.
- `PageOf`는 0-based page number를 사용하고 items를 snapshot하며,
  `Items`는 shallow copy를 반환합니다.
- `Permutations`는 lazy iterator입니다. 호출 시 입력을 복사하고 각 결과를
  fresh shallow snapshot으로 반환하며, 결과 수는 입력 크기에 대해 factorial로
  증가합니다. 큰 입력에서는 early stop을 사용하세요.
- Mutable container는 goroutine-safe가 아니며 blocking queue도 아닙니다.
- Kotlin/JVM collection extension parity, Java stream, synchronized container
  facade, primitive-array conversion DSL, broad sequence DSL은 의도적으로
  제외합니다.

## 테스트

```bash
go test -count=1 ./collections
```
