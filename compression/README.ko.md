# compression

[English](README.md) | [한국어](README.ko.md)

`compression`은 byte/stream operation을 제공하는 명시적인 compressor adapter를 제공합니다. Standard-library algorithm은 gzip, zlib, raw deflate를 담당하고, focused Go dependency는 zstd, lz4, snappy를 제공합니다.

## 가져오기

```go
import "github.com/bluetape4k/bluetape-go/compression"
```

## 사용 예

```go
compressor := compression.Default()
compressed, err := compressor.Compress(payload)
if err != nil {
    return err
}

decompressed, err := compressor.Decompress(compressed)
if err != nil {
    return err
}
```

## 동작

- `Default()`는 현재 zstd를 반환합니다.
- `All()`은 gzip, zlib, deflate, zstd, lz4, snappy를 안정적인 순서로 반환합니다.
- Stream API는 nil reader/writer를 거부합니다.
- gzip, zlib, deflate, zstd에는 level-specific constructor가 있습니다.

## 테스트

```bash
go test -count=1 ./compression
```
