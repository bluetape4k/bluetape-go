# Issue #455 Environment

Issue: #455
Milestone: 0.15.0
Baseline revision: `c66021c24345`
Generated UTC: `2026-07-07T16:08:08Z`
Generated local: `2026-07-08 01:08:08 KST`
Host CPU: `Apple M5`

## Target Rows

- `BenchmarkCompressorsCompress/json/large/zstd`
- `BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd`

## Checksums

```text
1010f819c093ed89a4231de5500a0ab7ad62f2ca9b8ffc03abe2bb4c38c4b115  zstd-compress-json-large-baseline.mem.pprof
14bdabc48dbef8199428cae330e730142553eeccecdc527089312b4405758142  zstd-serde-repeated-baseline.mem.pprof
2c85a798302d209f5ee64300717d89a4394cb38059810f7298240e42a194bdba  zstd-serde-repeated-after-profile-bench.txt
3545bb2c88de67c83294982e38a3317a16094649f049da33e05d95d02e1a0b9e  zstd-serde-repeated-after.mem.pprof
37cdaa820ff35aef65b0574ce59af7692635e593e82059a72eeb73b757e434cf  zstd-serde-repeated-after-bench.txt
436ed4a2912789baa477ff39c134759ab4c8d365299591b584480c3a058b4e61  zstd-serde-repeated-after-mem-top.txt
51a16407d5cee9b15840cd13f9b99e1b55e8dab021dd6c4f0aeb26793efccd8c  zstd-compress-json-large-baseline-mem-top.txt
76fa287544c4f560d09da1bc01a7c61bfb96fdab86c3424bb27469f106882653  zstd-serde-repeated-baseline-bench.txt
a0766abf42cbe55cf93bf3bd3753b4d084262b45b5a2e6938368d0eeff062109  zstd-compress-json-large-baseline-bench.txt
a3c8c809a8a93edaa4c09265bd012b48fda8ee12400a8a0aae2713c7aa0882ab  zstd-serde-repeated-baseline-mem-top.txt
b6c8a4a238cea2d73116b984e3e928a81aec21895206eabd983da84625237935  zstd-compress-json-large-after-profile-bench.txt
c1501632c583b88ce66aef17617cef99637d58f89278dc87fd8d325e75351217  zstd-compress-json-large-after.mem.pprof
c64b4c6fab33bb9734d51d528e54a12eae655fafedf597ea0b7e4301bf1d7867  zstd-compress-json-large-after-mem-top.txt
e6833e7e377e21ca716d972de87d358f52ec182d86428f533d52268baec9492d  zstd-compress-json-large-baseline-profile-bench.txt
f165a50d4f6a5eaf8d91cba2c7e7f94bde3038964ff81afba3b54c8b7215fb2f  zstd-compress-json-large-after-bench.txt
f7b33efe6254c02e9368a040370ac5362127ee46bef09bc532fa3008df0272c0  zstd-serde-repeated-baseline-profile-bench.txt
```

## Validation Commands

- `gofmt -w compression/zstd.go compression/compression_test.go`
- `go test -count=1 ./compression ./serialization`
- `go test -race -count=1 ./compression ./serialization`
- `go test -run '^$' -bench '^BenchmarkCompressorsCompress/json/large/zstd$' -benchmem -benchtime=1x ./compression`
- `go test -run '^$' -bench '^BenchmarkSerializationSerializeThenCompress/JSON/serde-repeated-collection-v1/zstd$' -benchmem -benchtime=1x ./serialization`
- `golangci-lint cache clean`
- `make ci`
