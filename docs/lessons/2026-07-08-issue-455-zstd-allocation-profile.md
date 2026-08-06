# 교훈: zstd Compress는 NewWriter 변경 없이 Stream Encoder를 재사용할 수 있다

Issue: #455

## Context

zstd byte-slice compression은 large JSON과 SerDe repeated-collection benchmark
row에서 약 19.6MB/op를 allocation했다. 각 `Compress` 호출이 새 stream encoder를
만들었기 때문이다.

## Lesson

compression allocation을 최적화할 때는 더 빠른 API를 고르기 전에 wire byte를
보존한다. `zstd.Encoder.EncodeAll`은 매력적이지만 일부 payload에서는 stream
writer와 다른 byte를 만들 수 있다. `Compress` 뒤에서 stream encoder를 재사용하면
반복되는 encoder/history allocation cost를 제거하면서 output compatibility를
유지할 수 있다.

## Follow-up Rule

pooled compressor internal에는 다음 둘을 모두 추가한다.

- `Compress`와 `NewWriter` 간 byte equality
- shared compressor instance에 대한 `GoroutineStressTester` coverage
