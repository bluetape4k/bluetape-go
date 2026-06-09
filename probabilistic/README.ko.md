# probabilistic

[English](README.md) | [한국어](README.ko.md)

`probabilistic`는 first-party 확률적 자료구조를 제공합니다. 현재 공개 surface는
인메모리 Bloom filter입니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/probabilistic"
```

## Bloom Filter

```go
cfg, err := probabilistic.NewConfig(1_000_000, 0.01)
if err != nil {
    return err
}

filter, err := probabilistic.NewStringBloomFilter(cfg)
if err != nil {
    return err
}

filter.Put("user:42")
if filter.MightContain("user:42") {
    // 값이 있을 수 있습니다.
}
```

string이나 `[]byte`가 아닌 값은 stable compatibility key를 가진 명시적 hasher를
제공합니다.

```go
hasher, err := probabilistic.NewHasher("int-decimal", func(v int) []byte {
    return []byte(strconv.Itoa(v))
})
```

패키지 생성 filter 병합은 config와 hasher key가 모두 같은 경우에만 허용됩니다.
Custom hasher 함수는 deterministic하고 goroutine-safe해야 합니다. 같은 key를 가진
두 filter는 merge-compatible로 간주되므로 compatibility key 안정성은 caller가
보장합니다.

## 동작

- `MightContain`이 `false`를 반환하면 값이 없다는 뜻입니다.
- `MightContain`이 `true`를 반환하면 값이 있을 수 있습니다. False positive는
  Bloom filter의 정상 동작입니다.
- 성공적으로 삽입되고 이후 `Clear`로 지워지지 않은 값은 false negative를 만들지
  않아야 합니다.
- `Put`은 하나 이상의 bit가 새로 켜졌는지 반환합니다. `false`가 값의 기존 존재를
  증명하지는 않습니다.
- 삭제는 지원하지 않습니다.
- 구현은 concurrent `Put`, `MightContain`, `PutAll`, `Clear`, metadata read에
  대해 hasher가 goroutine-safe일 때 goroutine-safe입니다.
- 패키지는 context-aware I/O나 background job 경계를 갖지 않습니다.

## 오류

Sentinel error는 `errors.Is`를 지원합니다.

- `ErrInvalidConfig`
- `ErrIncompatibleFilter`
- `ErrNilFilter`
- `ErrNilHasher`
- `ErrEmptyHasherKey`

## 후속 범위

Redis-backed Bloom, Cuckoo, HyperLogLog 지원은
[#182](https://github.com/bluetape4k/bluetape-go/issues/182)로 넘깁니다. 이
패키지는 Redis, RedisBloom, Testcontainers를 사용하지 않습니다.

## 테스트

```bash
go test -count=1 ./probabilistic
go test -race -count=1 ./probabilistic
```
