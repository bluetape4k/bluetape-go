# Issue 43 I/O, Codec, Crypto, Protocol 연구 범위

Issue #43은 portable I/O, compression, serialization, crypto, protocol concept 중
어떤 항목을 추가 Go work로 삼을지 결정하는 0.7.0 research gate다. 현재 repo에는 이미
`codec`, `compression`, `serialization`이 있으므로, 핵심 결정은 Go standard library나
기존 package를 중복하지 않는 것이다.

## 소스 인벤토리

Source repository: `/Users/debop/work/bluetape4k/bluetape4k-projects/io`

- `io/io` provides Kotlin compressors, binary serializers, serializer +
  compression combinations, file utilities, safe path helpers, and zip-bomb
  protection.
- `io/tink` provides AEAD/DAEAD wrappers, associated-data-first APIs, string
  and byte encryption helpers, versioned keysets, Redis keyset stores, MAC,
  digest helpers, and rotation guidance.
- `io/http`, `io/grpc`, `io/csv`, `io/protobuf`, `io/avro`, `io/json`,
  `io/jackson*`, `io/fastjson2`, `io/okio`, `io/netty`, `io/vertx`,
  `io/feign`, and `io/retrofit2` are mostly JVM/backend integration surfaces.
- `bluetape4k-aws` contributes SigV4 and service-protocol examples, but #42
  already rejected a Ktor-shaped or broad AWS wrapper port.

## 현재 bluetape-go 증거

- `codec`는 이미 Base58, Base62, Base64, hex, URL-safe helper, alphabet validation,
  text와 byte API 사이의 UTF-8 distinction을 다룬다.
- `compression`은 이미 gzip, zlib, deflate, zstd, lz4, snappy, registry-backed
  selection, byte-slice API, stream API, bounded decompression helper를 다룬다.
- `serialization`은 이미 JSON, raw bytes, UTF-8 string, strict JSON, versioned
  `BTGS` envelope을 다룬다.
- #71은 encryption facade decision을 위해 이미 열려 있으며 AEAD/DAEAD/keyset/KMS/Redis
  key material work의 올바른 owner다.
- #42는 SigV4 protocol work를 #43으로 라우팅했지만 standalone AWS wrapper는 rejected했다.

## 외부 Go 증거

- Go standard library package는 이미 DEFLATE, gzip, zlib, JSON, CSV, HTTP, TLS,
  URL handling, hash, core cryptographic primitive를 다룬다.
- `google.golang.org/protobuf`는 현재 Go Protobuf implementation 및
  code-generation/runtime path다. 이전 `github.com/golang/protobuf` API는 superseded다.
- `google.golang.org/grpc`는 canonical Go gRPC stack이며, 구체적인 service example이
  shared setup을 요구하지 않는 한 직접 사용해야 한다.
- Avro에는 `github.com/linkedin/goavro/v2`, `github.com/hamba/avro/v2` 같은 credible
  Go option이 있지만, bluetape-go package가 되려면 schema evolution, registry
  integration, Kafka payload policy에 대한 concrete consumer가 먼저 필요하다.
- Tink Go와 `filippo.io/age`는 credible encryption candidate지만, key material boundary가
  design을 지배하므로 API shape는 #71에 속한다.
- AWS SDK for Go v2는 이미 generic SigV4/SigV4A signer support를 갖고 있다.
  bluetape-go wrapper가 정당화되려면 non-AWS HTTP client consumer가 먼저 필요하다.

## 순위

| Area | Go fit | Risk | Decision |
|---|---:|---:|---|
| Existing `codec` | High | Low | Keep; no new issue. |
| Existing `compression` | High | Medium | Keep; no new issue until bzip2/zip or benchmark gaps are proven. |
| Existing `serialization` | High | Medium | Keep; no new issue until a non-JSON binary format has a consumer. |
| AEAD/DAEAD/keysets/KMS | High | High | Route to #71. |
| MAC/digest helpers | Medium | Medium/high | Route to #71 when security-related; otherwise use stdlib directly. |
| Protobuf | Medium/high | Medium | Use canonical generated-code/runtime stack directly; examples only if a service needs it. |
| Avro | Medium | High | Defer until Kafka/schema-registry or data pipeline consumer exists. |
| CSV/JSON | High | Low/medium | Use stdlib/current serialization package; no wrapper. |
| HTTP/gRPC | High | Medium | Use stdlib/gRPC directly; package wrappers need concrete repeated setup evidence. |
| Okio-style buffered pipeline | Low/medium | High | Defer; Go `io.Reader`/`io.Writer` already owns this shape. |
| Netty/Vert.x/Feign/Retrofit | Low | High | Reject as JVM framework/client ports. |
| SigV4 generic HTTP signing | Medium | Medium/high | Defer; AWS SDK signer exists and #42 rejected AWS wrapper ports. |

## 유지 / 구현됨

- 기존 `codec`, `compression`, `serialization` package를 Go-native I/O foundation으로
  유지한다.
- `compression.DecompressLimit`를 untrusted compressed byte에 대한 default guidance로
  유지한다.
- `serialization.VersionedSerializer`를 repo-owned envelope boundary로 유지한다.

## #71로 라우팅

- AEAD, deterministic AEAD, associated data, keyset, Redis keyset store,
  KMS envelope compatibility, MAC, digest, encryption error semantic.
- #43은 crypto API를 정의하지 않고, 어디에 속하는지만 말해야 한다.

## Example-only / 직접 사용

- Protobuf and gRPC should use `google.golang.org/protobuf` and
  `google.golang.org/grpc` directly. Add examples only when a service package
  needs shared setup or fixtures.
- CSV and JSON should use the standard library and existing `serialization`
  package.
- HTTP should use `net/http` and existing `resilience` HTTP adapters unless a
  repeated caller-owned setup emerges.

## Defer

- Avro, schema registry integration, and Kafka payload codecs until a messaging
  or data-pipeline issue requires them.
- Zip archive helpers and bzip2 until there is a file/archive workflow and
  bounded extraction contract.
- SigV4 generic signing until a non-AWS SDK HTTP client package needs it.
- Okio-style pipeline helpers until a concrete streaming codec package proves
  `io.Reader`/`io.Writer` composition is insufficient.

## Rejected Wrapper

- Generic JSON, CSV, HTTP, gRPC, Protobuf, AWS SigV4, Feign, Retrofit, Netty,
  Vert.x, Jackson, Fastjson2, or Okio ports.
- JVM binary serializers such as JDK/Kryo/Fory as general Go package surfaces.
- Unsafe deserialization or dynamic type loading boundaries.

## 필요한 Issue 업데이트

- #43 should record that no new implementation issues are created from this
  research pass.
- #71 should record that encryption/keyset/MAC/digest/KMS decisions are owned
  there and should use #43 only as routing evidence.

## 검증 계획

- Documentation-only PR에서는 `git diff --check`와 targeted `rg`를 실행한다.
- #43과 #71 issue body가 #43 research update를 포함하는지 확인한다.
- External evidence는 `bluetape4k-wiki`에 보존하고
  `gno update`, `gno embed --collection bluetape4k-wiki`, and representative
  `gno search`로 검증한다.
- Go code change가 없으므로 이 PR에는 Go test가 필요하지 않다.

## 후속 권고

Encryption은 다음으로 #71에서 다룬다. Real caller가 Avro/schema-registry, archive
extraction, SigV4 generic HTTP signing, shared Protobuf/gRPC setup을 필요로 하기
전까지 새 I/O implementation issue를 추가하지 않는다.
