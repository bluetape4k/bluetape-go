# compression

[English](README.md) | [한국어](README.ko.md)

`compression` provides explicit compressor adapters with byte and stream
operations. Standard-library algorithms cover gzip, zlib, and raw deflate;
focused Go dependencies provide zstd, lz4, and snappy.

## Import

```go
import "github.com/bluetape4k/bluetape-go/compression"
```

## Usage

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

## Behavior

- `Default()` currently returns zstd.
- `All()` returns gzip, zlib, deflate, zstd, lz4, and snappy in a stable order.
- Stream APIs reject nil readers or writers.
- Level-specific constructors are available for gzip, zlib, deflate, and zstd.

## Test

```bash
go test -count=1 ./compression
```
