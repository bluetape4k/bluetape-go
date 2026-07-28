# Issue #402 교차 저장소 SerDe 권고 매트릭스

Issue: #402
Parent: #398
Milestone: 0.14.0
Date: 2026-07-07
Work type: Recommendation matrix

## 목표

`0.14.0` 교차 저장소 SerDe와 compression 권고 매트릭스를 공개하되,
하나의 benchmark window를 production ranking으로 오해하지 않도록 한다.

이 매트릭스는 `bluetape-go`, `bluetape-rs`, JVM
`bluetape4k-projects`에 대해 측정된 사실, caveat, 후속 가설을 분리한다.

## 근거 목록

| Evidence | Scope | Environment and raw output |
|---|---|---|
| Go #399 fixture contract | serialization, codec, compression의 scenario와 fixture boundary. | [docs/benchmarks/2026-07-07-issue-399-serde-fixtures.md](../benchmarks/2026-07-07-issue-399-serde-fixtures.md) |
| Go #401 retained outputs | 현재 Go serialization, codec, compression raw output. | [docs/research/outputs/issue-401/environment.md](outputs/issue-401/environment.md), [go-serialization-bench.txt](outputs/issue-401/go-serialization-bench.txt), [go-codec-bench.txt](outputs/issue-401/go-codec-bench.txt), [go-compression-bench.txt](outputs/issue-401/go-compression-bench.txt) |
| Rust/JVM/Go same-condition compression snapshot | 공유 payload bytes에서 수행한 이전 normalized compression comparison. | [bluetape-rs compression report](https://github.com/bluetape4k/bluetape-rs/blob/8ab2bc46288dbec5982d9a9f00968c3cd0a984ee/docs/benchmark/compression-same-condition-benchmark.md), [metadata](https://github.com/bluetape4k/bluetape-rs/blob/8ab2bc46288dbec5982d9a9f00968c3cd0a984ee/docs/benchmark/compression-same-condition-metadata.md) |
| JVM serializer guidance | Fory/Kryo/JDK/Jackson throughput notes와 fast-mode caveat. | [bluetape4k-projects io README](https://github.com/bluetape4k/bluetape4k-projects/blob/a7dcf538e624709fa8d46fc7ea0647f30068578a/io/io/README.md) |
| JVM trust boundary | trusted-internal, allow-listed, no-dynamic-type-loading guidance. | [serialization trust profiles](https://github.com/bluetape4k/bluetape4k-projects/blob/a7dcf538e624709fa8d46fc7ea0647f30068578a/docs/security/serialization-trust-profiles.md) |
| JVM compressor artifact | JVM same-condition compressor benchmark command와 raw artifact list. | [same-condition IO compressor benchmark](https://github.com/bluetape4k/bluetape4k-projects/blob/a7dcf538e624709fa8d46fc7ea0647f30068578a/docs/benchmarks/2026-06-11-io-same-condition-compressor-benchmark.md) |

## 지표 방향

| Metric | Direction | Applies to |
|---|---|---|
| `ns/op` | 같은 benchmark row와 host 안에서는 낮을수록 좋다. | Go benchmark rows |
| `B/op` | 낮을수록 좋다. | Go allocation rows |
| `allocs/op` | 낮을수록 좋다. | Go allocation rows |
| `MB/s` or `MiB/s` | 같은 fixture class 안에서는 높을수록 좋다. | Go, Rust, JVM throughput rows |
| `encoded_bytes` | 낮을수록 조밀하지만, 단독 performance winner를 뜻하지 않는다. | Go codec/serialization rows |
| `compressed_bytes` | 낮을수록 조밀하다. | Go compression rows |
| `compressed/original` and `compressed/serialized` | 낮을수록 조밀하다. | Compression and serialize-then-compress rows |
| JVM `ops/s` | 같은 JMH shape 안에서는 높을수록 좋다. | JVM serializer README rows |

## 권고 매트릭스

| Use case | Go guidance | Rust guidance | JVM guidance | Measured evidence | Excluded interpretation |
|---|---|---|---|---|---|
| Go object payloads across storage/cache/message boundaries | JSON compatibility가 필요하면 `serialization.JSONSerializer`를 사용하고, Go-local `BTGS` envelope metadata가 필요하면 `VersionedSerializer`를 사용한다. | Go evidence만으로는 cross-runtime adapter를 권고하지 않는다. | Go JSON/versioned row에서 JVM wire compatibility를 추론하지 않는다. | Go #401은 small, medium, repeated fixture의 JSON encode/decode/round-trip row와 `BTGS` versioned envelope row를 보고한다. | 이 row들은 Go에서 Fory, Kryo, Protobuf, Avro 선택 근거가 아니다. |
| Go byte/text transport encoding | 일반 byte transport에는 Base64/Base64URL/hex를 사용한다. Base58/Base62/URL62는 ID/key 크기 값과 UUID rendering에 한정한다. | Rust에도 대응 codec family가 있지만, #402에는 보존된 Rust codec benchmark matrix가 없다. | JVM codec row는 Go #401 codec row와 normalized 상태가 아니다. | Go #401 codec output은 Base64/Base64URL/hex가 object, binary, repeated fixture를 다루고, Base58/Base62/URL62가 의도적으로 small 및 UUID fixture에 제한됨을 보여준다. | Base58/Base62/URL62를 large binary transport codec으로 승격하지 않는다. |
| Object serialize-then-compress in Go | density가 중요하면 zstd를 먼저 평가하고, CPU/latency가 중요하면 lz4 또는 snappy를 먼저 평가한다. | 넓은 방향은 Rust compression same-condition report를 따르되, default를 바꾸기 전 Rust에서 재측정한다. | JVM에도 대응 compressor evidence가 있지만 serializer-compressor 선택은 trust profile을 따라야 한다. | Go #401 serialize-then-compress row는 많은 JSON object fixture에서 zstd/deflate/gzip이 더 조밀하고, medium/repeated path에서는 lz4/snappy가 훨씬 빠름을 보여준다. | Compression density만으로 production default를 정하지 않는다. Decode/decompress cost와 trust boundary도 적용한다. |
| Large structured payload compression | 첫 density candidate는 zstd로 두고, throughput이 지배적이면 lz4/snappy를 평가한다. | Rust same-condition report는 structured payload에서 zstd가 가장 좋은 ratio를 보이고 lz4/snappy를 high-throughput candidate로 둔다. | JVM same-condition report도 zstd-density와 lz4/snappy-throughput split이라는 같은 큰 방향을 보인다. | Go #401 large JSON/text/binary rows: zstd ratios `0.05706`, `0.009397`, `0.01992`; snappy/lz4가 compression throughput을 주도하는 경우가 많다. Rust report large-payload rows는 cross-ecosystem comparison을 보존한다. | 현재 Go #401 값을 이전 Rust/JVM same-condition snapshot과 strict ranking으로 직접 비교하지 않는다. |
| Large payload decompression | decompression latency가 지배적이면 lz4를 첫 후보로 사용하고, service payload로 확인한다. | 같은 same-condition compression report에서 같은 큰 방향이 나온다. | JVM same-condition report에서도 같은 큰 방향이 나온다. | Go #401 large JSON/text/binary/random decompression rows는 보존된 row들에서 lz4가 가장 빠른 listed decompressor임을 보여준다. | Decompression speed는 best compression ratio나 smallest storage footprint를 뜻하지 않는다. |
| Interoperability with common external systems | gzip과 deflate는 compatibility choice로 유지한다. | gzip/deflate는 optional이고 explicit하게 둔다. | Java ecosystem tool과 호환성을 위해 gzip/deflate를 유지한다. | Go #401과 same-condition reports는 gzip/deflate row를 보존한다. | Compatibility-oriented codec은 보존된 snapshot에서 보통 throughput 또는 density winner가 아니다. |
| Random or already-compressed payloads | 기본 압축을 피하고, CPU cost를 추가하기 전에 측정한다. | Same-condition report는 ratio가 `1.0` 근처 또는 그 이상임을 보인다. | Same-condition report는 ratio가 `1.0` 근처 또는 그 이상임을 보인다. | Go #401 random large rows는 `compressed/original`이 약 `1.000`임을 보고한다. Rust/JVM same-condition rows도 같은 큰 behavior를 보인다. | Random-payload row를 structured JSON/text/binary payload에 일반화하지 않는다. |
| JVM fixed-schema volatile caches | Go 권고가 아니다. | concrete adapter가 생기기 전까지 Rust 권고가 아니다. | `ForyBinarySerializer.fast()`는 fixed-schema volatile cache에서 leading JVM throughput candidate이고, compressed FastFory variant는 throughput과 size를 교환한다. | JVM README는 `ForyBinarySerializer.fast()` 약 `116K ops/s`, `BinarySerializers.Fory` 약 `68K ops/s`, compressed FastFory variants 약 `12K-30K ops/s`를 보고한다. | FastFory는 default Fory와 다른 wire mode를 쓰며 trusted internal fixed-schema boundary용이지 shared untrusted input용이 아니다. |
| JVM persistent or shared boundaries | Go 권고가 아니다. | Rust 권고가 아니다. | allow-listed 또는 no-dynamic-type-loading boundary를 선호한다. Fory/Kryo는 trust profile과 schema evolution이 명시된 곳에서만 사용한다. | JVM trust profile 문서는 Kryo/Fory binary serializer를 기본적으로 `TrustedInternal`로 표시하고 shared boundary용 secure factory를 문서화한다. | Benchmark speed가 deserialization safety를 이기면 안 된다. JDK는 compatibility/deprecated이지 새 default가 아니다. |
| Rust serialization adapters | Go는 Rust adapter를 정의하지 않는다. | concrete JSON/CBOR/MessagePack/Protobuf/Avro/Fory evidence를 추가하는 adapter issue 전까지 현재 `bluetape-rs-serialization` contract-only 입장을 유지한다. | JVM evidence는 Rust adapter gap을 채우지 않는다. | Rust README와 serialization docs는 metadata, trust profiles, typed errors, safe config defaults, serde-compatible traits를 설명한다. concrete adapter는 follow-up work다. | Go/JVM measurement만으로 Rust adapter optimization issue를 만들지 않는다. |

## 안정적인 사용자 권고

- Go의 `compression.Default()`는 zstd로 유지한다. 보존된 근거는 zstd를
  density-first candidate로 지지하지만, universal production winner로 만들지는 않는다.
- latency 또는 throughput이 지배적이고 더 큰 output이 허용되면 lz4와 snappy를
  첫 후보로 둔다.
- gzip과 deflate는 compatibility choice로 유지한다.
- Base64/Base64URL/hex는 일반 byte transport encoding이다. Base58/Base62/URL62는
  compact ID/key surface로 남긴다.
- Go `serialization`은 작고 안전하게 유지한다. JSON, bytes, strings, Go `BTGS`
  envelope가 범위이며, JVM Fory/Kryo 선택은 별도 wire format이다.
- JVM Fory/Kryo guidance는 trust-profile language를 반드시 포함해야 한다.
  Trusted-internal serializer speed만으로 shared 또는 untrusted boundary에 충분하지 않다.
- Rust serialization은 concrete adapter evidence가 생기기 전까지 contract-first로 둔다.

## Caveats

- Go #401은 Apple M5, darwin/arm64, Go 1.26.4에서 캡처되었다.
- Rust/JVM same-condition compression evidence는 shared payload bytes를 사용한
  2026-06-11 local snapshot이다. 큰 방향에는 유용하지만 strict regression threshold로는
  쓰지 않는다.
- 현재 Go #401 compression row와 이전 Rust/JVM same-condition snapshot은 같은 실행이
  아니다. 따라서 이 보고서는 cross-runtime winner 대신 broad candidate를 명명한다.
- JVM serializer row는 문서화된 JMH throughput shape를 쓰며 Go #399 object fixture와
  다른 fixture를 사용한다.
- Allocation data는 보존된 #401 evidence 안에서 Go-only다.

## 후속 가설

다음 항목은 #403 triage를 위한 evidence-backed candidate이며, 이 보고서가 직접 생성한
issue가 아니다.

| Candidate | Evidence | Suggested #403 handling |
|---|---|---|
| Go zstd allocation cost on serialize-then-compress | Go #401 serialize-then-compress zstd row는 density를 개선하는 경우가 많지만 lz4/snappy row보다 훨씬 많은 bytes를 allocate한다. | public default를 바꾸기 전에 zstd writer/buffer reuse를 profile한다. |
| Go JSON decode allocation cost for repeated collections | Go #401 repeated JSON decode/round-trip row는 단순 byte/string serializer row보다 더 많이 allocate한다. | pooling을 추가하기 전에 fixture-specific allocation source를 profile한다. |
| Go codec large-payload guidance | Base64/Base64URL/hex는 넓은 byte-payload coverage를 가지며, Base58/Base62/URL62는 small/UUID scoped다. | documentation boundary를 유지한다. 실제 large-ID/key workload가 나타나기 전까지 optimization issue를 만들지 않는다. |
| Cross-repo full rerun | Rust/JVM same-condition compression evidence가 Go #401보다 오래되었다. | release note에 strict cross-runtime ranking이 필요하면 하나의 committed fixture contract에서 세 ecosystem을 모두 재실행한다. |

## #402에서 새 Optimization Issue를 만들지 않음

#403이 이미 follow-up optimization issue 생성을 소유한다. 이 보고서는 candidate evidence를
제공하며, #403이 순위를 정하기 전에 새로운 narrow optimization issue를 열지 않는다.
