# Lesson: zstd Compress Can Reuse Stream Encoders Without Changing NewWriter

Issue: #455

## Context

zstd byte-slice compression allocated about 19.6MB/op on the large JSON and
SerDe repeated-collection benchmark rows because each `Compress` call created a
fresh stream encoder.

## Lesson

When optimizing compression allocation, preserve wire bytes before choosing a
faster API. `zstd.Encoder.EncodeAll` is attractive but can produce different
bytes from the stream writer for some payloads. Reusing stream encoders behind
`Compress` keeps output compatible while removing the repeated encoder/history
allocation cost.

## Follow-up Rule

For pooled compressor internals, add both:

- byte equality between `Compress` and `NewWriter`;
- `GoroutineStressTester` coverage for a shared compressor instance.
